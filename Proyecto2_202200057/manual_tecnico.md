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
