#!/bin/sh

while true
do
	sleep 0.5
	curl -s http://127.0.0.1:80/update -O /dev/null
done
