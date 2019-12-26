help:
	@echo make '[ setup | run | install | run ]'

setup:
	sudo apt update
	sudo apt install gcc libssl-dev mysql-server libmysqlclient-dev python virtualenv python-dev python3-venv python3-dev libnginx-mod-http-lua

setup2:
	sudo mkdir -p /opt/isucon2
	sudo mkdir -p /opt/isucon2/config
	sudo mkdir -p /opt/isucon2/python
	sudo cp -r ../config/* /opt/isucon2/config
	sudo python3 -m venv /opt/isucon2/python/venv
	sudo /opt/isucon2/python/venv/bin/pip install -r requirements.txt
	sudo rsync -av --delete templates/ /opt/isucon2/python/templates/
	sudo mkdir -p /opt/isucon2/cache
	sudo mkdir -p /opt/isucon2/cache/artist
	sudo mkdir -p /opt/isucon2/cache/ticket
	sudo chown -Rh www-data:www-data /opt/isucon2/cache
	sudo mkdir -p /opt/isucon2/python/templates_compiled
	sudo chown www-data:www-data /opt/isucon2/python/templates_compiled
	sudo rsync -av --delete ../staticfiles/ /opt/isucon2/python/static/
	sudo rsync -av --delete ../staticfiles/ /opt/isucon2/staticfiles/
	find /opt/isucon2/staticfiles -type f | sudo xargs gzip
	sudo cp app.py /opt/isucon2/python/app.py
	sudo cp update_loop.sh /opt/isucon2/python/update_loop.sh
	sudo mkdir -p /var/log/gunicorn

setup3:
	sudo cp update_loop.service /etc/systemd/system
	sudo cp isucon2.service /etc/systemd/system
	sudo systemctl daemon-reload
	sudo systemctl enable update_loop.service
	sudo systemctl restart update_loop.service
	sudo systemctl enable isucon2.service
	sudo systemctl restart isucon2.service
	sudo cp nginx-site-conf /etc/nginx/sites-enabled/default
	sudo systemctl restart nginx.service

app-deploy:
	sudo cp app.py /opt/isucon2/python/
	sudo cp isucon2.service /etc/systemd/system/
	sudo systemctl daemon-reload
	sudo systemctl restart isucon2.service

install:
	python3 -m venv venv3
	venv3/bin/pip install -r requirements.txt

run:
	venv3/bin/gunicorn -c gunicorn_config.py -w 30 app:app --access-logfile - --access-logformat '%(h)s %(l)s %(u)s %(t)s "%(r)s" %(s)s %(b)s "%(f)s" "%(a)s" %(D)s'

kataribe:
	cat /var/log/nginx/main.log | ./kataribe


rotate-nginx-log:
	sudo mv /var/log/nginx/main.log /var/log/nginx/main.log-`date +%Y%m%d-%H%M%S` ; sudo systemctl restart nginx
