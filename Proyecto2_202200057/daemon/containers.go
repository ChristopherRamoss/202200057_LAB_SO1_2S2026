package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Categorias de contenedor. Se asignan por lo que el contenedor ES (su
// imagen), nunca por su consumo puntual de CPU/RAM.
const (
	CategoryHigh     = "high"     // roldyoran/go-client
	CategoryLow      = "low"      // alpine (sleep 240)
	CategoryIntruder = "intruder" // proyecto2-intruder
	CategoryGrafana  = "grafana"
	CategoryInfra    = "infra" // valkey
	CategoryOther    = "other" // cualquier otra cosa: el daemon no la toca
)

// Restriccion clave (guillotina): tras cada iteracion deben quedar vivos
// exactamente KeepLow contenedores de bajo consumo y KeepHigh de alto consumo.
const (
	KeepLow  = 3
	KeepHigh = 2
)

// Solo se gestionan los contenedores creados por el cronjob (todos llevan este
// label). Asi un contenedor ajeno con imagen alpine no altera los conteos.
const managedLabelFilter = "label=proyecto2=true"

type Container struct {
	ID        string
	Name      string
	Image     string
	IsGrafana bool
	Category  string
}

// classifyContainer decide la categoria segun la imagen (o el nombre para
// grafana). El intruso se evalua primero.
func classifyContainer(name, image string) string {
	lowerImg := strings.ToLower(image)
	lowerName := strings.ToLower(name)
	switch {
	case strings.Contains(lowerImg, "intruder"):
		return CategoryIntruder
	case strings.Contains(lowerImg, "grafana") || strings.Contains(lowerName, "grafana"):
		return CategoryGrafana
	case strings.Contains(lowerImg, "valkey"):
		return CategoryInfra
	case strings.Contains(lowerImg, "go-client"):
		return CategoryHigh
	case strings.Contains(lowerImg, "alpine"):
		return CategoryLow
	default:
		return CategoryOther
	}
}

func ListManagedContainers() ([]Container, error) {
	cmd := exec.Command("docker", "ps", "--filter", managedLabelFilter, "--format", "{{json .}}")
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
			ID:       id,
			Name:     name,
			Image:    image,
			Category: classifyContainer(name, image),
		}
		c.IsGrafana = c.Category == CategoryGrafana
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
	score     float64 // CPU% + RAM% del proceso principal del contenedor
	found     bool    // true si el PID aparecio en el snapshot del modulo de kernel
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

// consumptionScore devuelve CPU% + RAM% del PID segun el snapshot del modulo
// de kernel. Si el PID no aparece en el snapshot el consumo se toma como 0.
//
// El segundo valor indica si el PID aparecio en el snapshot. Distingue "consumo
// realmente cercano a 0" de "este PID no esta en los datos del modulo".
func consumptionScore(snap *Snapshot, pid int) (score float64, found bool) {
	if pid <= 0 {
		return 0, false
	}
	for _, p := range snap.Processes {
		if p.PID == pid {
			return p.MemPct + p.CPUPct, true
		}
	}
	return 0, false
}

// sortByConsumptionDesc ordena de mayor a menor consumo. El desempate por ID
// hace el orden determinista cuando varios contenedores tienen el mismo score.
func sortByConsumptionDesc(list []scoredContainer) {
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].score != list[j].score {
			return list[i].score > list[j].score
		}
		return list[i].container.ID < list[j].container.ID
	})
}

// DecideContainersToKill devuelve los contenedores que hay que eliminar:
//
//  1. Los intrusos, siempre y sin condiciones (van primero en la lista).
//  2. Los excedentes de cada grupo. Los grupos se forman por imagen (go-client
//     = alto, alpine = bajo), se ordenan de mayor a menor consumo y se eliminan
//     los que MAS consumen hasta que queden KeepLow bajos y KeepHigh altos.
//
// Si de un grupo hay menos contenedores que el minimo a conservar no se elimina
// ninguno de ese grupo (el daemon no puede crear contenedores).
func DecideContainersToKill(snap *Snapshot, containers []Container) []Container {
	pidMap := matchPIDs(containers)

	var intruders []Container
	var lows, highs []scoredContainer

	for _, c := range containers {
		switch c.Category {
		case CategoryIntruder:
			intruders = append(intruders, c)
		case CategoryHigh, CategoryLow:
			pid := pidMap[c.ID]
			score, found := consumptionScore(snap, pid)
			sc := scoredContainer{container: c, pid: pid, score: score, found: found}
			if c.Category == CategoryHigh {
				highs = append(highs, sc)
			} else {
				lows = append(lows, sc)
			}
		}
		// grafana, infra y other no se tocan
	}

	sortByConsumptionDesc(lows)
	sortByConsumptionDesc(highs)

	// Detalle real de cada grupo, ya ordenado de mayor a menor consumo
	// (CPU% + RAM%). Los primeros de cada lista son los candidatos a eliminar.
	log.Printf("Snapshot del modulo: %d procesos", len(snap.Processes))
	logGroup := func(label string, list []scoredContainer) {
		for _, s := range list {
			log.Printf("  [%s] %-24s pid=%-7d en_snapshot=%-5t consumo(CPU+RAM)=%.4f",
				label, s.container.Name, s.pid, s.found, s.score)
		}
	}
	logGroup("alto", highs)
	logGroup("bajo", lows)

	toKill := make([]Container, 0, len(intruders)+len(lows)+len(highs))
	toKill = append(toKill, intruders...)

	remainingLow, remainingHigh := len(lows), len(highs)

	for _, s := range lows {
		if remainingLow <= KeepLow {
			break
		}
		toKill = append(toKill, s.container)
		remainingLow--
	}

	for _, s := range highs {
		if remainingHigh <= KeepHigh {
			break
		}
		toKill = append(toKill, s.container)
		remainingHigh--
	}

	// Todo sale de los datos de ESTE tick (docker ps + snapshot). Lo unico fijo
	// son KeepHigh y KeepLow, que son la regla del enunciado.
	statusf("Antes: %d alto, %d bajo, %d intruso(s) | Se eliminan: %d alto, %d bajo, %d intruso(s) | Quedan: %d alto, %d bajo (objetivo %d alto, %d bajo)",
		len(highs), len(lows), len(intruders),
		len(highs)-remainingHigh, len(lows)-remainingLow, len(intruders),
		remainingHigh, remainingLow, KeepHigh, KeepLow)

	return toKill
}

// CountByCategory cuenta cuantos contenedores vivos hay de cada tipo.
func CountByCategory(containers []Container) (high, low, intruder int) {
	for _, c := range containers {
		switch c.Category {
		case CategoryHigh:
			high++
		case CategoryLow:
			low++
		case CategoryIntruder:
			intruder++
		}
	}
	return high, low, intruder
}
