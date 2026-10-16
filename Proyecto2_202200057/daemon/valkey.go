package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ValkeyClient encapsula todas las escrituras hacia Valkey que luego lee
// Grafana para armar el dashboard.
type ValkeyClient struct {
	rdb *redis.Client
	ctx context.Context
}

func NewValkeyClient(addr string) *ValkeyClient {
	return &ValkeyClient{
		rdb: redis.NewClient(&redis.Options{Addr: addr}),
		ctx: context.Background(),
	}
}

// LogSystemSnapshot guarda el estado de RAM y lo agrega a una serie de
// tiempo (sorted set) para el panel "Uso de RAM a lo largo del tiempo".
func (v *ValkeyClient) LogSystemSnapshot(snap *Snapshot) error {
	ts := time.Now().Unix()
	data, _ := json.Marshal(snap.System)
	key := fmt.Sprintf("system:ram:%d", ts)

	if err := v.rdb.Set(v.ctx, key, data, 24*time.Hour).Err(); err != nil {
		return err
	}
	if err := v.rdb.Set(v.ctx, "system:ram:latest", data, 0).Err(); err != nil {
		return err
	}
	ramUsedMB := float64(snap.System.UsedRAMKB) / 1024.0
	return v.rdb.ZAdd(v.ctx, "system:ram:timeline", redis.Z{
		Score:  float64(time.Now().UnixMilli()),
		Member: fmt.Sprintf("%.2f", ramUsedMB),
	}).Err()
}

// CAMBIE ACA

// LogContainerRemoved SOLO debe llamarse cuando eBPF confirmo la terminacion
// real via sys_kill (ver ebpfprobe.go / main.go).
func (v *ValkeyClient) LogContainerRemoved(containerID, name, signal string) error {
	ts := time.Now().Unix()
	payload := map[string]interface{}{
		"container_id": containerID,
		"name":         name,
		"timestamp":    ts,
		"signal":       signal,
	}
	data, _ := json.Marshal(payload)
	key := fmt.Sprintf("removed:%d:%s", ts, containerID)

	if err := v.rdb.Set(v.ctx, key, data, 0).Err(); err != nil {
		return err
	}
	if err := v.rdb.ZAdd(v.ctx, "removed:timeline", redis.Z{
		Score:  float64(ts),
		Member: key,
	}).Err(); err != nil {
		return err
	}
	return v.rdb.Incr(v.ctx, "removed:total_count").Err()
}

// LogProcessRanking alimenta los sorted sets que usan los pie charts de
// "Top 5 por RAM" y "Top 5 por CPU" (incluye activos e historicos).
func (v *ValkeyClient) LogProcessRanking(snap *Snapshot) error {
	pipe := v.rdb.Pipeline()
	for _, p := range snap.Processes {
		member := fmt.Sprintf("pid:%d name:%s", p.PID, p.Name)
		if p.RSSKB > 0 {
			pipe.ZAdd(v.ctx, "ranking:ram", redis.Z{Score: float64(p.RSSKB), Member: member})
		}
		if p.CPUPct > 0 {
			pipe.ZAdd(v.ctx, "ranking:cpu", redis.Z{Score: p.CPUPct, Member: member})
		}
	}
	_, err := pipe.Exec(v.ctx)
	return err
}

// IncrEBPFEvent incrementa el contador de eventos capturados por la sonda.
func (v *ValkeyClient) IncrEBPFEvent() error {
	return v.rdb.Incr(v.ctx, "ebpf:events_total").Err()
}
