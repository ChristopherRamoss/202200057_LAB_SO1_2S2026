package main

import (
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// ===== Configuracion =====
const (
	CARNET           = "202200057"
	ProcFilePath     = "/proc/continfo_pr2_so1_" + CARNET
	ValkeyAddr       = "localhost:6379"
	ComposeFilePath  = "/home/christopher/Descargas/Eliminar/202200057_LAB_SO1_2S2026/Proyecto2_202200057/docker-compose.yml"
	CronScriptPath   = "/home/christopher/Descargas/Eliminar/202200057_LAB_SO1_2S2026/Proyecto2_202200057/scripts/cronjob_generate_containers.sh"
	KernelLoadScript = "/home/christopher/Descargas/Eliminar/202200057_LAB_SO1_2S2026/Proyecto2_202200057/kernel/load_module.sh"
	EBPFObjectPath   = "/home/christopher/Descargas/Eliminar/202200057_LAB_SO1_2S2026/Proyecto2_202200057/ebpf/kill_probe.o"
	LoopInterval     = 30 * time.Second
)

// PendingKills es un mapa PID -> Container protegido con mutex, porque lo
// escribe el loop principal y lo lee/borra la gorutina de eventos eBPF al
// mismo tiempo. Sin este candado, el acceso concurrente al mapa es
// comportamiento indefinido en Go (lecturas/escrituras se pueden perder).
type PendingKills struct {
	mu sync.Mutex
	m  map[int]Container
}

func NewPendingKills() *PendingKills {
	return &PendingKills{m: make(map[int]Container)}
}

func (p *PendingKills) Add(pid int, c Container) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.m[pid] = c
}

func (p *PendingKills) TakeIfPresent(pid int) (Container, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	c, ok := p.m[pid]
	if ok {
		delete(p.m, pid)
	}
	return c, ok
}

func main() {
	log.Println("== Daemon Proyecto 2 SO1 iniciando ==")

	composeUp := exec.Command("docker", "compose", "-f", ComposeFilePath, "up", "-d")
	composeUp.Stdout = os.Stdout
	composeUp.Stderr = os.Stderr
	if err := composeUp.Run(); err != nil {
		log.Fatalf("no se pudo levantar docker compose: %v", err)
	}
	log.Println("Grafana y Valkey levantados via docker compose.")

	valkey := NewValkeyClient(ValkeyAddr)

	if err := InstallCronjob(CronScriptPath); err != nil {
		log.Fatalf("no se pudo instalar el cronjob: %v", err)
	}
	log.Println("Cronjob instalado (cada 2 minutos).")

	loadCmd := exec.Command("bash", KernelLoadScript)
	loadCmd.Env = append(os.Environ(), "CARNET="+CARNET)
	loadCmd.Stdout = os.Stdout
	loadCmd.Stderr = os.Stderr
	if err := loadCmd.Run(); err != nil {
		log.Fatalf("no se pudo cargar el modulo de kernel: %v", err)
	}
	log.Println("Modulo de kernel cargado.")

	probe, err := LoadEBPFProbe(EBPFObjectPath)
	if err != nil {
		log.Fatalf("no se pudo cargar la sonda eBPF: %v", err)
	}
	defer probe.Close()
	log.Println("Sonda eBPF adjuntada a sys_enter_kill.")

	pendingKills := NewPendingKills()

	go func() {
		for ev := range probe.Events() {
			log.Printf("[eBPF] kill detectado: emisor=%d receptor=%d senal=%d", ev.SrcPID, ev.DstPID, ev.Signal)
			if c, ok := pendingKills.TakeIfPresent(int(ev.DstPID)); ok {
				if err := valkey.LogContainerRemoved(c.ID, c.Name, "SIGTERM"); err != nil {
					log.Printf("error registrando eliminacion en Valkey: %v", err)
				}
				if err := valkey.IncrEBPFEvent(); err != nil {
					log.Printf("error incrementando contador eBPF: %v", err)
				}
				log.Printf(">>> Contenedor %s confirmado como eliminado (evento eBPF, PID %d).", c.Name, ev.DstPID)
			}
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(LoopInterval)
	defer ticker.Stop()

	log.Printf("Loop principal iniciado (cada %s). Ctrl+C para detener.", LoopInterval)

runLoop:
	for {
		select {
		case <-ticker.C:
			runIteration(valkey, pendingKills)
		case <-stop:
			log.Println("Senal de terminacion recibida, cerrando...")
			break runLoop
		}
	}

	if err := RemoveCronjob(); err != nil {
		log.Printf("advertencia: no se pudo eliminar el cronjob: %v", err)
	} else {
		log.Println("Cronjob eliminado.")
	}
	log.Println("Daemon detenido.")
}

func runIteration(valkey *ValkeyClient, pendingKills *PendingKills) {
	snap, err := ReadProcFile(ProcFilePath)
	if err != nil {
		log.Printf("error leyendo %s: %v", ProcFilePath, err)
		return
	}

	if err := valkey.LogSystemSnapshot(snap); err != nil {
		log.Printf("error guardando snapshot de RAM en Valkey: %v", err)
	}
	if err := valkey.LogProcessRanking(snap); err != nil {
		log.Printf("error guardando ranking de procesos en Valkey: %v", err)
	}

	containers, err := ListManagedContainers()
	if err != nil {
		log.Printf("error listando contenedores: %v", err)
		return
	}

	toKill := DecideContainersToKill(snap, containers)
	pidMap := matchPIDs(containers)

	for _, c := range toKill {
		log.Printf("Decision: terminando contenedor %s (%s)", c.Name, c.Image)

		if pid, ok := pidMap[c.ID]; ok {
			pendingKills.Add(pid, c)
			log.Printf("Esperando confirmacion eBPF para PID %d (contenedor %s)", pid, c.Name)
		} else {
			log.Printf("advertencia: no se pudo obtener el PID de %s via docker inspect, no se podra confirmar via eBPF", c.Name)
		}

		if err := KillContainer(c.ID); err != nil {
			log.Printf("error enviando SIGTERM a %s: %v", c.ID, err)
			continue
		}

		go func(id string) {
			time.Sleep(5 * time.Second)
			_ = RemoveContainer(id)
		}(c.ID)
	}
}
