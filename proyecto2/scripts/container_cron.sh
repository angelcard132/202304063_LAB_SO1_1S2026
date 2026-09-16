#!/bin/bash
# Script de Cronjob para simulación de contenedores (SO1 Proyecto 2 - Carnet 202304063)

# 1. Crear 5 contenedores de bajo / alto consumo (despliegue aleatorio)
TIMESTAMP=$(date +%s)
for i in {1..5}; do
    TYPE=$((RANDOM % 2))
    if [ $TYPE -eq 0 ]; then
        docker run -d --name "test_container_0_${TIMESTAMP}_${i}" alpine sh -c "while true; do echo '2^20' | bc > /dev/null; sleep 2; done" > /dev/null 2>&1
    else
        docker run -d --name "test_container_1_${TIMESTAMP}_${i}" roldyoran/go-client > /dev/null 2>&1
    fi
done

# 2. Crear contenedor intruso
docker run -d --name "intruder_container_${TIMESTAMP}" roldyoran/intruder > /dev/null 2>&1
