# Uso:
#   make CARNET=202112345          -> compila con tu carnet
#   make CARNET=202112345 load     -> carga el modulo (pide sudo)
#   make CARNET=202112345 show     -> muestra el contenido de /proc
#   make CARNET=202112345 unload   -> descarga el modulo
#
# Si no pasas CARNET, se usa un valor por defecto de prueba.

CARNET ?= 202200057

obj-m += continfo.o
ccflags-y += -DCARNET_STR=\"$(CARNET)\"

KDIR := /lib/modules/$(shell uname -r)/build
PWD  := $(shell pwd)

all:
	$(MAKE) -C $(KDIR) M=$(PWD) modules

clean:
	$(MAKE) -C $(KDIR) M=$(PWD) clean

load: all
	sudo insmod continfo.ko
	@echo "Cargado -> /proc/continfo_pr2_so1_$(CARNET)"

unload:
	sudo rmmod continfo || true

reload: unload load

show:
	cat /proc/continfo_pr2_so1_$(CARNET)

.PHONY: all clean load unload reload show
