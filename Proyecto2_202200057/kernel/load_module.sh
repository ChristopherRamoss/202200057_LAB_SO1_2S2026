#!/bin/bash
# Compila y carga el modulo de kernel. El daemon de Go llama a este script.
set -e

CARNET="${CARNET:-000000000}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

cd "$SCRIPT_DIR"

# Si ya esta cargado, no hacer nada (idempotente)
if lsmod | grep -q "^continfo"; then
    echo "[load_module] continfo ya esta cargado."
    exit 0
fi

make CARNET="$CARNET" >/dev/null
sudo insmod continfo.ko
echo "[load_module] Modulo cargado -> /proc/continfo_pr2_so1_$CARNET"
