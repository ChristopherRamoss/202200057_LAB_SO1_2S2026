package main
import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
)
func main() {
	if len(os.Args) < 2 {
		log.Fatal("uso: sudo ./probetest /ruta/a/kill_probe.o")
	}
	objPath := os.Args[1]
	spec, err := ebpf.LoadCollectionSpec(objPath)
	if err != nil {
		log.Fatalf("no se pudo leer %s: %v", objPath, err)
	}
	coll, err := ebpf.NewCollection(spec)
	if err != nil {
		log.Fatalf("no se pudo cargar la coleccion (verifier?): %v", err)
	}
	defer coll.Close()
	prog, ok := coll.Programs["trace_kill"]
	if !ok {
		log.Fatal("programa 'trace_kill' no encontrado en el objeto")
	}
	tp, err := link.Tracepoint("syscalls", "sys_enter_kill", prog, nil)
	if err != nil {
		log.Fatalf("no se pudo adjuntar al tracepoint: %v", err)
	}
	defer tp.Close()
	m, ok := coll.Maps["kill_events"]
	if !ok {
		log.Fatal("map 'kill_events' no encontrado")
	}
	reader, err := ringbuf.NewReader(m)
	if err != nil {
		log.Fatalf("no se pudo abrir ringbuf: %v", err)
	}
	defer reader.Close()
	fmt.Println("Sonda cargada. Esperando eventos... (Ctrl+C para salir)")
	fmt.Println("En OTRA terminal: sleep 300 &   luego: kill -TERM <ese_PID>")
	for {
		record, err := reader.Read()
		if err != nil {
			log.Printf("lector cerrado: %v", err)
			return
		}
		if len(record.RawSample) < 12 {
			continue
		}
		src := binary.LittleEndian.Uint32(record.RawSample[0:4])
		dst := binary.LittleEndian.Uint32(record.RawSample[4:8])
		sig := binary.LittleEndian.Uint32(record.RawSample[8:12])
		fmt.Printf(">>> EVENTO: emisor=%d receptor=%d senal=%d\n", src, dst, sig)
	}
}
