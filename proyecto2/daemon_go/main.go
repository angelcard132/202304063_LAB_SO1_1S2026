package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/cilium/ebpf/rlimit"
	"github.com/redis/go-redis/v9"
)

const (
	PROC_PATH    = "/proc/continfo_pr2_so1_202304063"
	BPF_OBJ_PATH = "../ebpf/kill_probe.bpf.o"
)

type ProcessInfo struct {
	PID        int     `json:"pid"`
	Name       string  `json:"name"`
	VszKb      uint64  `json:"vsz_kb"`
	RssKb      uint64  `json:"rss_kb"`
	RamPercent float64 `json:"ram_percent"`
	CpuTimeMs  uint64  `json:"cpu_time_ms"`
	CpuPercent float64 `json:"cpu_percent"` // compatibilidad con el módulo cargado anteriormente
}

type ProcData struct {
	TotalRamMb uint64        `json:"total_ram_mb"`
	FreeRamMb  uint64        `json:"free_ram_mb"`
	UsedRamMb  uint64        `json:"used_ram_mb"`
	TotalRamKb uint64        `json:"total_ram_kb"`
	FreeRamKb  uint64        `json:"free_ram_kb"`
	UsedRamKb  uint64        `json:"used_ram_kb"`
	Processes  []ProcessInfo `json:"processes"`
}

type ContainerInfo struct {
	ID               string
	Name             string
	Image            string
	PID              int
	VszKb            uint64
	RssKb            uint64
	RamPercent       float64
	CpuPercent       float64
	KernelCpuPercent float64
	IsIntruder       bool
}

// TopContainerRow is stored as JSON so Grafana can extract one row per
// container instead of receiving the flattened output of ZRANGE.
type TopContainerRow struct {
	Rank        int     `json:"rank"`
	Label       string  `json:"label"`
	Name        string  `json:"name"`
	ContainerID string  `json:"container_id"`
	PID         int     `json:"pid"`
	Value       float64 `json:"value"`
}

type KillEvent struct {
	PidCaller uint32
	PidTarget uint32
	Sig       int32
}

var rdb *redis.Client
var ctx = context.Background()

var ebpfKillEvents = make(chan KillEvent, 100)

type cpuSample struct {
	time      time.Time
	cpuTimeMs uint64
}

var previousCPUSamples = make(map[int]cpuSample)

func main() {
	log.Println("Iniciando Daemon en Go - Proyecto 2 SO1 (202304063)")

	// 1. Conectar a Valkey
	valkeyAddr := os.Getenv("VALKEY_ADDR")
	if valkeyAddr == "" {
		valkeyAddr = "localhost:6379"
	}
	rdb = redis.NewClient(&redis.Options{
		Addr: valkeyAddr,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("Advertencia: No se pudo conectar a Valkey en %s: %v", valkeyAddr, err)
	} else {
		log.Printf("Conexión exitosa a Valkey en %s", valkeyAddr)
		// Grafana debe mostrar 0 en lugar de "No data" antes del primer evento.
		rdb.SetNX(ctx, "counter:ebpf_kills", 0, 0)
		rdb.SetNX(ctx, "system:containers_deleted_total", 0, 0)
	}

	// 2. Cargar módulo de kernel si no está cargado
	if _, err := os.Stat(PROC_PATH); os.IsNotExist(err) {
		log.Println("Cargando módulo de kernel...")
		koPath, _ := filepath.Abs("../kernel_module/continfo_pr2_so1_202304063.ko")
		exec.Command("insmod", koPath).Run()
	} else {
		log.Println("Módulo de kernel ya cargado, /proc disponible.")
	}

	// 3. Iniciar sonda eBPF
	go runEBPFProbe()

	// 4. Configurar cronjob
	setupCronjob()

	// 5. Manejo de señales para cierre limpio
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("\nDeteniendo Daemon de Go. Limpiando cronjob...")
		removeCronjob()
		os.Exit(0)
	}()

	// 6. Loop principal (cada 20 segundos)
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	// Ejecutar primera iteración inmediatamente
	runMonitoringCycle()

	for range ticker.C {
		runMonitoringCycle()
	}
}

// CICLO DE MONITOREO

func runMonitoringCycle() {
	log.Println("\n--- [Ciclo de Monitoreo] ---")

	// a. Leer /proc
	raw, err := os.ReadFile(PROC_PATH)
	if err != nil {
		log.Printf("Error al leer %s: %v", PROC_PATH, err)
		return
	}

	var data ProcData
	if err := json.Unmarshal(raw, &data); err != nil {
		log.Printf("Error al deserializar JSON de /proc: %v", err)
		return
	}

	log.Printf("Telemetría: RAM Total: %d MB | Usada: %d MB | Libre: %d MB",
		data.TotalRamMb, data.UsedRamMb, data.FreeRamMb)

	// Guardar métricas en Valkey con el esquema exacto solicitado
	nowMs := time.Now().UnixMilli()
	if rdb != nil {
		pipe := rdb.Pipeline()

		// Claves STRING principales
		pipe.Set(ctx, "system:total_ram_mb", data.TotalRamMb, 0)
		pipe.Set(ctx, "system:ram_used_mb", data.UsedRamMb, 0)
		pipe.Set(ctx, "system:ram_free_mb", data.FreeRamMb, 0)

		// Claves auxiliares de compatibilidad
		pipe.Set(ctx, "telemetry:ram:total_mb", data.TotalRamMb, 0)
		pipe.Set(ctx, "telemetry:ram:used_mb", data.UsedRamMb, 0)
		pipe.Set(ctx, "telemetry:ram:free_mb", data.FreeRamMb, 0)

		// Historiales (Lista JSON y Stream)
		histJSON := fmt.Sprintf(`{"ts":%d,"total_mb":%d,"used_mb":%d,"free_mb":%d}`,
			nowMs, data.TotalRamMb, data.UsedRamMb, data.FreeRamMb)
		pipe.LPush(ctx, "telemetry:ram:history", histJSON)
		pipe.LTrim(ctx, "telemetry:ram:history", 0, 199)

		// Streams dedicados para Grafana. El plugin Redis devuelve los campos de
		// XRANGE como una lista plana; por eso cada entrada debe tener una sola
		// métrica y el dashboard filtra los tokens no numéricos.
		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: "telemetry:ram:used",
			MaxLen: 200,
			Approx: true,
			Values: map[string]interface{}{
				"value": data.UsedRamMb,
			},
		})
		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: "telemetry:ram:free",
			MaxLen: 200,
			Approx: true,
			Values: map[string]interface{}{
				"value": data.FreeRamMb,
			},
		})

		pipe.Exec(ctx)
	}

	// b. Obtener contenedores Docker y cruzar métricas
	procMetrics := buildProcMetricsMap(data.Processes)
	containers := getDockerContainers(procMetrics)
	log.Printf("Contenedores Docker activos: %d", len(containers))

	// Guardar Top 5 solo de contenedores gestionables. Grafana y Valkey no
	// deben ocupar los primeros lugares del ranking de la aplicación.
	var rankedContainers []ContainerInfo
	for _, c := range containers {
		if !isSystemContainer(c.Name) {
			rankedContainers = append(rankedContainers, c)
		}
	}
	saveTopContainers(rankedContainers)

	// c. Separar contenedores protegidos (Grafana, Valkey) vs gestionables
	var lowRes []ContainerInfo
	var highRes []ContainerInfo
	var intruders []ContainerInfo

	for _, c := range containers {
		if isSystemContainer(c.Name) {
			continue
		}
		if c.IsIntruder {
			intruders = append(intruders, c)
		} else if isHighResource(c) {
			highRes = append(highRes, c)
		} else {
			lowRes = append(lowRes, c)
		}
	}

	// Eliminar intrusos inmediatamente
	for _, c := range intruders {
		log.Printf("[ALERTA SEGURIDAD] Eliminando intruso: %s (%s)", c.Name, c.ID)
		removeContainer(c)
	}

	// Mantener exactamente 3 de bajo consumo
	sort.Slice(lowRes, func(i, j int) bool {
		return lowRes[i].CpuPercent > lowRes[j].CpuPercent
	})
	for i := 3; i < len(lowRes); i++ {
		log.Printf("[RESTRICCIÓN] Exceso bajo consumo. Eliminando: %s", lowRes[i].Name)
		removeContainer(lowRes[i])
	}

	// Mantener exactamente 2 de alto consumo
	sort.Slice(highRes, func(i, j int) bool {
		return highRes[i].RssKb > highRes[j].RssKb
	})
	for i := 2; i < len(highRes); i++ {
		log.Printf("[RESTRICCIÓN] Exceso alto consumo. Eliminando: %s (RAM=%.2f%%, RSS=%dKB)",
			highRes[i].Name, highRes[i].RamPercent, highRes[i].RssKb)
		removeContainer(highRes[i])
	}

	// Drenar eventos eBPF pendientes y registrar en Valkey
	drainEBPFEvents()
}

func runEBPFProbe() {
	bpfObjPath, _ := filepath.Abs(BPF_OBJ_PATH)
	log.Printf("Iniciando sonda eBPF desde: %s", bpfObjPath)

	// Los mapas BPF necesitan más memoria bloqueada que el límite por defecto
	// de muchos sistemas. Si no se eleva este límite, NewCollection falla antes
	// de que el programa pueda adjuntarse al tracepoint.
	if err := rlimit.RemoveMemlock(); err != nil {
		log.Printf("[eBPF] No se pudo elevar MEMLOCK: %v", err)
		return
	}

	spec, err := ebpf.LoadCollectionSpec(bpfObjPath)
	if err != nil {
		log.Printf("[eBPF] Error al cargar spec del objeto BPF: %v", err)
		return
	}

	coll, err := ebpf.NewCollection(spec)
	if err != nil {
		log.Printf("[eBPF] Error al crear colección BPF: %v", err)
		return
	}
	defer coll.Close()

	prog, ok := coll.Programs["trace_sys_enter_kill"]
	if !ok {
		log.Printf("[eBPF] Programa 'trace_sys_enter_kill' no encontrado")
		return
	}

	tp, err := link.Tracepoint("syscalls", "sys_enter_kill", prog, nil)
	if err != nil {
		log.Printf("[eBPF] Error al adjuntar sys_enter_kill: %v", err)
		return
	}
	defer tp.Close()
	log.Println("[eBPF] Programa adjuntado a sys_enter_kill")

	rbMap, ok := coll.Maps["events"]
	if !ok {
		log.Printf("[eBPF] Mapa 'events' no encontrado en la colección BPF")
		return
	}

	rd, err := ringbuf.NewReader(rbMap)
	if err != nil {
		log.Printf("[eBPF] Error al crear ring buffer reader: %v", err)
		return
	}
	defer rd.Close()

	log.Println("[eBPF] Sonda activa. Escuchando señales SIGKILL/SIGTERM...")

	for {
		record, err := rd.Read()
		if err != nil {
			if strings.Contains(err.Error(), "closed") {
				return
			}
			log.Printf("[eBPF] Error al leer evento: %v", err)
			continue
		}

		if len(record.RawSample) < int(unsafe.Sizeof(KillEvent{})) {
			continue
		}

		var event KillEvent
		event.PidCaller = binary.LittleEndian.Uint32(record.RawSample[0:4])
		event.PidTarget = binary.LittleEndian.Uint32(record.RawSample[4:8])
		event.Sig = int32(binary.LittleEndian.Uint32(record.RawSample[8:12]))

		sigName := "SIGTERM"
		if event.Sig == 9 {
			sigName = "SIGKILL"
		}
		log.Printf("[eBPF] kill() interceptado: caller_pid=%d → target_pid=%d señal=%s(%d)",
			event.PidCaller, event.PidTarget, sigName, event.Sig)

		select {
		case ebpfKillEvents <- event:
		default:
			log.Println("[eBPF] Canal de eventos lleno, descartando evento")
		}
	}
}

func drainEBPFEvents() {
	for {
		select {
		case event := <-ebpfKillEvents:
			if rdb == nil {
				continue
			}
			sigName := "SIGTERM"
			if event.Sig == 9 {
				sigName = "SIGKILL"
			}
			nowMs := time.Now().UnixMilli()
			evJSON := fmt.Sprintf(
				`{"timestamp":%d,"caller_pid":%d,"target_pid":%d,"signal":"%s","source":"ebpf"}`,
				nowMs, event.PidCaller, event.PidTarget, sigName,
			)
			rdb.LPush(ctx, "events:ebpf_kills", evJSON)
			rdb.LTrim(ctx, "events:ebpf_kills", 0, 499)
			rdb.Incr(ctx, "counter:ebpf_kills")
			log.Printf("[eBPF→Valkey] Evento kill registrado: target_pid=%d señal=%s", event.PidTarget, sigName)
		default:
			return
		}
	}
}

func buildProcMetricsMap(procs []ProcessInfo) map[int]ProcessInfo {
	m := make(map[int]ProcessInfo, len(procs))
	for _, p := range procs {
		m[p.PID] = p
	}
	return m
}

func getDockerContainers(procMetrics map[int]ProcessInfo) []ContainerInfo {
	cmd := exec.Command("docker", "ps", "--format", "{{.ID}}|{{.Names}}|{{.Image}}")
	out, err := cmd.Output()
	if err != nil {
		log.Printf("Error al ejecutar docker ps: %v", err)
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var result []ContainerInfo

	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 3 {
			continue
		}
		cID, cName, cImage := parts[0], parts[1], parts[2]

		pidCmd := exec.Command("docker", "inspect", "-f", "{{.State.Pid}}", cID)
		pidOut, err := pidCmd.Output()
		pid := 0
		if err == nil {
			fmt.Sscanf(strings.TrimSpace(string(pidOut)), "%d", &pid)
		}

		info := ContainerInfo{
			ID:         cID,
			Name:       cName,
			Image:      cImage,
			PID:        pid,
			IsIntruder: strings.Contains(cImage, "intruder") || strings.Contains(cName, "intruder"),
		}

		if pm, ok := procMetrics[pid]; ok {
			info.VszKb = pm.VszKb
			info.RssKb = pm.RssKb
			info.RamPercent = pm.RamPercent
			if pm.CpuTimeMs > 0 {
				info.KernelCpuPercent = calculateKernelCPUPercent(pid, pm.CpuTimeMs)
			} else {
				// El módulo de kernel anterior exponía cpu_percent en lugar de
				// cpu_time_ms. Mantener este fallback evita que el ranking quede
				// en cero mientras se recarga el .ko actualizado.
				info.KernelCpuPercent = pm.CpuPercent
			}
		}

		statsCmd := exec.Command("docker", "stats", "--no-stream", "--format", "{{.CPUPerc}}", cID)
		if statsOut, err := statsCmd.Output(); err == nil {
			cpuStr := strings.TrimSuffix(strings.TrimSpace(string(statsOut)), "%")
			fmt.Sscanf(cpuStr, "%f", &info.CpuPercent)
		}

		result = append(result, info)
	}

	return result
}

// calculateKernelCPUPercent converts the cumulative CPU time exposed by the
// kernel module into the percentage used since the previous monitoring cycle.
// The result is normalized by the number of logical CPUs, like docker stats.
func calculateKernelCPUPercent(pid int, currentCpuTimeMs uint64) float64 {
	now := time.Now()
	previous, exists := previousCPUSamples[pid]
	previousCPUSamples[pid] = cpuSample{time: now, cpuTimeMs: currentCpuTimeMs}
	if !exists || currentCpuTimeMs < previous.cpuTimeMs {
		return 0
	}

	elapsedSeconds := now.Sub(previous.time).Seconds()
	if elapsedSeconds <= 0 {
		return 0
	}

	percent := (float64(currentCpuTimeMs-previous.cpuTimeMs) / (elapsedSeconds * 1000)) * 100
	if cpus := runtime.NumCPU(); cpus > 0 {
		percent /= float64(cpus)
	}
	return percent
}

func isSystemContainer(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "grafana") || strings.Contains(n, "valkey") || strings.Contains(n, "redis")
}

func isHighResource(c ContainerInfo) bool {
	img := strings.ToLower(c.Image)
	name := strings.ToLower(c.Name)
	return strings.Contains(img, "go-client") || strings.Contains(name, "high") || c.RamPercent > 0.5 || c.CpuPercent > 20.0
}

func removeContainer(c ContainerInfo) {
	exec.Command("docker", "rm", "-f", c.ID).Run()

	log.Printf("[ELIMINADO] Contenedor %s (%s) detenido y eliminado", c.Name, c.ID)

	nowMs := time.Now().UnixMilli()
	if rdb != nil {
		evJSON := fmt.Sprintf(
			`{"timestamp":%d,"container_id":"%s","name":"%s","image":"%s","action":"DAEMON_KILL","value":1}`,
			nowMs, c.ID, c.Name, c.Image,
		)
		rdb.LPush(ctx, "events:containers_killed", evJSON)
		rdb.LTrim(ctx, "events:containers_killed", 0, 499)
		rdb.Incr(ctx, "counter:containers_killed")

		// Claves de la nueva especificación
		rdb.Incr(ctx, "system:containers_deleted_total")

		// Stream de una sola métrica para que XRANGE pueda convertirse en una
		// serie temporal numérica en Grafana.
		rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: "telemetry:containers:deleted",
			MaxLen: 500,
			Approx: true,
			Values: map[string]interface{}{
				"value": 1,
			},
		})
	}
}

func saveTopContainers(containers []ContainerInfo) {
	if rdb == nil {
		return
	}

	byRam := make([]ContainerInfo, len(containers))
	copy(byRam, containers)
	sort.Slice(byRam, func(i, j int) bool {
		return byRam[i].RssKb > byRam[j].RssKb
	})

	byCpu := make([]ContainerInfo, len(containers))
	copy(byCpu, containers)
	sort.Slice(byCpu, func(i, j int) bool {
		return dashboardCPU(byCpu[i]) > dashboardCPU(byCpu[j])
	})

	pipe := rdb.Pipeline()

	// Limpiar ZSET y HASH para Top 5
	pipe.Del(ctx, "top:ram")
	pipe.Del(ctx, "top:cpu")
	pipe.Del(ctx, "telemetry:top5:ram")
	pipe.Del(ctx, "telemetry:top5:cpu")
	pipe.Del(ctx, "top:ram:dashboard")
	pipe.Del(ctx, "top:cpu:dashboard")

	if len(containers) == 0 {
		pipe.Exec(ctx)
		return
	}

	for i := 0; i < len(byRam) && i < 5; i++ {
		label := containerLabel(byRam[i])
		// Guardar en ZSET (Nuevas especificaciones)
		pipe.ZAdd(ctx, "top:ram", redis.Z{
			Score:  float64(byRam[i].RssKb),
			Member: label,
		})
		// Guardar en HASH (Compatibilidad)
		pipe.HSet(ctx, "telemetry:top5:ram", label, byRam[i].RssKb)
		pipe.RPush(ctx, "top:ram:dashboard", marshalTopContainer(TopContainerRow{
			Rank: i + 1, Label: fmt.Sprintf("%s (%s) — PID %d", byRam[i].Name, shortContainerID(byRam[i].ID), byRam[i].PID),
			Name: byRam[i].Name, ContainerID: byRam[i].ID,
			PID: byRam[i].PID, Value: float64(byRam[i].RssKb) / 1024,
		}))
	}

	for i := 0; i < len(byCpu) && i < 5; i++ {
		label := containerLabel(byCpu[i])
		// Guardar en ZSET (Nuevas especificaciones)
		pipe.ZAdd(ctx, "top:cpu", redis.Z{
			Score:  dashboardCPU(byCpu[i]),
			Member: label,
		})
		// Guardar en HASH (Compatibilidad)
		pipe.HSet(ctx, "telemetry:top5:cpu", label, dashboardCPU(byCpu[i]))
		pipe.RPush(ctx, "top:cpu:dashboard", marshalTopContainer(TopContainerRow{
			Rank: i + 1, Label: fmt.Sprintf("%s (%s) — PID %d", byCpu[i].Name, shortContainerID(byCpu[i].ID), byCpu[i].PID),
			Name: byCpu[i].Name, ContainerID: byCpu[i].ID,
			PID: byCpu[i].PID, Value: dashboardCPU(byCpu[i]),
		}))
	}

	pipe.Exec(ctx)

	log.Printf("Top 5 RAM (primero): %s | Top 5 CPU (primero): %s",
		formatTopFirst(byRam), formatTopFirst(byCpu))
}

func dashboardCPU(c ContainerInfo) float64 {
	return c.KernelCpuPercent
}

func containerLabel(c ContainerInfo) string {
	return fmt.Sprintf("%s (%s)", c.Name, shortContainerID(c.ID))
}

func shortContainerID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func marshalTopContainer(row TopContainerRow) string {
	b, err := json.Marshal(row)
	if err != nil {
		return `{"rank":0,"label":"(error)","name":"(error)","container_id":"","pid":0,"value":0}`
	}
	return string(b)
}

func formatTopFirst(cs []ContainerInfo) string {
	if len(cs) == 0 {
		return "N/A"
	}
	return fmt.Sprintf("%s(%.2f%%)", cs[0].Name, cs[0].RamPercent)
}

func setupCronjob() {
	absPath, _ := filepath.Abs("../scripts/container_cron.sh")
	cronCmd := fmt.Sprintf("*/2 * * * * bash %s > /dev/null 2>&1", absPath)

	out, _ := exec.Command("crontab", "-l").Output()
	cronLines := string(out)

	if !strings.Contains(cronLines, absPath) {
		newCron := cronLines + cronCmd + "\n"
		cmd := exec.Command("crontab", "-")
		cmd.Stdin = strings.NewReader(newCron)
		cmd.Run()
		log.Printf("Cronjob instalado exitosamente: %s", cronCmd)
	} else {
		log.Println("Cronjob ya registrado en crontab.")
	}
}

func removeCronjob() {
	absPath, _ := filepath.Abs("../scripts/container_cron.sh")
	out, err := exec.Command("crontab", "-l").Output()
	if err != nil {
		return
	}

	lines := strings.Split(string(out), "\n")
	var newLines []string

	for _, line := range lines {
		if !strings.Contains(line, absPath) && strings.TrimSpace(line) != "" {
			newLines = append(newLines, line)
		}
	}

	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(strings.Join(newLines, "\n") + "\n")
	cmd.Run()
	log.Println("Cronjob eliminado del sistema.")
}
