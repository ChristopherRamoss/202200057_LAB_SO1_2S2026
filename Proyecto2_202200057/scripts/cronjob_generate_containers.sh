#!/bin/bash
set -e

INTRUDER_IMAGE="proyecto2-intruder:latest"

# Alto consumo: imagen roldyoran/go-client
run_high() {
    docker run -d --label proyecto2=true --label proyecto2_profile=high \
        roldyoran/go-client >/dev/null
}

# Bajo consumo: alpine sleep 240
run_low() {
    docker run -d --label proyecto2=true --label proyecto2_profile=low \
        alpine sleep 240 >/dev/null
}

NEW_HIGH=0
NEW_LOW=0

# 5 contenedores elegidos al azar entre alto y bajo consumo
# (ojo: con "set -e" NO usar ((var++)), devuelve 1 cuando var vale 0 y aborta el script)
for i in $(seq 1 5); do
    if [ $((RANDOM % 2)) -eq 0 ]; then
        run_high
        NEW_HIGH=$((NEW_HIGH + 1))
    else
        run_low
        NEW_LOW=$((NEW_LOW + 1))
    fi
done

# Contenedor intruso: se crea siempre en cada corrida
docker run -d --label proyecto2=true --label proyecto2_intruder=true \
    "$INTRUDER_IMAGE" >/dev/null

echo "[cronjob] $(date): 5 contenedores + 1 intruso generados."
echo "[cronjob] Creados en esta corrida: ${NEW_HIGH} alto consumo, ${NEW_LOW} bajo consumo, 1 intruso."

# Conteo REAL de lo que hay vivo ahora mismo (incluye los de corridas anteriores),
# clasificado por imagen, la misma regla que usa el daemon.
IMAGES=$(docker ps --filter label=proyecto2=true --format '{{.Image}}')
ALIVE_HIGH=$(grep -c 'go-client' <<< "$IMAGES" || true)
ALIVE_LOW=$(grep -c 'alpine' <<< "$IMAGES" || true)
ALIVE_INTRUDER=$(grep -c 'intruder' <<< "$IMAGES" || true)

echo "[cronjob] Vivos ahora: ${ALIVE_HIGH} alto consumo, ${ALIVE_LOW} bajo consumo, ${ALIVE_INTRUDER} intruso(s)."