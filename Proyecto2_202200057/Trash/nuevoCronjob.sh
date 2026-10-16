#!/bin/bash
set -e

INTRUDER_IMAGE="proyecto2-intruder:latest"

run_high() {
    docker run -d --label proyecto2=true --label proyecto2_profile=high \
        roldyoran/go-client >/dev/null
    echo "[cronjob] Creado contenedor de ALTO consumo (go-client)"
}

run_low() {
    docker run -d --label proyecto2=true --label proyecto2_profile=low \
        alpine sleep 240 >/dev/null
    echo "[cronjob] Creado contenedor de BAJO consumo (alpine sleep)"
}

# 1. Crear 3 de bajo consumo
for i in {1..3}; do run_low; done

# 2. Crear 2 de alto consumo
for i in {1..2}; do run_high; done

# 3. Crear el contenedor intruso
docker run -d --label proyecto2=true --label proyecto2_intruder=true \
    "$INTRUDER_IMAGE" >/dev/null
echo "[cronjob] Creado contenedor INTRUSO"

echo "[cronjob] $(date): Lote de 6 contenedores generado con exito."