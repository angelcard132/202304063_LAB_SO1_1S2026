#include <linux/module.h>
#include <linux/export-internal.h>
#include <linux/compiler.h>

MODULE_INFO(name, KBUILD_MODNAME);

__visible struct module __this_module
__section(".gnu.linkonce.this_module") = {
	.name = KBUILD_MODNAME,
	.init = init_module,
#ifdef CONFIG_MODULE_UNLOAD
	.exit = cleanup_module,
#endif
	.arch = MODULE_ARCH_INIT,
};



static const struct modversion_info ____versions[]
__used __section("__versions") = {
	{ 0x581a7cd, "single_open" },
	{ 0x40c7247c, "si_meminfo" },
	{ 0x2529ebe1, "seq_printf" },
	{ 0xd2450a5e, "init_task" },
	{ 0x15ba50a6, "jiffies" },
	{ 0xf0fdf6cb, "__stack_chk_fail" },
	{ 0xc4e07007, "remove_proc_entry" },
	{ 0xf99f317e, "seq_read" },
	{ 0x993ee334, "seq_lseek" },
	{ 0x768bc98e, "single_release" },
	{ 0xbdfb6dbb, "__fentry__" },
	{ 0xdc679947, "proc_create" },
	{ 0x92997ed8, "_printk" },
	{ 0x5b8239ca, "__x86_return_thunk" },
	{ 0x5002fa32, "module_layout" },
};

MODULE_INFO(depends, "");


MODULE_INFO(srcversion, "B6F6BEAA7A39397056F1894");
