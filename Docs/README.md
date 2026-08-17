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


# GUIA DE COMANDOS 
## PASO 1: En la PC Física (HOST)

Abre la terminal en tu PC física (como superusuario `sudo su`):

### 1. Iniciar el servicio de Máquinas Virtuales (KVM):
```bash
systemctl start libvirtd
```
```bash
sudo virsh net-start default
```


### 2. Encender las 3 Máquinas Virtuales:
```bash
virsh start VM1
virsh start VM2
virsh start VM3
```
O abrir el gestor de maquinas virtuales de linux (virt-manager) y encender las maquinas virtuales de forma manual.
---

## PASO 2: En la VM1 (`192.168.122.50`)

Abrir la terminal dentro de **VM1**:

### 1. Iniciar el servicio de Docker:
```bash
systemctl start docker
```

### 2. Iniciar el Registro Privado Zot:
```bash
docker start zot-registry
```

### 3. Iniciar los contenedores de API1 y API2:
```bash
docker start api1 api2
```
Si los contenedores fueron borrados o algo por el estilo, colocar esto:
```bash
docker run -d --name api1 -p 8081:8081 -e API2_URL="http://192.168.122.50:8082" -e API3_URL="http://192.168.122.227:8083" 192.168.122.50:5000/api1-202304063:latest
docker run -d --name api2 -p 8082:8082 -e API1_URL="http://192.168.122.50:8081" -e API3_URL="http://192.168.122.227:8083" 192.168.122.50:5000/api2-202304063:latest
```

---

## PASO 3: En la VM3 (`192.168.122.227`)

Abre la terminal dentro de **VM3**:

### 1. Iniciar el servicio de Docker:
```bash
systemctl start docker
```

### 2. Iniciar el contenedor de API3:
```bash
docker start api3
```
Si los contenedores fueron borrados o algo por el estilo, colocar esto:
```bash
docker run -d --name api3 -p 8083:8083 -e API1_URL="http://192.168.122.50:8081" -e API2_URL="http://192.168.122.50:8082" 192.168.122.50:5000/api3-202304063:latest
```

---

## PASO 4: En la VM2 (`192.168.122.213`) - Pruebas Completa (Cliente)

Abre la terminal en **VM2**:

### A. Verificar el Registro Zot (en VM1 desde VM2):
```bash
curl -i http://192.168.122.50:5000/v2/_catalog
```

### B. Pruebas de Salud (Health Checks):
```bash
# 1. Estado de API1
curl -i http://192.168.122.50:8081/health

# 2. Estado de API2
curl -i http://192.168.122.50:8082/health

# 3. Estado de API3
curl -i http://192.168.122.227:8083/health
```

### C. Pruebas de Comunicación Cruzada entre APIs:

#### Desde API1 (VM1):
```bash
# Llamar a API2
curl -i http://192.168.122.50:8081/api1/202304063/call-api2

# Llamar a API3
curl -i http://192.168.122.50:8081/api1/202304063/call-api3
```

#### Desde API2 (VM1):
```bash
# Llamar a API1
curl -i http://192.168.122.50:8082/api2/202304063/call-api1

# Llamar a API3
curl -i http://192.168.122.50:8082/api2/202304063/call-api3
```

#### Desde API3 (VM3):
```bash
# Llamar a API1
curl -i http://192.168.122.227:8083/api3/202304063/call-api1

# Llamar a API2
curl -i http://192.168.122.227:8083/api3/202304063/call-api2
```

---

## Respuestas Esperadas

En todas las pruebas de llamada cruzada (`call-api`), debes recibir una respuesta JSON con código `HTTP/1.1 200 OK` y `"connection": true`:

```json
{
  "apiname": "API#",
  "message": "The API# located on the VM# is working",
  "connection": true,
  "carnet": "202304063"
}
```

