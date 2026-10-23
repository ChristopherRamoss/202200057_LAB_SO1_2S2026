# Guía de vueltas — Proyecto 3 M.U.M.N.K8s (carnet 202200057, país asignado: ESP)

**Raíz del proyecto:**
`~/Descargas/Eliminar/202200057_LAB_SO1_2S2026/Proyecto3_202200057`

**Namespace de la app:** `mumnk8s` · **Namespace del Operator:** `mumnk8s-operator-system` · **Namespace de Falco:** `falco`

**Terminales que usaré:**
- **A:** túnel de Grafana (queda abierta)
- **B:** alertas de Falco (queda abierta)
- **C:** comandos de cada vuelta
- **D:** Locust

---

## 0. Preparación (30 minutos antes del turno)

### 0.1 Chequeo general — Terminal C, ruta: cualquiera

```bash
kubectl get nodes
kubectl get pods -A --no-headers | grep -v -E "Running|Completed"
docker ps | grep zot
free -h
```

Esperado:
- Nodo `Ready`.
- El segundo comando **no imprime nada** (ningún pod fuera de `Running`).
- Zot en `Up`.
- `free -h`: columna **disponible** por encima de ~600 MiB.

Si algo no arranca:
- k3s caído: `sudo systemctl start k3s` y esperar 3-5 minutos.
- Zot caído: `docker start zot`.

### 0.2 Terminal A (se queda abierta) — ruta: cualquiera

```bash
kubectl -n mumnk8s port-forward svc/grafana 3000:3000
```

### 0.3 Terminal B (se queda abierta) — ruta: cualquiera

```bash
kubectl -n falco logs -f -l app.kubernetes.io/name=falco -c falco | grep --line-buffered "ALERTA MUMNK8s"
```

### 0.4 Pestañas del navegador

| Pestaña | Dirección | Nota |
|---|---|---|
| 1 | http://localhost:8089 | Locust |
| 2 | https://zot.local:5000 | Zot. Avanzado → Continuar (certificado propio) |
| 3 | http://localhost:3000 | Grafana (usuario `admin`, contraseña `admin`) |

### 0.5 Calentar los datos — Terminal D, ruta: carpeta `locust/`

La serie temporal solo muestra los últimos 15 minutos, así que sin tráfico reciente sale "No data".

```bash
source ~/locust-env/bin/activate
cd ~/Descargas/Eliminar/202200057_LAB_SO1_2S2026/Proyecto3_202200057/locust
locust -f locustfile.py --headless -u 50 -r 5 -t 180s
```

Esperado: al terminar, `# fails` en 0.

---

## Vuelta 1 — Infraestructura, Zot y Gateway API

### 1.1 Terminal C, ruta: cualquiera

```bash
kubectl get nodes -o wide
kubectl get gateway,httproute -n mumnk8s
```

Esperado:
- Un nodo `Ready`.
- Gateway `mumnk8s-gateway` con `PROGRAMMED True` (clase `traefik`).
- HTTPRoute `grpc-202200057`.

### 1.2 Probar la ruta — Terminal C, ruta: cualquiera

```bash
curl -i -X POST http://localhost/grpc-202200057 \
  -H "Content-Type: application/json" \
  -d '{"country":"ESP","warplanes_in_air":42,"warships_in_water":14,"timestamp":"2026-03-12T20:15:30Z"}'
```

Esperado: `HTTP/1.1 200 OK` y `{"status":"forwarded"}`.

### 1.3 Navegador: pestaña 2 (Zot)

Esperado: el catálogo con las imágenes del proyecto (`go-api`, `grpc-client`, `grpc-server`, `rabbit-writer`, `consumer`, `rust-api`, `rabbitmq`, `valkey`, `grafana`, `operator`) y `test/busybox`.


### Por si pide comprobar el artefact
```bash
  oras repo tags --ca-file ~/zot/certs/server.crt zot.local:5000/mumnk8s/config-reportes
  oras manifest fetch --ca-file ~/zot/certs/server.crt zot.local:5000/mumnk8s/config-reportes:v1

```
---

## Vuelta 2 — Locust, API Rust, gRPC (Go) y HPA

### 2.1 Arrancar Locust — Terminal D, ruta: carpeta `locust/`

```bash
source ~/locust-env/bin/activate
cd ~/Descargas/Eliminar/202200057_LAB_SO1_2S2026/Proyecto3_202200057/locust
locust -f locustfile.py
```

### 2.2 Navegador: pestaña 1 (Locust)

Poner **50** usuarios, **5** de ramp up, host `http://localhost` y pulsar **Start**.

Esperado: unos 150-165 RPS y **0% de fallos**.

Conviene iniciarlo 2 minutos antes de mostrar el HPA: tarda 1 a 2 minutos en reaccionar.

### 2.3 Terminal C, ruta: cualquiera (con Locust corriendo)

```bash
kubectl get hpa,deployments -n mumnk8s
kubectl logs -n mumnk8s -l app=api-rust --tail=10
kubectl logs -n mumnk8s deploy/go-deployment1 -c grpc-client --tail=5
kubectl logs -n mumnk8s deploy/go-deployment2 -c grpc-server --tail=5
```

Esperado:
- **HPA `rust-api`:** `TARGETS` por encima de `30%`, `MINPODS 1`, `MAXPODS 3` y `REPLICAS` subiendo hasta 3.
- **Deployments:** todos `READY`.
- **Rust:** líneas `[rust-api] #N PAÍS aviones=.. barcos=.. -> go-api OK`.
- **Cliente gRPC:** líneas `[grpc-client] ... -> SendReport: ok`.
- **Servidor gRPC:** líneas `[grpc-server] reporte recibido: ...`.

### 2.3b Opcional (consumo)

```bash
kubectl top pods -n mumnk8s
```

---

## Vuelta 3 — RabbitMQ, consumer y Operator (Valkey/Grafana)

### 3.1 Terminal C, ruta: cualquiera

```bash
kubectl get crd | grep mumnk8s
kubectl get valkeyinstances,grafanainstances -n mumnk8s
kubectl get pods -n mumnk8s
kubectl get pods -n mumnk8s-operator-system
```

Esperado:
- CRDs `grafanainstances.cache.mumnk8s.io` y `valkeyinstances.cache.mumnk8s.io`.
- Instancias `valkey` y `grafana`.
- Pods de `mumnk8s` (`rabbitmq`, `consumer`, `valkey`, `grafana`, Go y Rust) en `Running`.
- Operator `controller-manager` en `1/1`.

### 3.2 Valkey y RabbitMQ — Terminal C, ruta: cualquiera

```bash
kubectl exec -it deploy/valkey -n mumnk8s -- valkey-cli DBSIZE
kubectl exec -n mumnk8s deploy/valkey -- valkey-cli HGETALL reports:count
kubectl exec -n mumnk8s deploy/rabbitmq -- rabbitmqctl list_queues
```

Esperado:
- `DBSIZE`: **14**. Cuenta claves, no reportes: 4 de máx./mín., 2 de tops, 2 de frecuencias, 1 de conteo por país y 5 series temporales.
- `HGETALL`: los 5 países con su cantidad de reportes.
- `list_queues`: `war_reports` con 0 mensajes, o muy pocos (el consumer va al día).

### 3.3 Persistencia (solo si la piden)

```bash
kubectl delete pod -n mumnk8s -l app=valkey
sleep 20
kubectl get pods -n mumnk8s -l app=valkey
kubectl exec -n mumnk8s deploy/valkey -- valkey-cli DBSIZE
```

Esperado: pod nuevo en `1/1` y `DBSIZE` otra vez en **14** (los datos viven en el PVC).

---

## Vuelta 4 — Grafana, Falco y manual

### 4.1 Navegador: pestaña 3 (Grafana)

Dashboards → tu dashboard. La Terminal A debe estar abierta.

Esperado:
- Máx. y mín. de aviones y barcos.
- Top de países con más aviones y con más barcos.
- Moda de aviones y de barcos.
- País asignado **ESP** y carnet **202200057**.
- Reportes recibidos de ESP.
- Serie temporal de ESP (con datos de los últimos 15 minutos).
- Los números se mueven con el refresco (10 s) mientras haya tráfico.

### 4.2 Falco — Terminal C, ruta: cualquiera (con la Terminal B visible)

```bash
kubectl get pods -n falco
kubectl -n mumnk8s exec deploy/rust-api -- cat /etc/shadow
kubectl -n mumnk8s exec deploy/go-deployment2 -c rabbit-writer -- cat /etc/shadow
```

Esperado:
- Falco en `2/2 Running`.
- En la Terminal C, el contenido de `/etc/shadow`.
- En la **Terminal B**, dos líneas `Warning ALERTA MUMNK8s: lectura de archivo sensible` con el pod, el comando (`cat /etc/shadow`) y el contenedor.

La regla vive en `k8s/falco/mumnk8s-rules.yaml`.

Nota: la regla detecta la **lectura de archivos**. Un `ls /app/secretos` abre un directorio y no la dispara; para la demostración usar `cat /etc/shadow`.

### 4.3 Manual técnico

Abrir `MANUAL_TECNICO.md` en el repositorio de GitHub (guía de instalación, pruebas de carga, HPA y análisis de réplicas).

---

## Si algo falla

| Síntoma | Qué hacer |
|---|---|
| Grafana no carga | Reabrir el túnel de la Terminal A |
| Serie temporal vacía ("No data") | Falta tráfico reciente: correr Locust unos 3 minutos |
| HPA con `REPLICAS 1` | Esperar 1-2 minutos con Locust corriendo |
| Pods reiniciándose tras suspender la PC |