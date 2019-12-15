help:
	@echo make '[ setup | run | install | run ]'

setup:
	sudo apt update
	sudo apt install gcc libssl-dev mysql-server libmysqlclient-dev python virtualenv python-dev python3-venv python3-dev libnginx-mod-http-lua

install:
	python3 -m venv venv3
	venv3/bin/pip install -r requirements.txt

run:
	venv3/bin/gunicorn -c gunicorn_config.py -w 30 app:app --access-logfile - --access-logformat '%(h)s %(l)s %(u)s %(t)s "%(r)s" %(s)s %(b)s "%(f)s" "%(a)s" %(D)s'

kataribe:
	cat /var/log/nginx/main.log | ./kataribe


rotate-nginx-log:
	sudo mv /var/log/nginx/main.log /var/log/nginx/main.log-`date +%Y%m%d-%H%M%S` ; sudo systemctl restart nginx
