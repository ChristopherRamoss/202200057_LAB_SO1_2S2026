package main

import (
	"encoding/binary"
	"fmt"
	"log"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
)

// KillEvent refleja struct kill_event definida en ebpf/kill_probe.c.
type KillEvent struct {
	SrcPID uint32
	DstPID uint32
	Signal uint32
}

// EBPFProbe mantiene cargada la sonda y expone un canal de eventos.
type EBPFProbe struct {
	coll   *ebpf.Collection
	tp     link.Link
	reader *ringbuf.Reader
	events chan KillEvent
}

// LoadEBPFProbe carga kill_probe.o, lo adjunta a syscalls:sys_enter_kill y
// arranca la lectura del ring buffer en background.
func LoadEBPFProbe(objPath string) (*EBPFProbe, error) {
	spec, err := ebpf.LoadCollectionSpec(objPath)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer %s: %w", objPath, err)
	}

	coll, err := ebpf.NewCollection(spec)
	if err != nil {
		return nil, fmt.Errorf("no se pudo crear la coleccion eBPF (revisa dmesg/verifier): %w", err)
	}

	prog, ok := coll.Programs["trace_kill"]
	if !ok {
		coll.Close()
		return nil, fmt.Errorf("programa 'trace_kill' no encontrado en %s", objPath)
	}

	tp, err := link.Tracepoint("syscalls", "sys_enter_kill", prog, nil)
	if err != nil {
		coll.Close()
		return nil, fmt.Errorf("no se pudo adjuntar al tracepoint sys_enter_kill: %w", err)
	}

	m, ok := coll.Maps["kill_events"]
	if !ok {
		tp.Close()
		coll.Close()
		return nil, fmt.Errorf("map 'kill_events' no encontrado")
	}

	reader, err := ringbuf.NewReader(m)
	if err != nil {
		tp.Close()
		coll.Close()
		return nil, fmt.Errorf("no se pudo abrir el ring buffer: %w", err)
	}

	probe := &EBPFProbe{
		coll:   coll,
		tp:     tp,
		reader: reader,
		events: make(chan KillEvent, 64),
	}

	go probe.readLoop()
	return probe, nil
}

func (p *EBPFProbe) readLoop() {
	for {
		record, err := p.reader.Read()
		if err != nil {
			log.Printf("[eBPF] lector de ring buffer cerrado: %v", err)
			close(p.events)
			return
		}
		if len(record.RawSample) < 12 {
			continue
		}
		ev := KillEvent{
			SrcPID: binary.LittleEndian.Uint32(record.RawSample[0:4]),
			DstPID: binary.LittleEndian.Uint32(record.RawSample[4:8]),
			Signal: binary.LittleEndian.Uint32(record.RawSample[8:12]),
		}
		p.events <- ev
	}
}

// Events expone el canal de lectura de eventos de kill confirmados.
func (p *EBPFProbe) Events() <-chan KillEvent {
	return p.events
}

func (p *EBPFProbe) Close() {
	p.reader.Close()
	p.tp.Close()
	p.coll.Close()
}
