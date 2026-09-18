# prueba


sudo insmod continfo.ko
cat /proc/continfo_pr2_so1_202200057 | head -20
sudo rmmod continfo

FUNCIONA


# Manual Técnico — Proyecto 2, Sistemas Operativos 1

Cual va a ser la linea de trabajo

**Autor:** Christopher — USAC
**Carnet:** 202200057

## 1. Introducción

## 2. Arquitectura
Daemon → Grafana / Cronjob / Módulo de kernel / eBPF → Valkey)_

## 3. Módulo de kernel
- Cómo captura RAM y procesos (`task_struct`, `get_cmdline`, `get_task_mm`)
- Formato del archivo `/proc/continfo_pr2_so1_<CARNET>`
- Decisiones de diseño (ej. por qué dos pasadas sobre la lista de procesos)

## 4. Sonda eBPF
- Qué syscall intercepta y por qué
- Cómo se comunica con el daemon (ring buffer)
- Cómo se confirma la eliminación de un contenedor

## 5. Daemon de Go
- Ciclo de vida (inicio, cronjob, carga de módulo, loop, finalización)
- Lógica de decisión: cómo ordena y elige qué contenedores matar
- Cómo garantiza las restricciones (mínimo 3 bajo consumo, 2 alto consumo,
  nunca tocar Grafana)
- Estructura de datos guardada en Valkey

## 6. Dashboard de Grafana
- Capturas de cada panel con datos reales
- Explicación breve de cada query

## 7. Guía de instalación paso a paso
_(comandos exactos para replicar desde cero: dependencias, compilación,
levantar servicios, cargar el módulo, correr el daemon)_

## 8. Evidencia funcional
_(capturas: módulo cargado, /proc con datos, contenedores generándose,
daemon matando un contenedor, log de confirmación eBPF, dashboard completo)_

## 9. Decisiones y aprendizajes
_(cualquier cosa que tuviste que ajustar respecto al diseño original — por
ejemplo el umbral de alto/bajo consumo, o ajustes al struct del tracepoint eBPF)_


# Manual Técnico: Sistema de Monitoreo y Defensa con eBPF y Go

## 1. Descripción General del Sistema
Este sistema está diseñado para la detección y mitigación automatizada de eventos a nivel de kernel utilizando una sonda eBPF, un daemon de control desarrollado en Go, almacenamiento en memoria con Valkey y visualización de métricas en tiempo real mediante Grafana.

---

## 2. Requisitos del Sistema y Arquitectura
* **Arquitectura:** x86
* **Sistema Operativo:** Linux (compatible con compilación de módulos de kernel y soporte para eBPF/BCC o libbpf).
* **Dependencias:**
  * Docker y Docker Compose
  * Go (para la compilación del daemon)
  * Clang / LLVM (para la compilación de la sonda eBPF)

---

## 3. Estructura del Proyecto y Componentes

### 3.1. Contenedores y Servicios (Docker Compose)
El entorno levanta servicios auxiliares para la persistencia de datos y la monitorización:
* **Valkey:** Base de datos en memoria utilizada para almacenar el estado y las métricas recopiladas por el daemon.
* **Grafana:** Panel de visualización conectado a las fuentes de datos para graficar la actividad del sistema.

### 3.2. Módulo de Kernel y Sonda eBPF (`continfo`)
* **Módulo `continfo`:** Se encarga de engancharse a las llamadas al sistema (syscalls), específicamente interceptando `sys_enter_kill`.
* **Captura de Eventos:** Monitorea de forma activa los procesos que emiten señales, identificando al emisor, al receptor y el tipo de señal enviada (por ejemplo, sondeos con señal 0 o terminaciones con `SIGTERM`/señal 15).

### 3.3. Daemon de Control
* Escanea y evalúa de forma continua los eventos capturados por la sonda.
* Aplica la **lógica de decisión** para identificar contenedores o procesos objetivo (como imágenes marcadas o clientes externos).
* Ejecuta acciones de mitigación de manera automática (como finalizar procesos o contenedores intrusos).
* Gestiona tareas periódicas mediante un sistema de cronjobs internos.

---

## 4. Guía de Instalación y Despliegue

### 4.1. Clonación y Configuración
1. Asegúrate de contar con permisos de superusuario (`sudo`) para la carga de módulos de kernel y eBPF.
2. Clona el repositorio del proyecto en tu entorno x86.

### 4.2. Despliegue de Servicios con Docker Compose
```bash
# Levantar los contenedores de Valkey y Grafana
docker-compose up -d