# Guía de Instalación - Proyecto 1 

**Estudiante:** Angel Jose Cardenas Pocon  
**Carnet:** 202304063  
**Curso:** Sistemas Operativos 1  
**Semestre:** 2S 2026  

---

## Descripcion General
Este proyecto implementa un entorno distribuido virtualizado con KVM utilizando tres Máquinas Virtuales que alojan distintas APIs escritas en Go y gestionadas mediante diferentes runtimes y un registro privado Zot.

---

## Infraestructura y Direcciones IP

| Máquina Virtual | IP | Runtime / Herramienta | Servicios / APIs Alojadas | Puertos |
| :--- | :--- | :--- | :--- | :--- |
| **VM1** | `192.168.122.50` | Containerd / Docker + Zot | `Zot Registry`, `API1`, `API2` | Zot: 5000<br>API1: 8081<br>API2: 8082 |
| **VM2** | `192.168.122.213` | Podman | Cliente de Pruebas (`curl`, `podman`) | N/A |
| **VM3** | `192.168.122.227` | Docker | `API3` | API3: 8083 |

---

## Guia de Instalacion Paso a Paso

### 1. Requisitos Previos en la PC Host
Instalar KVM, QEMU, libvirt y virt-manager:
```bash
sudo apt update && sudo apt install -y qemu-kvm libvirt-daemon-system libvirt-clients bridge-utils virt-manager virtinst
sudo systemctl enable --now libvirtd
```

### 2. Configuración de VM1 (`192.168.122.50`)
1. Instalar Ubuntu Server 22.04 LTS en KVM con el nombre `VM1`.
2. Instalar Docker y Containerd:
   ```bash
   sudo apt update && sudo apt install -y docker.io containerd curl
   sudo systemctl enable --now docker
   ```
3. Iniciar el Registro Privado Zot:
   ```bash
   sudo docker run -d -p 5000:5000 --name zot-registry ghcr.io/project-zot/zot-linux-amd64:latest
   ```
4. Configurar registro inseguro en `/etc/docker/daemon.json`:
   ```json
   {
     "insecure-registries": ["192.168.122.50:5000", "localhost:5000"]
   }
   ```
   Reiniciar docker:
   ```bash
   sudo systemctl restart docker
   sudo docker start zot-registry
   ```
5. Desplegar API1 y API2 en VM1:
   ```bash
   sudo docker run -d --name api1 -p 8081:8081 -e API2_URL="http://192.168.122.50:8082" -e API3_URL="http://192.168.122.227:8083" 192.168.122.50:5000/api1-202304063:latest
   sudo docker run -d --name api2 -p 8082:8082 -e API1_URL="http://192.168.122.50:8081" -e API3_URL="http://192.168.122.227:8083" 192.168.122.50:5000/api2-202304063:latest
   ```

---

### 3. Configuración de VM3 (`192.168.122.227`)
1. Instalar Ubuntu Server 22.04 LTS en KVM con el nombre `VM3`.
2. Instalar Docker:
   ```bash
   sudo apt update && sudo apt install -y docker.io curl
   sudo systemctl enable --now docker
   ```
3. Permitir el registro Zot en `/etc/docker/daemon.json`:
   ```json
   {
     "insecure-registries": ["192.168.122.50:5000"]
   }
   ```
   Reiniciar docker:
   ```bash
   sudo systemctl restart docker
   ```
4. Desplegar API3 en VM3:
   ```bash
   sudo docker run -d --name api3 -p 8083:8083 -e API1_URL="http://192.168.122.50:8081" -e API2_URL="http://192.168.122.50:8082" 192.168.122.50:5000/api3-202304063:latest
   ```

---

### 4. Configuración de VM2 (`192.168.122.213`)
1. Instalar Ubuntu Server 22.04 LTS en KVM con el nombre `VM2`.
2. Instalar Podman y Curl:
   ```bash
   sudo apt update && sudo apt install -y podman curl
   ```

---

## Pruebas de Verificacion desde VM2

Ejecutar las peticiones en la terminal de **VM2**:

```bash
# Pruebas de Estado 
curl -i http://192.168.122.50:8081/health
curl -i http://192.168.122.50:8082/health
curl -i http://192.168.122.227:8083/health

# Pruebas de Interconexión 
curl -i http://192.168.122.50:8081/api1/202304063/call-api2
curl -i http://192.168.122.50:8081/api1/202304063/call-api3

curl -i http://192.168.122.50:8082/api2/202304063/call-api1
curl -i http://192.168.122.50:8082/api2/202304063/call-api3

curl -i http://192.168.122.227:8083/api3/202304063/call-api1
curl -i http://192.168.122.227:8083/api3/202304063/call-api2
```
