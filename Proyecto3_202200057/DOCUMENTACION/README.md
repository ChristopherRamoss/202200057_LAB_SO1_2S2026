# Correr Grafana
kubectl -n mumnk8s port-forward svc/grafana 3000:3000

# IR A GRAFANA
http://localhost:3000       (usuario admin, contraseña admin).

# Reportes RUST
kubectl -n mumnk8s port-forward svc/rust-api 8000:8000


## ENVIAR REPORTES
for i in $(seq 1 15); do
  curl -s -o /dev/null -X POST localhost:8000/report \
    -H "Content-Type: application/json" \
    -d "{\"country\":\"ESP\",\"warplanes_in_air\":$((RANDOM % 51)),\"warships_in_water\":$((RANDOM % 31)),\"timestamp\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}"
  sleep 4
done
echo listo



# activar Locust
source ~/locust-env/bin/activate
cd ~/Descargas/Eliminar/202200057_LAB_SO1_2S2026/Proyecto3_202200057/locust
locust -f locustfile.py


| Prueba | Usuarios | RPS | Fallos | Mediana | p95 | p99 | Réplicas de Rust |
|--------|----------|-----|--------|---------|-----|-----|------------------|
| Base | 10 | 33.1 | 0% | 5 ms | 10 ms | 14 ms | 1 |
| Carga | 50 | 163.4 | 0% | 4 ms | 13 ms | 32 ms | 3 (HPA) |