package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// SystemInfo son las metricas globales de RAM que expone el modulo de kernel.
type SystemInfo struct {
	TotalRAMKB uint64 `json:"total_ram_kb"`
	FreeRAMKB  uint64 `json:"free_ram_kb"`
	UsedRAMKB  uint64 `json:"used_ram_kb"`
}

// ProcInfo es una fila de proceso tal como la reporta el modulo de kernel.
type ProcInfo struct {
	PID     int
	Name    string
	Cmdline string
	VSZKB   uint64
	RSSKB   uint64
	MemPct  float64
	CPUPct  float64
}

// Snapshot es la lectura completa de /proc/continfo_pr2_so1_<CARNET> en un
// momento dado.
type Snapshot struct {
	System    SystemInfo
	Processes []ProcInfo
}

// ReadProcFile lee y deserializa el archivo expuesto por el modulo de kernel.
func ReadProcFile(path string) (*Snapshot, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir %s: %w", path, err)
	}
	defer f.Close()

	snap := &Snapshot{}
	scanner := bufio.NewScanner(f)
	pastHeader := false

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r\n")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		switch {
		case strings.HasPrefix(trimmed, "RAM_TOTAL_KB:"):
			snap.System.TotalRAMKB = parseUint(trimmed, "RAM_TOTAL_KB:")
		case strings.HasPrefix(trimmed, "RAM_FREE_KB:"):
			snap.System.FreeRAMKB = parseUint(trimmed, "RAM_FREE_KB:")
		case strings.HasPrefix(trimmed, "RAM_USED_KB:"):
			snap.System.UsedRAMKB = parseUint(trimmed, "RAM_USED_KB:")
		case trimmed == "---":
			continue
		case strings.HasPrefix(trimmed, "PID\t"), strings.HasPrefix(trimmed, "PID "):
			pastHeader = true
			continue
		default:
			if !pastHeader {
				continue
			}
			fields := strings.Split(line, "\t")
			if len(fields) < 7 {
				continue
			}
			pid, _ := strconv.Atoi(strings.TrimSpace(fields[0]))
			vsz, _ := strconv.ParseUint(strings.TrimSpace(fields[3]), 10, 64)
			rss, _ := strconv.ParseUint(strings.TrimSpace(fields[4]), 10, 64)
			memPct, _ := strconv.ParseFloat(strings.TrimSpace(fields[5]), 64)
			cpuPct, _ := strconv.ParseFloat(strings.TrimSpace(fields[6]), 64)

			snap.Processes = append(snap.Processes, ProcInfo{
				PID:     pid,
				Name:    strings.TrimSpace(fields[1]),
				Cmdline: strings.TrimSpace(fields[2]),
				VSZKB:   vsz,
				RSSKB:   rss,
				MemPct:  memPct,
				CPUPct:  cpuPct,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error leyendo %s: %w", path, err)
	}
	return snap, nil
}

func parseUint(line, prefix string) uint64 {
	v := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	n, _ := strconv.ParseUint(v, 10, 64)
	return n
}
