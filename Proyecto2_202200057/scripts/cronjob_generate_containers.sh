#!/bin/bash
# Se ejecuta cada 2 minutos (via cron, instalado por el daemon de Go).
# Crea 5 contenedores eligiendo aleatoriamente entre los 3 perfiles de carga,
# y con 1/3 de probabilidad agrega tambien el contenedor intruso.

set -e

INTRUDER_IMAGE="proyecto2-intruder:latest"

run_go_client() {
    docker run -d --label proyecto2=true --label proyecto2_profile=high \
        roldyoran/go-client >/dev/null
}

run_high_cpu() {
    docker run -d --label proyecto2=true --label proyecto2_profile=high \
        alpine sh -c "while true; do echo '2^20' | bc > /dev/null; sleep 2; done" >/dev/null
}

run_low() {
    docker run -d --label proyecto2=true --label proyecto2_profile=low \
        alpine sleep 240 >/dev/null
}

PROFILES=(run_go_client run_high_cpu run_low)

for i in $(seq 1 5); do
    IDX=$((RANDOM % ${#PROFILES[@]}))
    ${PROFILES[$IDX]}
done

# ~1 de cada 3 corridas del cron tambien lanza el contenedor intruso
if [ $((RANDOM % 3)) -eq 0 ]; then
    docker run -d --label proyecto2=true --label proyecto2_intruder=true \
        "$INTRUDER_IMAGE" >/dev/null
    echo "[cronjob] $(date): 5 contenedores + 1 intruso generados."
else
    echo "[cronjob] $(date): 5 contenedores generados."
fi
