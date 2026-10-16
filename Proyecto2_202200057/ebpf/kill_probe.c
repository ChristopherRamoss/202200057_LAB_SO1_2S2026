// SPDX-License-Identifier: GPL-2.0
/*
 * kill_probe.c - Sonda eBPF que audita las llamadas a sys_kill Y
 * pidfd_send_signal (que es lo que realmente usa Docker moderno).
 * Proyecto 2 - Sistemas Operativos 1 - USAC
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

/*
 * Docker/containerd modernos usan pidfd_send_signal() en vez de kill(),
 * para evitar condiciones de carrera con PIDs reciclados. Esta syscall NO
 * recibe el PID objetivo directamente, sino un "pidfd" (file descriptor que
 * apunta al proceso). Aqui solo capturamos el pidfd crudo; resolverlo al PID
 * real se hace en espacio de usuario (Go), leyendo
 * /proc/<pid_emisor>/fdinfo/<pidfd>, donde el kernel expone la linea
 * "Pid:\t<pid_real>".
 */
struct pidfd_event {
	__u32 src_pid;
	__u32 pidfd;
	__u32 sig;
};

struct {
	__uint(type, BPF_MAP_TYPE_RINGBUF);
	__uint(max_entries, 256 * 1024);
} pidfd_events SEC(".maps");

struct trace_event_raw_sys_enter_pidfd_send_signal {
	unsigned long long unused;
	long syscall_nr;
	long pidfd;
	long sig;
	long info;
	long flags;
};

SEC("tracepoint/syscalls/sys_enter_pidfd_send_signal")
int trace_pidfd_send_signal(struct trace_event_raw_sys_enter_pidfd_send_signal *ctx)
{
	struct pidfd_event *e;

	e = bpf_ringbuf_reserve(&pidfd_events, sizeof(*e), 0);
	if (!e)
		return 0;

	e->src_pid = (__u32)(bpf_get_current_pid_tgid() >> 32);
	e->pidfd   = (__u32)ctx->pidfd;
	e->sig     = (__u32)ctx->sig;

	bpf_ringbuf_submit(e, 0);
	return 0;
}

char LICENSE[] SEC("license") = "GPL";
