package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

const (
	CARNET           = "202200057"
	ProcFilePath     = "/proc/continfo_pr2_so1_" + CARNET
	ValkeyAddr       = "localhost:6379"
	ComposeFilePath  = "/home/christopher/Descargas/Eliminar/202200057_LAB_SO1_2S2026/Proyecto2_202200057/docker-compose.yml"
	CronScriptPath   = "/home/christopher/Descargas/Eliminar/202200057_LAB_SO1_2S2026/Proyecto2_202200057/scripts/cronjob_generate_containers.sh"
	KernelLoadScript = "/home/christopher/Descargas/Eliminar/202200057_LAB_SO1_2S2026/Proyecto2_202200057/kernel/load_module.sh"
	EBPFObjectPath   = "/home/christopher/Descargas/Eliminar/202200057_LAB_SO1_2S2026/Proyecto2_202200057/ebpf/kill_probe.o"
	LoopInterval     = 30 * time.Second
	CronLogPath      = "/tmp/proyecto2_cron.log"

	// Tiempos de la terminacion en dos fases.
	sigtermGrace = 3 * time.Second // espera tras SIGTERM antes de escalar a SIGKILL
	sigkillGrace = 3 * time.Second // espera tras SIGKILL para verificar la muerte
	ebpfGrace    = 2 * time.Second // espera maxima a que llegue el evento eBPF tras verificar la muerte
)

// pendingKill representa un contenedor al que se le esta pidiendo terminar.
// SignalSeen es la ultima senal (distinta de 0) que la sonda eBPF vio dirigida
// al PID del contenedor. Que eBPF vea la senal NO significa que el contenedor
// haya muerto: es solo la mitad de la confirmacion.
type pendingKill struct {
	Container  Container
	SignalSeen uint32
}

type PendingKills struct {
	mu sync.Mutex
	m  map[int]*pendingKill
}

func NewPendingKills() *PendingKills {
	return &PendingKills{m: make(map[int]*pendingKill)}
}

func (p *PendingKills) Add(pid int, c Container) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.m[pid] = &pendingKill{Container: c}
}

// RecordSignal anota que eBPF vio una senal hacia pid. Devuelve true si ese
// PID estaba pendiente.
func (p *PendingKills) RecordSignal(pid int, sig uint32) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	pk, ok := p.m[pid]
	if !ok {
		return false
	}
	pk.SignalSeen = sig
	return true
}

// SignalSeen devuelve la ultima senal observada por eBPF para pid (0 si
// ninguna).
func (p *PendingKills) SignalSeen(pid int) uint32 {
	p.mu.Lock()
	defer p.mu.Unlock()
	if pk, ok := p.m[pid]; ok {
		return pk.SignalSeen
	}
	return 0
}

func (p *PendingKills) Remove(pid int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.m, pid)
}

// statusf escribe en el log del daemon y ademas agrega la linea al log del
// cron, de modo que un solo `tail -f /tmp/proyecto2_cron.log` muestre todo el
// ciclo: creacion (cron), decision de la guillotina y estado final (daemon).
func statusf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Print(msg)

	f, err := os.OpenFile(CronLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "[daemon] %s: %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)
}

func signalName(sig uint32) string {
	switch sig {
	case 9:
		return "SIGKILL"
	case 15:
		return "SIGTERM"
	default:
		return "SIG(" + strconv.Itoa(int(sig)) + ")"
	}
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

	// Lector de eventos eBPF. Solo REGISTRA la senal observada; la
	// confirmacion final (contenedor realmente muerto) la hace
	// terminateContainer.
	go func() {
		for ev := range probe.Events() {
			log.Printf("[eBPF] kill detectado: emisor=%d receptor=%d senal=%d", ev.SrcPID, ev.DstPID, ev.Signal)
			if ev.Signal == 0 {
				// kill(pid, 0) es solo una prueba de existencia, no termina nada.
				continue
			}
			if pendingKills.RecordSignal(int(ev.DstPID), ev.Signal) {
				log.Printf("[eBPF] senal %s hacia PID %d observada (pendiente de verificar que el contenedor muera)",
					signalName(ev.Signal), ev.DstPID)
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

	var wg sync.WaitGroup

	for _, c := range toKill {
		log.Printf("Decision: terminando contenedor %s (%s)", c.Name, c.Image)

		pid, ok := pidMap[c.ID]
		if !ok {
			log.Printf("advertencia: no se pudo obtener el PID de %s via docker inspect, se omite", c.Name)
			continue
		}

		// Se registra ANTES de enviar la senal para no perder el evento eBPF.
		pendingKills.Add(pid, c)
		log.Printf("Esperando confirmacion eBPF para PID %d (contenedor %s)", pid, c.Name)

		wg.Add(1)
		go func(c Container, pid int) {
			defer wg.Done()
			terminateContainer(c, pid, valkey, pendingKills)
		}(c, pid)
	}

	// Cuando terminan TODAS las eliminaciones se vuelve a consultar Docker y se
	// informa el estado real que quedo (sin bloquear el loop principal).
	if len(toKill) > 0 {
		go func() {
			wg.Wait()
			if alive, err := ListManagedContainers(); err == nil {
				h, l, i := CountByCategory(alive)
				statusf("Estado final (verificado con docker ps): %d alto, %d bajo, %d intruso(s) vivos", h, l, i)
			}
		}()
	}
}

// terminateContainer termina un contenedor en dos fases (SIGTERM y, si sigue
// vivo, SIGKILL) pidiendoselo a dockerd, y solo lo da por eliminado cuando se
// cumplen DOS condiciones:
//  1. dockerd confirma que el contenedor ya no esta corriendo, y
//  2. la sonda eBPF vio la senal dirigida a su PID.
func terminateContainer(c Container, pid int, valkey *ValkeyClient, pendingKills *PendingKills) {
	if err := KillContainer(c.ID, "SIGTERM"); err != nil {
		log.Printf("error enviando SIGTERM a %s (PID %d): %v", c.Name, pid, err)
		pendingKills.Remove(pid) // no dejar pendientes que luego confirmen en falso
		return
	}

	exited := WaitForContainerExit(c.ID, sigtermGrace)
	if !exited {
		// Si el PID 1 del contenedor no tiene manejador de SIGTERM, el kernel
		// ignora la senal enviada desde fuera de su namespace: hay que escalar.
		log.Printf("%s (PID %d) sigue corriendo tras SIGTERM, escalando a SIGKILL", c.Name, pid)
		if err := KillContainer(c.ID, "SIGKILL"); err != nil {
			log.Printf("error enviando SIGKILL a %s (PID %d): %v", c.Name, pid, err)
		}
		exited = WaitForContainerExit(c.ID, sigkillGrace)
	}

	if !exited {
		log.Printf("ADVERTENCIA: %s (PID %d) sigue corriendo tras SIGKILL; NO se confirma como eliminado", c.Name, pid)
		pendingKills.Remove(pid)
		return
	}

	// El contenedor murio de verdad. Falta la evidencia eBPF: puede llegar con
	// unos milisegundos de retraso por el ring buffer, asi que se espera un poco.
	var sig uint32
	deadline := time.Now().Add(ebpfGrace)
	for {
		if sig = pendingKills.SignalSeen(pid); sig != 0 {
			break
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	pendingKills.Remove(pid)

	if sig == 0 {
		log.Printf("ADVERTENCIA: %s (PID %d) murio pero eBPF no observo ninguna senal hacia ese PID; no se registra como confirmado",
			c.Name, pid)
	} else {
		sigName := signalName(sig)
		if err := valkey.LogContainerRemoved(c.ID, c.Name, sigName); err != nil {
			log.Printf("error registrando eliminacion en Valkey: %v", err)
		}
		if err := valkey.IncrEBPFEvent(); err != nil {
			log.Printf("error incrementando contador eBPF: %v", err)
		}
		log.Printf(">>> Contenedor %s confirmado como eliminado (proceso muerto + evento eBPF, PID %d, senal %s).",
			c.Name, pid, sigName)
	}

	_ = RemoveContainer(c.ID)
}
