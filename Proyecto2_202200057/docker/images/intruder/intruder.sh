#!/bin/sh


echo "[INTRUDER] Iniciando intento de lectura de archivos sensibles..."

for f in /etc/passwd /etc/shadow /root/.ssh/id_rsa /etc/hostname; do
    if [ -r "$f" ]; then
        echo "[INTRUDER] Acceso logrado a $f"
        head -n 2 "$f" 2>/dev/null
    else
        echo "[INTRUDER] Sin permisos sobre $f"
    fi
    sleep 1
done

echo "[INTRUDER] Escaneo finalizado, permaneciendo activo..."
sleep 300
