# Guía de Calificación y Ejecución - Proyecto 1

**Carnet:** 202200057  
**Entorno:** Linux Mint / KVM / QEMU  
**Estructura de Red:**
* **VM1 (Containerd):** `192.168.122.87` (API1: Port 8081 | API2: Port 8082)
* **VM2 (Podman):** `192.168.122.56` (API3: Port 8083)
* **VM3 (Docker):** `192.168.122.118` (Zot Registry: Port 5000)


# Conexiones ssh
- ssh vm1_containerd@192.168.122.87
- ssh vm2_podman@192.168.122.56
- ssh vm3_docker@192.168.122.118

---

## 1. Preparación del Entorno

### 1.1 Validación de Máquinas Virtuales y KVM
Abrir **Virtual Machine Manager** 
Abrir una terminal en Linux Mint y escribir:
- virsh list --all

### 1.2 Verificacion de Runtimes

VM3 Verificar docker
- docker info 
- docker -v 

VM2 Verificar Podman
- podman info 
- podman -v
- docker info #Sin info

VM1 Verificar Containerd
- sudo ctr namespace ls 
- sudo systemctl status containerd
- docker info #Sin info


## 2. Desarrollo y contenerizacion

### 2.1 Desarrollo de apis en go 

Vm3 verificar docker
- docker ps

VM2 Verificar podman
- podman ps
- podman start api3_container #PorSiEstaApagada

VM1 Verificar Containerd listarlos
- sudo ctr -n default containers ls #[ API#-#CARNET ]


### Endpoints /health

VM2 probar api3
- curl -s http://localhost:8083/health | jq .

VM1 pribar api1 y api2

- curl -s http://localhost:8081/health | jq .

- curl -s http://localhost:8082/health | jq .


### Creacion dockerfile y construccion imagenes

VM1 containerd 
- sudo ctr -n default images ls

VM2 Podman
- podman images

VM3 Docker
- docker images #(cat Dockerfile)

### Registros y comunicacion cruzada
## 3.1 -  3.2  Registry y descarga de imagenes

VM1 containerd 
- curl http://192.168.122.118:5000/v2/_catalog

VM2 Podman
- curl http://192.168.122.118:5000/v2/_catalog

VM3 Docker
- curl http://localhost:5000/v2/_catalog
- o tambien
- curl http://192.168.122.118:5000/v2/_catalog


### 3.3 - 4.1 Comunicacion apis


# Pruebas a API1
- curl -s http://192.168.122.87:8081/api1-202200057/call-api2
- curl -s http://192.168.122.87:8081/api1-202200057/call-api3

# Pruebas a API2
- curl -s http://192.168.122.87:8082/api2-202200057/call-api1
- curl -s http://192.168.122.87:8082/api2-202200057/call-api3

# Pruebas a API3 (VM2) 
- curl -s http://192.168.122.56:8083/api3-202200057/call-api1
- curl -s http://192.168.122.56:8083/api3-202200057/call-api2
