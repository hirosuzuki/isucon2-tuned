help:
	@echo make '[ setup | install | run | stop ]'

setup:
	sudo apt update
	sudo apt install mysql-server nodejs npm

install:
	(cd http_load_isucon2; make)
	npm install

run:
	nohup node manager.js &
	nohup node agent.js &

stop:
	killall node
