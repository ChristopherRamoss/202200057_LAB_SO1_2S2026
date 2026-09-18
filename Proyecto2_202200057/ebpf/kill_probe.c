// SPDX-License-Identifier: GPL-2.0
/*
 * kill_probe.c - Sonda eBPF que audita las llamadas a sys_kill.
 * Proyecto 2 - Sistemas Operativos 1 - USAC
 *
 * Se adjunta al tracepoint syscalls:sys_enter_kill y envia cada evento
 * (PID emisor, PID receptor, senal) al Daemon de Go via un ring buffer.
 *
 * NOTA: los campos "pid" y "sig" de sys_enter_kill se leen segun el layout
 * estandar reportado por:
 *   cat /sys/kernel/debug/tracing/events/syscalls/sys_enter_kill/format
 * Si tu kernel reporta offsets distintos, ajusta la struct de abajo.
 */

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

struct kill_event {
	__u32 src_pid;
	__u32 dst_pid;
	__u32 sig;
};

struct {
	__uint(type, BPF_MAP_TYPE_RINGBUF);
	__uint(max_entries, 256 * 1024);
} kill_events SEC(".maps");

/* Layout generico de un tracepoint sys_enter_<syscall> de 2 argumentos:
 * 8 bytes de cabecera comun + 2 argumentos de 8 bytes cada uno
 * (pid_t se promueve a 8 bytes de alineacion en el formato del tracepoint). */
struct trace_event_raw_sys_enter_kill {
	unsigned long long unused;
	long syscall_nr;
	long pid;
	long sig;
};

SEC("tracepoint/syscalls/sys_enter_kill")
int trace_kill(struct trace_event_raw_sys_enter_kill *ctx)
{
	struct kill_event *e;

	e = bpf_ringbuf_reserve(&kill_events, sizeof(*e), 0);
	if (!e)
		return 0;

	e->src_pid = (__u32)(bpf_get_current_pid_tgid() >> 32);
	e->dst_pid = (__u32)ctx->pid;
	e->sig     = (__u32)ctx->sig;

	bpf_ringbuf_submit(e, 0);
	return 0;
}

char LICENSE[] SEC("license") = "GPL";
