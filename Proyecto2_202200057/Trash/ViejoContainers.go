package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Container struct {
	ID        string
	Name      string
	Image     string
	IsGrafana bool
	Category  string
}

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

// KillContainer le pide a dockerd que envie la senal al contenedor
// (`docker kill --signal=...`). La senal la envia dockerd/containerd, que
// estan autorizados por el perfil AppArmor docker-default, en lugar de
// enviarla el daemon directamente desde el host (donde AppArmor la bloquea
// con EACCES si el proceso corre confinado, p. ej. bajo el perfil "vscode").
//
// sig es el nombre de la senal: "SIGTERM", "SIGKILL", etc.
func KillContainer(containerID string, sig string) error {
	cmd := exec.Command("docker", "kill", "--signal="+sig, containerID)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker kill --signal=%s fallo: %w (%s)", sig, err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// IsContainerRunning consulta el estado REAL del contenedor a traves de
// dockerd. Si el contenedor ya no existe (p. ej. se creo con --rm) se
// considera que no esta corriendo.
//
// Esta funcion reemplaza a IsProcessAlive(pid) basada en kill(pid, 0): esa
// prueba tambien pasa por AppArmor y devuelve EACCES, con lo cual un proceso
// vivo se interpretaba como muerto.
func IsContainerRunning(containerID string) (bool, error) {
	cmd := exec.Command("docker", "inspect", "--format", "{{.State.Running}}", containerID)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if strings.Contains(strings.ToLower(stderr.String()), "no such") {
			return false, nil
		}
		return false, fmt.Errorf("docker inspect fallo para %s: %w (%s)", containerID, err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(out.String()) == "true", nil
}

// WaitForContainerExit espera hasta timeout a que el contenedor deje de
// estar corriendo. Devuelve true solo si se verifico que ya no corre; ante un
// error de consulta sigue reintentando hasta agotar el tiempo.
func WaitForContainerExit(containerID string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		running, err := IsContainerRunning(containerID)
		if err == nil && !running {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func RemoveContainer(id string) error {
	return exec.Command("docker", "rm", "-f", id).Run()
}

type scoredContainer struct {
	container Container
	pid       int
	score     float64
	isHigh    bool
}

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

const highConsumptionThreshold = 20.0

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
