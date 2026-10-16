package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
)

type KillEvent struct {
	SrcPID uint32
	DstPID uint32
	Signal uint32
}

type EBPFProbe struct {
	coll       *ebpf.Collection
	tpKill     link.Link
	tpPidfd    link.Link
	readerKill *ringbuf.Reader
	readerFd   *ringbuf.Reader
	events     chan KillEvent
}

func LoadEBPFProbe(objPath string) (*EBPFProbe, error) {
	spec, err := ebpf.LoadCollectionSpec(objPath)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer %s: %w", objPath, err)
	}

	coll, err := ebpf.NewCollection(spec)
	if err != nil {
		return nil, fmt.Errorf("no se pudo crear la coleccion eBPF (revisa dmesg/verifier): %w", err)
	}

	progKill, ok := coll.Programs["trace_kill"]
	if !ok {
		coll.Close()
		return nil, fmt.Errorf("programa 'trace_kill' no encontrado en %s", objPath)
	}
	tpKill, err := link.Tracepoint("syscalls", "sys_enter_kill", progKill, nil)
	if err != nil {
		coll.Close()
		return nil, fmt.Errorf("no se pudo adjuntar al tracepoint sys_enter_kill: %w", err)
	}

	progPidfd, ok := coll.Programs["trace_pidfd_send_signal"]
	if !ok {
		tpKill.Close()
		coll.Close()
		return nil, fmt.Errorf("programa 'trace_pidfd_send_signal' no encontrado en %s", objPath)
	}
	tpPidfd, err := link.Tracepoint("syscalls", "sys_enter_pidfd_send_signal", progPidfd, nil)
	if err != nil {
		tpKill.Close()
		coll.Close()
		return nil, fmt.Errorf("no se pudo adjuntar al tracepoint sys_enter_pidfd_send_signal: %w", err)
	}

	mKill, ok := coll.Maps["kill_events"]
	if !ok {
		tpPidfd.Close()
		tpKill.Close()
		coll.Close()
		return nil, fmt.Errorf("map 'kill_events' no encontrado")
	}
	readerKill, err := ringbuf.NewReader(mKill)
	if err != nil {
		tpPidfd.Close()
		tpKill.Close()
		coll.Close()
		return nil, fmt.Errorf("no se pudo abrir el ring buffer de kill_events: %w", err)
	}

	mPidfd, ok := coll.Maps["pidfd_events"]
	if !ok {
		readerKill.Close()
		tpPidfd.Close()
		tpKill.Close()
		coll.Close()
		return nil, fmt.Errorf("map 'pidfd_events' no encontrado")
	}
	readerFd, err := ringbuf.NewReader(mPidfd)
	if err != nil {
		readerKill.Close()
		tpPidfd.Close()
		tpKill.Close()
		coll.Close()
		return nil, fmt.Errorf("no se pudo abrir el ring buffer de pidfd_events: %w", err)
	}

	probe := &EBPFProbe{
		coll:       coll,
		tpKill:     tpKill,
		tpPidfd:    tpPidfd,
		readerKill: readerKill,
		readerFd:   readerFd,
		events:     make(chan KillEvent, 128),
	}

	go probe.readKillLoop()
	go probe.readPidfdLoop()
	return probe, nil
}

func (p *EBPFProbe) readKillLoop() {
	for {
		record, err := p.readerKill.Read()
		if err != nil {
			log.Printf("[eBPF] lector kill_events cerrado: %v", err)
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

func (p *EBPFProbe) readPidfdLoop() {
	for {
		record, err := p.readerFd.Read()
		if err != nil {
			log.Printf("[eBPF] lector pidfd_events cerrado: %v", err)
			return
		}
		if len(record.RawSample) < 12 {
			continue
		}
		srcPID := binary.LittleEndian.Uint32(record.RawSample[0:4])
		pidfd := binary.LittleEndian.Uint32(record.RawSample[4:8])
		sig := binary.LittleEndian.Uint32(record.RawSample[8:12])

		dstPID, ok := resolvePidFd(srcPID, pidfd)
		if !ok {
			continue
		}

		p.events <- KillEvent{SrcPID: srcPID, DstPID: dstPID, Signal: sig}
	}
}

func resolvePidFd(ownerPID, fd uint32) (uint32, bool) {
	path := fmt.Sprintf("/proc/%d/fdinfo/%d", ownerPID, fd)
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "Pid:") {
			fields := strings.Fields(line)
			if len(fields) == 2 {
				n, err := strconv.Atoi(fields[1])
				if err == nil && n > 0 {
					return uint32(n), true
				}
			}
		}
	}
	return 0, false
}

func (p *EBPFProbe) Events() <-chan KillEvent {
	return p.events
}

func (p *EBPFProbe) Close() {
	p.readerKill.Close()
	p.readerFd.Close()
	p.tpPidfd.Close()
	p.tpKill.Close()
	p.coll.Close()
}
