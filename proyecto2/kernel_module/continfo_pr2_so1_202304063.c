#include <linux/init.h>
#include <linux/kernel.h>
#include <linux/mm.h>
#include <linux/module.h>
#include <linux/proc_fs.h>
#include <linux/sched/cputime.h>
#include <linux/sched/signal.h>
#include <linux/seq_file.h>
#include <linux/version.h>

MODULE_LICENSE("GPL");
MODULE_AUTHOR("202304063");
MODULE_DESCRIPTION(
    "Modulo de Kernel Telemetria de Contenedores SO1 Pr2 202304063");
MODULE_VERSION("1.0");

#define PROC_NAME "continfo_pr2_so1_202304063"

static int continfo_show(struct seq_file *m, void *v) {
  struct sysinfo i;
  struct task_struct *task;
  unsigned long total_ram_mb, free_ram_mb, used_ram_mb;
  unsigned long total_ram_kb, free_ram_kb, used_ram_kb;
  bool first = true;

  si_meminfo(&i);

  total_ram_kb = i.totalram * (i.mem_unit / 1024);
  free_ram_kb = i.freeram * (i.mem_unit / 1024);
  used_ram_kb = total_ram_kb - free_ram_kb;

  total_ram_mb = total_ram_kb / 1024;
  free_ram_mb = free_ram_kb / 1024;
  used_ram_mb = used_ram_kb / 1024;

  seq_printf(m, "{\n");
  seq_printf(m, "  \"total_ram_mb\": %lu,\n", total_ram_mb);
  seq_printf(m, "  \"free_ram_mb\": %lu,\n", free_ram_mb);
  seq_printf(m, "  \"used_ram_mb\": %lu,\n", used_ram_mb);
  seq_printf(m, "  \"total_ram_kb\": %lu,\n", total_ram_kb);
  seq_printf(m, "  \"free_ram_kb\": %lu,\n", free_ram_kb);
  seq_printf(m, "  \"used_ram_kb\": %lu,\n", used_ram_kb);
  seq_printf(m, "  \"processes\": [\n");

  rcu_read_lock();
  for_each_process(task) {
    unsigned long vsz = 0;
    unsigned long rss = 0;
    unsigned long ram_permille = 0;
    u64 utime = 0, stime = 0, total_time = 0;

    if (task->mm) {
      vsz = task->mm->total_vm << (PAGE_SHIFT - 10);   // En KB
      rss = get_mm_rss(task->mm) << (PAGE_SHIFT - 10); // En KB
      if (total_ram_kb > 0) {
        ram_permille = (rss * 1000) / total_ram_kb;
      }
    }

    task_cputime_adjusted(task, &utime, &stime);
    total_time = (utime + stime) / 1000000; // En ms aprox

    if (!first) {
      seq_printf(m, ",\n");
    }
    first = false;

    seq_printf(m, "    {\n");
    seq_printf(m, "      \"pid\": %d,\n", task->pid);
    seq_printf(m, "      \"name\": \"%s\",\n", task->comm);
    seq_printf(m, "      \"vsz_kb\": %lu,\n", vsz);
    seq_printf(m, "      \"rss_kb\": %lu,\n", rss);
    seq_printf(m, "      \"ram_percent\": %lu.%lu,\n", ram_permille / 10,
               ram_permille % 10);
    seq_printf(m, "      \"cpu_time_ms\": %llu\n", total_time);
    seq_printf(m, "    }");
  }
  rcu_read_unlock();

  seq_printf(m, "\n  ]\n}\n");
  return 0;
}

static int continfo_open(struct inode *inode, struct file *file) {
  return single_open(file, continfo_show, NULL);
}

#if LINUX_VERSION_CODE >= KERNEL_VERSION(5, 6, 0)
static const struct proc_ops continfo_fops = {
    .proc_open = continfo_open,
    .proc_read = seq_read,
    .proc_lseek = seq_lseek,
    .proc_release = single_release,
};
#else
static const struct file_operations continfo_fops = {
    .owner = THIS_MODULE,
    .open = continfo_open,
    .read = seq_read,
    .llseek = seq_lseek,
    .release = single_release,
};
#endif

static int __init continfo_init(void) {
  struct proc_dir_entry *entry;
  entry = proc_create(PROC_NAME, 0444, NULL, &continfo_fops);
  if (!entry) {
    pr_err("Error al crear /proc/%s\n", PROC_NAME);
    return -ENOMEM;
  }
  pr_info("Modulo %s cargado correctamente.\n", PROC_NAME);
  return 0;
}

static void __exit continfo_cleanup(void) {
  remove_proc_entry(PROC_NAME, NULL);
  pr_info("Modulo %s descargado.\n", PROC_NAME);
}

module_init(continfo_init);
module_exit(continfo_cleanup);
