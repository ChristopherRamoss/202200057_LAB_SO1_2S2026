// SPDX-License-Identifier: GPL-2.0
/*
 * continfo.c - Modulo de kernel para telemetria de contenedores
 * Proyecto 2 - Sistemas Operativos 1 - USAC
 *
 * NOTA DE DISENO: get_cmdline() no esta exportada en algunos kernels (ej.
 * 7.0.x), asi que leemos el cmdline con access_process_vm(), exportada de
 * forma estable.
 */

#include <linux/module.h>
#include <linux/kernel.h>
#include <linux/init.h>
#include <linux/proc_fs.h>
#include <linux/seq_file.h>
#include <linux/sched.h>
#include <linux/sched/signal.h>
#include <linux/sched/mm.h>
#include <linux/mm.h>
#include <linux/slab.h>
#include <linux/jiffies.h>
#include <linux/string.h>

#ifndef CARNET_STR
#define CARNET_STR "000000000"
#endif

#define MAX_PROCESSES 8192
#define CMDLINE_BUF_SIZE 256

static struct proc_dir_entry *continfo_entry;
static char proc_name[64];

static void read_cmdline(struct task_struct *task, char *buf, size_t buflen)
{
	struct mm_struct *mm;
	unsigned long arg_start = 0, arg_end = 0, len;
	int copied;
	size_t i;

	memset(buf, 0, buflen);

	mm = get_task_mm(task);
	if (!mm) {
		scnprintf(buf, buflen, "[%s]", task->comm);
		return;
	}

	mmap_read_lock(mm);
	arg_start = mm->arg_start;
	arg_end = mm->arg_end;
	mmap_read_unlock(mm);

	if (arg_end <= arg_start) {
		scnprintf(buf, buflen, "[%s]", task->comm);
		mmput(mm);
		return;
	}

	len = arg_end - arg_start;
	if (len > buflen - 1)
		len = buflen - 1;

	copied = access_process_vm(task, arg_start, buf, (int)len, 0);
	mmput(mm);

	if (copied <= 0) {
		scnprintf(buf, buflen, "[%s]", task->comm);
		return;
	}

	for (i = 0; i < (size_t)copied - 1; i++) {
		if (buf[i] == '\0')
			buf[i] = ' ';
	}
}

static int continfo_show(struct seq_file *m, void *v)
{
	struct sysinfo si;
	struct task_struct *task;
	struct task_struct **task_array;
	int count = 0, i;
	unsigned long total_ram_kb, free_ram_kb, used_ram_kb;

	si_meminfo(&si);
	total_ram_kb = si.totalram << (PAGE_SHIFT - 10);
	free_ram_kb  = si.freeram  << (PAGE_SHIFT - 10);
	used_ram_kb  = total_ram_kb - free_ram_kb;

	seq_printf(m, "RAM_TOTAL_KB: %lu\n", total_ram_kb);
	seq_printf(m, "RAM_FREE_KB: %lu\n", free_ram_kb);
	seq_printf(m, "RAM_USED_KB: %lu\n", used_ram_kb);
	seq_printf(m, "---\n");
	seq_printf(m, "PID\tNAME\tCMDLINE\tVSZ_KB\tRSS_KB\tMEM_PCT\tCPU_PCT\n");

	task_array = kmalloc_array(MAX_PROCESSES, sizeof(struct task_struct *), GFP_KERNEL);
	if (!task_array) {
		seq_printf(m, "ERROR: sin memoria para listar procesos\n");
		return 0;
	}

	rcu_read_lock();
	for_each_process(task) {
		if (count >= MAX_PROCESSES)
			break;
		get_task_struct(task);
		task_array[count++] = task;
	}
	rcu_read_unlock();

	for (i = 0; i < count; i++) {
		struct mm_struct *mm;
		unsigned long vsz_kb = 0, rss_kb = 0;
		unsigned long mem_pct_int = 0, mem_pct_dec = 0;
		u64 total_time, elapsed, cpu_pct_int = 0, cpu_pct_dec = 0;
		char cmdline[CMDLINE_BUF_SIZE];

		task = task_array[i];
		read_cmdline(task, cmdline, sizeof(cmdline));

		mm = get_task_mm(task);
		if (mm) {
			vsz_kb = mm->total_vm << (PAGE_SHIFT - 10);
			rss_kb = get_mm_rss(mm) << (PAGE_SHIFT - 10);
			mmput(mm);
		}

		if (total_ram_kb > 0) {
			mem_pct_int = (rss_kb * 100) / total_ram_kb;
			mem_pct_dec = ((rss_kb * 1000) / total_ram_kb) % 10;
		}

		total_time = task->utime + task->stime;
		elapsed = (u64)jiffies - (u64)task->start_time;
		if (elapsed == 0)
			elapsed = 1;
		cpu_pct_int = (total_time * 100) / elapsed;
		cpu_pct_dec = ((total_time * 1000) / elapsed) % 10;

		seq_printf(m, "%d\t%s\t%s\t%lu\t%lu\t%lu.%lu\t%llu.%llu\n",
			   task->pid, task->comm, cmdline,
			   vsz_kb, rss_kb,
			   mem_pct_int, mem_pct_dec,
			   cpu_pct_int, cpu_pct_dec);

		put_task_struct(task);
	}

	kfree(task_array);
	return 0;
}

static int continfo_open(struct inode *inode, struct file *file)
{
	return single_open(file, continfo_show, NULL);
}

static const struct proc_ops continfo_fops = {
	.proc_open    = continfo_open,
	.proc_read    = seq_read,
	.proc_lseek   = seq_lseek,
	.proc_release = single_release,
};

static int __init continfo_init(void)
{
	scnprintf(proc_name, sizeof(proc_name), "continfo_pr2_so1_%s", CARNET_STR);
	continfo_entry = proc_create(proc_name, 0444, NULL, &continfo_fops);
	if (!continfo_entry) {
		pr_err("continfo: no se pudo crear /proc/%s\n", proc_name);
		return -ENOMEM;
	}
	pr_info("continfo: modulo cargado, archivo /proc/%s creado\n", proc_name);
	return 0;
}

static void __exit continfo_exit(void)
{
	if (continfo_entry)
		proc_remove(continfo_entry);
	pr_info("continfo: modulo descargado, /proc/%s eliminado\n", proc_name);
}

module_init(continfo_init);
module_exit(continfo_exit);

MODULE_LICENSE("GPL");
MODULE_AUTHOR("Christopher - USAC");
MODULE_DESCRIPTION("Sonda de kernel para telemetria de contenedores - Proyecto 2 SO1");
