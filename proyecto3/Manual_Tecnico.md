# Manual Técnico - Proyecto #3 M.U.M.N.K8s
**Monitoreo de Unidades Militares en la Nube con Kubernetes**

* **Curso:** Sistemas Operativos 1
* **Universidad:** Universidad de San Carlos de Guatemala (USAC)
* **Facultad:** Facultad de Ingeniería, Escuela de Ciencias y Sistemas
* **Estudiante:** Angel Cárdenas
* **Carnet:** `202304063` (Último dígito: `3` ➔ **País Asignado: RUS (Rusia)**)
* **Ruta Gateway API Asignada:** `/grpc-202304063`

---

## 1. Arquitectura General y Flujo de Datos

El sistema implementa una arquitectura distribuida orientada a microservicios altamente escalable desplegada en **Google Cloud Platform (GCP)** utilizando un clúster de **Google Kubernetes Engine (GKE)** con nodos de cómputo N1 y un registro privado **Zot OCI Registry** fuera del clúster.

```
                  [ Locust (Generador de Carga) ]
                               │
                               │ HTTP POST (Reportes Militares JSON)
                               ▼
            [ Kubernetes Gateway API (136.82.83.245) ]
                     Ruta: /grpc-202304063
                               │
                               ▼
        ┌──────────────────────────────────────────────┐
        │        API REST en Rust (Axum Tokio)         │
        │      HPA Escalable (1-3 réplicas > 30% CPU)  │
        └──────────────────────┬───────────────────────┘
                               │ HTTP Interno
                               ▼
        ┌──────────────────────────────────────────────┐
        │    Deployment Go #1 (Pod Multi-contenedor)   │
        │   • Contenedor 1: Ingesta REST (Port 8081)   │
        │   • Contenedor 2: Cliente gRPC (Port 8082)   │
        └──────────────────────┬───────────────────────┘
                               │ gRPC (Protobuf / HTTP2)
                               ▼
        ┌──────────────────────────────────────────────┐
        │    Deployment Go #2 (Pod Multi-contenedor)   │
        │   • Contenedor 1: Servidor gRPC (Port 50051) │
        │   • Contenedor 2: Writer RabbitMQ (Port 8083)│
        └──────────────────────┬───────────────────────┘
                               │ AMQP (Mensajes Persistentes)
                               ▼
        ┌──────────────────────────────────────────────┐
        │             RabbitMQ Message Broker          │
        │            (Cola: war_reports_queue)         │
        └──────────────────────┬───────────────────────┘
                               │ AMQP Consumo en Tiempo Real
                               ▼
        ┌──────────────────────────────────────────────┐
        │             Consumer en Go                   │
        │   Procesa Máximos, Mínimos, Modas, Tops      │
        │   Filtra país asignado: RUS                  │
        └──────────────────────┬───────────────────────┘
                               │ TCP
                               ▼
        ┌──────────────────────────────────────────────┐
        │           Valkey Cluster (Key-Value)         │
        │     Desplegado vía OPERATOR SDK (Helm)       │
        │     Persistencia: PVC 1Gi (standard-rwo)     │
        └──────────────────────┬───────────────────────┘
                               │ Redis Data Source
                               ▼
        ┌──────────────────────────────────────────────┐
        │              Grafana Dashboard               │
        │     Desplegado vía OPERATOR SDK (Helm)       │
        │     Visualización en tiempo real (RUS)       │
        └──────────────────────────────────────────────┘
```

---

## 2. Tecnologías y Componentes

### 2.1 API Gateway (Kubernetes Gateway API)
En sustitución del tradicional Ingress Controller, se implementó el estándar oficial de **Gateway API** con la clase `gke-l7-global-external-managed`.
* **IP Pública Global:** `136.82.83.245`
* **Ruta de Ingesta:** `/grpc-202304063`
* **Ruta de Salud (Health Check):** `/healthz`

### 2.2 API REST en Rust
Desarrollada en Rust con el framework asíncrono **Axum** y runtime **Tokio**.
* Valida que el país pertenezca estrictamente a los aceptados: `USA`, `RUS`, `CHN`, `ESP`, `GTM`.
* Reenvía asíncronamente las solicitudes al deployment de Go.
* **Auto-escalado Horizontal (HPA):** Configurado de 1 a 3 réplicas cuando el consumo promedio de CPU supera el 30%.

### 2.3 Servicios de Ingesta y Procesamiento en Go
* **Go Deployment 1:**
  * **Contenedor 1 (REST Ingestion):** Recibe el payload JSON validado de Rust.
  * **Contenedor 2 (Cliente gRPC):** Transforma el JSON a formato binario estructurado según el contrato `wartweets.proto` y realiza la llamada RPC a través de un canal gRPC persistente.
* **Go Deployment 2:**
  * **Contenedor 1 (Servidor gRPC):** Implementa la interfaz RPC `SendReport`.
  * **Contenedor 2 (RabbitMQ Writer):** Publica el reporte militar con entrega persistente (`DeliveryMode: amqp.Persistent`) en la cola `war_reports_queue`.

### 2.4 Message Broker (RabbitMQ) y Consumidor
* RabbitMQ garantiza el desacoplamiento total y tolerancia a fallos ante picos masivos de tráfico.
* El **Consumer en Go** extrae los mensajes en tiempo real y calcula métricas directamente en **Valkey**:
  * `total_reports`: Conteo global.
  * `max_warplanes`, `min_warplanes`: Extremos globales de aviones en aire.
  * `max_warships`, `min_warships`: Extremos globales de barcos en mar.
  * `freq_warplanes`, `freq_warships`: Tablas hash para el cálculo de la moda estadística.
  * `top_countries_warplanes`, `top_countries_warships`: Conjuntos ordenados (*Sorted Sets*) para el ranking.
  * `timeseries_assigned`: Lista con registros temporales para el país asignado (**RUS**).

### 2.5 Operator de Kubernetes (Helm-based) para Valkey y Grafana
Construido utilizando **Operator SDK v1.39.1** bajo el patrón de controlador basado en Helm:
* **CRD 1: `ValkeyInstance`:** Reconcilia automáticamente el despliegue de Valkey, el `Service` de conexión interna y un `PersistentVolumeClaim` (PVC) de 1Gi sobre almacenamiento `standard-rwo`.
* **CRD 2: `GrafanaInstance`:** Reconcilia el despliegue de Grafana, el plugin oficial de Valkey/Redis y un servicio tipo `LoadBalancer` expuesto en la IP pública `34.132.42.217:3000`.

---

## 3. Zot OCI Container Registry
* Desplegado en una VM independiente en Google Compute Engine fuera del clúster de Kubernetes (`34.59.251.34:5000`).
* Configurado con certificados TLS/HTTPS y soporte de artefactos y esquemas **OCI**.
* Todas las imágenes de los microservicios fueron compiladas y descargadas exclusivamente desde este registro.

---

## 4. Pruebas de Carga con Locust y Verificación HPA

El script `locustfile.py` simula usuarios concurrentes generando reportes militares con las reglas de negocio:
* Aviones en aire: entre 0 y 50.
* Barcos en mar: entre 0 y 30.
* Timestamp en estándar ISO 8601 UTC.
* Destino: `http://136.82.83.245/grpc-202304063`
