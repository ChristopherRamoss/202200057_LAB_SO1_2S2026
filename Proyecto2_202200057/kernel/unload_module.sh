#!/bin/bash
# Descarga el modulo de kernel. Se llama al finalizar el daemon (opcional).
sudo rmmod continfo 2>/dev/null || echo "[unload_module] continfo no estaba cargado."
