# Comandos de Ejecución y Pruebas - Proyecto 2 SO1 (Carnet: 202304063)

## 1. Levantar la infraestructura de Valkey y Grafana (Docker Compose)

```bash
docker-compose up -d
```

## 2. Compilar el Módulo de Kernel

```bash
cd kernel_module
make
```

## 3. Compilar la Sonda eBPF

```bash
cd ../ebpf
clang -O2 -g -target bpf -c kill_probe.bpf.c -o kill_probe.bpf.o
```

## 4. Compilar y Ejecutar el Daemon de Go

```bash
cd ../daemon_go
go build -o daemon .
sudo ./daemon
```

## 5. Probar el script de Cronjob manualmente (opcional)

```bash
bash ../scripts/container_cron.sh
```

## 6. Probar la sonda eBPF interceptando kills manualmente

```bash
sudo kill -9 <PID_DE_PRUEBA>
```
