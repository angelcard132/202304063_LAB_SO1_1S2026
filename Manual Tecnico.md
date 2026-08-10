# Manual Técnico y Arquitectura de Infraestructura

**Estudiante:** Angel Jose Cardenas Pocon
**Carnet:** 202304063
**Curso:** Sistemas Operativos 1
**Semestre:** 2S 2026

---

## 1. Arquitectura General del Sistema

El sistema implementa un entorno distribuido virtualizado sobre hipervisor KVM (Kernel-based Virtual Machine) que integra múltiples motores de ejecución de contenedores (Docker, Containerd, Podman), un registro distribuido OCI (Zot Registry) y microservicios REST desarrollados en lenguaje Go.


---

## 2. Especificacion de Máquinas Virtuales y Runtimes

| Máquina Virtual | Dirección IP | Runtime | Servicios Alojados | Nombres de Contenedor | Puertos |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **VM1** | `192.168.122.50` | Containerd / Docker | Zot Registry, API1, API2 | `zot-registry`, `api1`, `api2` | `5000`, `8081`, `8082` |
| **VM2** | `192.168.122.213` | Podman | Cliente de Pruebas | N/A | N/A |
| **VM3** | `192.168.122.227` | Docker | API3 | `api3` | `8083` |

---

## 3. Almacenamiento de Imagenes y Registro Zot

El registro privado OCI **Zot** opera en la **VM1** (`192.168.122.50:5000`). Todas las imágenes fueron construidas respetando la convención de nombrado exigida `api#-202304063:latest` y publicadas en Zot:

* `192.168.122.50:5000/api1-202304063:latest`
* `192.168.122.50:5000/api2-202304063:latest`
* `192.168.122.50:5000/api3-202304063:latest`

### Consulta de Catálogo OCI (Verificación Zot):
```bash
curl -i http://192.168.122.50:5000/v2/_catalog
```
**Respuesta JSON:**
```json
{
  "repositories": [
    "api1-202304063",
    "api2-202304063",
    "api3-202304063"
  ]
}
```

---

## 4. Estructura de APIs y Endpoints

Las 3 APIs están desarrolladas en **Go** utilizando `net/http` estándar y respetando los esquemas de respuesta JSON solicitados.

### 4.1 Endpoints de Salud (`/health`)
Cada API expone un endpoint GET `/health` que retorna la disponibilidad del servicio.

* **Formato de Respuesta (`200 OK`):**
```json
{
  "status": "UP",
  "message": "API# is Ready",
  "timestamp": "2026-08-09T19:00:00Z",
  "VM": "VM#",
  "carnet": "202304063"
}
```

### 4.2 Endpoints de Comunicación Cruzada (Cross-Calls)
Cada API puede contactar el endpoint `/health` de sus servicios pares mediante cliente HTTP.

* **Formato de Respuesta Éxito (`connection: true`):**
```json
{
  "apiname": "API#",
  "message": "The API# located on the VM# is working",
  "connection": true,
  "carnet": "202304063"
}
```

* **Formato de Respuesta Fallo (`connection: false`):**
```json
{
  "apiname": "API#",
  "message": "ERROR: The API# located on the VM# is not working",
  "connection": false,
  "carnet": "202304063"
}
```

---

## 5. Matriz Completa de Verificación y Respuestas Obtener en Pruebas

| Origen (VM / Cliente) | Método | URL / Endpoint Evaluado | Estado HTTP | Resultado JSON Esperado |
| :--- | :--- | :--- | :--- | :--- |
| **VM2** (Podman) | `GET` | `http://192.168.122.50:8081/health` | `200 OK` | `{"status":"UP","message":"API1 is Ready","VM":"VM1","carnet":"202304063"}` |
| **VM2** (Podman) | `GET` | `http://192.168.122.50:8082/health` | `200 OK` | `{"status":"UP","message":"API2 is Ready","VM":"VM1","carnet":"202304063"}` |
| **VM2** (Podman) | `GET` | `http://192.168.122.227:8083/health` | `200 OK` | `{"status":"UP","message":"API3 is Ready","VM":"VM3","carnet":"202304063"}` |
| **VM2** ➔ **API1** | `GET` | `http://192.168.122.50:8081/api1/202304063/call-api2` | `200 OK` | `{"apiname":"API2","message":"The API2 located on the VM1 is working","connection":true,"carnet":"202304063"}` |
| **VM2** ➔ **API1** | `GET` | `http://192.168.122.50:8081/api1/202304063/call-api3` | `200 OK` | `{"apiname":"API3","message":"The API3 located on the VM3 is working","connection":true,"carnet":"202304063"}` |
| **VM2** ➔ **API2** | `GET` | `http://192.168.122.50:8082/api2/202304063/call-api1` | `200 OK` | `{"apiname":"API1","message":"The API1 located on the VM1 is working","connection":true,"carnet":"202304063"}` |
| **VM2** ➔ **API2** | `GET` | `http://192.168.122.50:8082/api2/202304063/call-api3` | `200 OK` | `{"apiname":"API3","message":"The API3 located on the VM3 is working","connection":true,"carnet":"202304063"}` |
| **VM2** ➔ **API3** | `GET` | `http://192.168.122.227:8083/api3/202304063/call-api1` | `200 OK` | `{"apiname":"API1","message":"The API1 located on the VM1 is working","connection":true,"carnet":"202304063"}` |
| **VM2** ➔ **API3** | `GET` | `http://192.168.122.227:8083/api3/202304063/call-api2` | `200 OK` | `{"apiname":"API2","message":"The API2 located on the VM1 is working","connection":true,"carnet":"202304063"}` |

---

## 6. Mantenimiento y Comandos de Control

### Iniciar la Infraestructura Completa desde Host PC:
```bash
# Encender el servicio KVM libvirtd
sudo systemctl start libvirtd

# Encender las tres VMs
sudo virsh start VM1
sudo virsh start VM2
sudo virsh start VM3
```

### Gestión de Contenedores en VM1:
```bash
sudo docker start zot-registry api1 api2
```

### Gestión de Contenedores en VM3:
```bash
sudo docker start api3
```