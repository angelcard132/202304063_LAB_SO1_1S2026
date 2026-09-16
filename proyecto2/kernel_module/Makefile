obj-m += continfo_pr2_so1_202304063.o

KDIR ?= $(firstword $(wildcard /lib/modules/*/build))

all:
	make -C $(KDIR) M=$(PWD) modules

clean:
	make -C $(KDIR) M=$(PWD) clean
