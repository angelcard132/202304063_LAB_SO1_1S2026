MODULE_NAME = continfo_pr2_so1_202304063
obj-m += $(MODULE_NAME).o

KDIR ?= /lib/modules/$(shell uname -r)/build
PWD := $(shell pwd)

all:
	$(MAKE) -C $(KDIR) M=$(PWD) modules

clean:
	$(MAKE) -C $(KDIR) M=$(PWD) clean

load:
	sudo insmod $(MODULE_NAME).ko

unload:
	sudo rmmod $(MODULE_NAME)

reload: unload load

status:
	lsmod | grep $(MODULE_NAME)
