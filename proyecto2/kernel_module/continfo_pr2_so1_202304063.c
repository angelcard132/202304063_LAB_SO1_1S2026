#include <linux/module.h>
#include <linux/kernel.h>
#include <linux/init.h>
#include <linux/proc_fs.h>
#include <linux/seq_file.h>
#include <linux/sched/signal.h>
#include <linux/sched.h>
#include <linux/mm.h>
#include <linux/fs.h>
#include <linux/version.h>

MODULE_LICENSE("GPL");
MODULE_AUTHOR("202304063");
MODULE_DESCRIPTION("Modulo de Kernel de Observabilidad para Telemetria de Contenedores y Procesos - SO1");
MODULE_VERSION("1.0");

#define PROC_NAME "continfo_pr2_so1_202304063"

static int continfo_show(struct seq_file *m, void *v) {
    struct sysinfo si;
    struct task_struct *task;
    unsigned long total_ram_kb, free_ram_kb, used_ram_kb;
    bool first = true;

    si_meminfo(&si);
    total_ram_kb = (si.totalram * si.mem_unit) / 1024;
    free_ram_kb = (si.freeram * si.mem_unit) / 1024;
    used_ram_kb = total_ram_kb - free_ram_kb;

    seq_printf(m, "{\n");
    seq_printf(m, "  \"total_ram_mb\": %lu,\n", total_ram_kb / 1024);
    seq_printf(m, "  \"free_ram_mb\": %lu,\n", free_ram_kb / 1024);
    seq_printf(m, "  \"used_ram_mb\": %lu,\n", used_ram_kb / 1024);
    seq_printf(m, "  \"total_ram_kb\": %lu,\n", total_ram_kb);
    seq_printf(m, "  \"free_ram_kb\": %lu,\n", free_ram_kb);
    seq_printf(m, "  \"used_ram_kb\": %lu,\n", used_ram_kb);
    seq_printf(m, "  \"processes\": [\n");

    for_each_process(task) {
        unsigned long vsz_kb = 0;
        unsigned long rss_kb = 0;
        unsigned long ram_percent_int = 0;
        unsigned long ram_percent_dec = 0;
        u64 cpu_time = 0;
        unsigned long cpu_percent_int = 0;
        unsigned long cpu_percent_dec = 0;

        if (task->mm) {
            rss_kb = (get_mm_rss(task->mm) * PAGE_SIZE) / 1024;
            vsz_kb = (task->mm->total_vm * PAGE_SIZE) / 1024;
        }

        if (total_ram_kb > 0) {
            unsigned long total_ram_percent = (rss_kb * 10000) / total_ram_kb;
            ram_percent_int = total_ram_percent / 100;
            ram_percent_dec = total_ram_percent % 100;
        }

        cpu_time = task->utime + task->stime;
        if (jiffies > 0) {
            unsigned long total_cpu = (cpu_time * 10000) / (jiffies + 1);
            cpu_percent_int = total_cpu / 100;
            cpu_percent_dec = total_cpu % 100;
        }

        if (!first) {
            seq_printf(m, ",\n");
        }
        first = false;

        seq_printf(m, "    {\n");
        seq_printf(m, "      \"pid\": %d,\n", task->pid);
        seq_printf(m, "      \"name\": \"%s\",\n", task->comm);
        seq_printf(m, "      \"cmdline\": \"%s\",\n", task->comm);
        seq_printf(m, "      \"vsz_kb\": %lu,\n", vsz_kb);
        seq_printf(m, "      \"rss_kb\": %lu,\n", rss_kb);
        seq_printf(m, "      \"ram_percent\": %lu.%02lu,\n", ram_percent_int, ram_percent_dec);
        seq_printf(m, "      \"cpu_percent\": %lu.%02lu\n", cpu_percent_int, cpu_percent_dec);
        seq_printf(m, "    }");
    }

    seq_printf(m, "\n  ]\n}\n");
    return 0;
}

static int continfo_open(struct inode *inode, struct file *file) {
    return single_open(file, continfo_show, NULL);
}

#if LINUX_VERSION_CODE >= KERNEL_VERSION(5, 6, 0)
static const struct proc_ops proc_fops = {
    .proc_open    = continfo_open,
    .proc_read    = seq_read,
    .proc_lseek   = seq_lseek,
    .proc_release = single_release,
};
#else
static const struct file_operations proc_fops = {
    .owner   = THIS_MODULE,
    .open    = continfo_open,
    .read    = seq_read,
    .llseek  = seq_lseek,
    .release = single_release,
};
#endif

static int __init continfo_init(void) {
    proc_create(PROC_NAME, 0444, NULL, &proc_fops);
    pr_info("Modulo %s cargado exitosamente. Carnet: 202304063\n", PROC_NAME);
    return 0;
}

static void __exit continfo_exit(void) {
    remove_proc_entry(PROC_NAME, NULL);
    pr_info("Modulo %s descargado exitosamente.\n", PROC_NAME);
}

module_init(continfo_init);
module_exit(continfo_exit);
