# Makefile: unify common tasks

.PHONY: build install start stop restart status

build:
	./build.sh

install:
	./install.sh

start:
	./start.sh start

stop:
	./start.sh stop

restart:
	./start.sh restart

status:
	./start.sh status
