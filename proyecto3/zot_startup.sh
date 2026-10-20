#!/bin/bash
set -e

# Actualizar paquetes y dependencias básicas
apt-get update
apt-get install -y ca-certificates curl jq openssl

# Descargar binario de Zot (última versión estable x86_64)
curl -sSL https://github.com/project-zot/zot/releases/download/v2.1.1/zot-linux-amd64 -o /usr/local/bin/zot
chmod +x /usr/local/bin/zot

# Crear directorios para certificados y almacenamiento de imágenes
mkdir -p /etc/zot/certs /var/lib/zot

# Generar certificado SSL autofirmado para HTTPS (válido para IP y localhost)
EXTERNAL_IP=$(curl -s -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip || echo "127.0.0.1")

openssl req -newkey rsa:2048 -nodes -sha256 -keyout /etc/zot/certs/server.key -x509 -days 365 -out /etc/zot/certs/server.crt \
    -subj "/CN=${EXTERNAL_IP}" \
    -addext "subjectAltName=IP:${EXTERNAL_IP},IP:127.0.0.1,DNS:localhost"

# Configuración de Zot con HTTPS
cat <<EOF > /etc/zot/config.json
{
  "distSpecVersion": "1.1.0",
  "storage": {
    "rootDirectory": "/var/lib/zot"
  },
  "http": {
    "address": "0.0.0.0",
    "port": "5000",
    "tls": {
      "cert": "/etc/zot/certs/server.crt",
      "key": "/etc/zot/certs/server.key"
    }
  },
  "log": {
    "level": "info"
  }
}
EOF

# Crear servicio Systemd para Zot
cat <<EOF > /etc/systemd/system/zot.service
[Unit]
Description=Zot OCI Registry Server
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/zot serve /etc/zot/config.json
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# Iniciar y habilitar servicio Zot
systemctl daemon-reload
systemctl enable zot
systemctl start zot
