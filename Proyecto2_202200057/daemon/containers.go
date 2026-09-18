package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// Container representa un contenedor en ejecucion, tal como lo ve `docker ps`.
type Container struct {
	ID        string
	Name      string
	Image     string
	IsGrafana bool
	Category  string // "grafana", "infra", "intruder", "managed"
}

// ListManagedContainers lista los contenedores activos con `docker ps`.
func ListManagedContainers() ([]Container, error) {
	cmd := exec.Command("docker", "ps", "--format", "{{json .}}")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("docker ps fallo: %w", err)
	}

	var containers []Container
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		id, _ := raw["ID"].(string)
		name, _ := raw["Names"].(string)
		image, _ := raw["Image"].(string)

		c := Container{
			ID:    id,
			Name:  name,
			Image: image,
		}
		lowerImg := strings.ToLower(image)
		lowerName := strings.ToLower(name)
		switch {
		case strings.Contains(lowerImg, "grafana") || strings.Contains(lowerName, "grafana"):
			c.Category = "grafana"
			c.IsGrafana = true
		case strings.Contains(lowerImg, "valkey"):
			c.Category = "infra"
		case strings.Contains(lowerImg, "intruder"):
			c.Category = "intruder"
		default:
			c.Category = "managed"
		}
		containers = append(containers, c)
	}
	return containers, nil
}

// GetContainerPID pregunta a Docker directamente cual es el PID real (en el
// namespace del host) del proceso principal del contenedor. Esto es mucho
// mas confiable que intentar adivinarlo buscando el ID del contenedor dentro
// del cmdline del proceso (Docker no lo incluye ahi).
func GetContainerPID(containerID string) (int, error) {
	cmd := exec.Command("docker", "inspect", "--format", "{{.State.Pid}}", containerID)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("docker inspect fallo para %s: %w", containerID, err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(out.String()))
	if err != nil {
		return 0, fmt.Errorf("PID invalido para %s: %w", containerID, err)
	}
	return pid, nil
}

// KillContainer envia SIGTERM al contenedor (esto es lo que la sonda eBPF
// debe detectar via sys_kill).
func KillContainer(id string) error {
	return exec.Command("docker", "kill", "--signal", "SIGTERM", id).Run()
}

// RemoveContainer limpia el contenedor detenido del sistema.
func RemoveContainer(id string) error {
	return exec.Command("docker", "rm", "-f", id).Run()
}

type scoredContainer struct {
	container Container
	pid       int
	score     float64 // combinacion de %MEM + %CPU segun el snapshot del kernel
	isHigh    bool
}

// matchPIDs pregunta a Docker el PID real de cada contenedor via `docker
// inspect`. Devuelve un mapa containerID -> PID del host.
func matchPIDs(containers []Container) map[string]int {
	result := make(map[string]int)
	for _, c := range containers {
		pid, err := GetContainerPID(c.ID)
		if err == nil && pid > 0 {
			result[c.ID] = pid
		}
	}
	return result
}

// highConsumptionThreshold: por encima de esto (score = %MEM + %CPU) un
// contenedor se considera "alto consumo" para efectos de la restriccion
// minima de 2 contenedores de alto consumo / 3 de bajo consumo.
const highConsumptionThreshold = 20.0

// DecideContainersToKill aplica la politica de gestion:
//  1. Nunca tocar Grafana ni infraestructura (Valkey).
//  2. Ordenar los candidatos por consumo (RAM/CPU) de mayor a menor.
//  3. Ir marcando para eliminar a los de mayor consumo, sin bajar de
//     3 contenedores de bajo consumo ni de 2 de alto consumo.
func DecideContainersToKill(snap *Snapshot, containers []Container) []Container {
	pidMap := matchPIDs(containers)

	var scored []scoredContainer
	lowCount, highCount := 0, 0

	for _, c := range containers {
		if c.Category == "grafana" || c.Category == "infra" {
			continue
		}
		pid, hasPID := pidMap[c.ID]
		score := 0.0
		if hasPID {
			for _, p := range snap.Processes {
				if p.PID == pid {
					score = p.MemPct + p.CPUPct
					break
				}
			}
		}
		isHigh := score >= highConsumptionThreshold
		if isHigh {
			highCount++
		} else {
			lowCount++
		}
		scored = append(scored, scoredContainer{container: c, pid: pid, score: score, isHigh: isHigh})
	}

	// Mayor consumo primero: esos son los primeros candidatos a eliminar.
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	var toKill []Container
	remainingLow, remainingHigh := lowCount, highCount

	for _, s := range scored {
		if s.isHigh {
			if remainingHigh <= 2 {
				continue
			}
			remainingHigh--
		} else {
			if remainingLow <= 3 {
				continue
			}
			remainingLow--
		}
		toKill = append(toKill, s.container)
	}

	return toKill
}
