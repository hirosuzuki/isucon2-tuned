build:
	go build

run:
	go build && ./go | tee accesslog

kataribe:
	cat accesslog | kataribe | less

pprof:
	go tool pprof -http=":8081" main cpu.pprof

slowquery_on:
	mysql -e 'set global long_query_time = 1; set global slow_query_log = ON'
