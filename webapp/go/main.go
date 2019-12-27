package main

// https://goji.io/

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"goji.io"
	"goji.io/pat"
)

func home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<a href='/hello/goji'>Hello</a>")
}

func hello(w http.ResponseWriter, r *http.Request) {
	name := pat.Param(r, "name")
	time.Sleep(time.Second * 1)
	fmt.Fprintf(w, "Hello, %s!", name)
}

// https://stackoverflow.com/questions/36706033/go-http-listenandserve-logging-response
type loggingResponseWriter struct {
	status  int
	bodyLen int
	http.ResponseWriter
}

func (w *loggingResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *loggingResponseWriter) Write(body []byte) (int, error) {
	w.bodyLen += len(body)
	return w.ResponseWriter.Write(body)
}

func log(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		lw := &loggingResponseWriter{
			status: 200,
			ResponseWriter: w,
		}
		now := time.Now()
		inner.ServeHTTP(lw, r)
		deltaTime := time.Now().Sub(now)
		fmt.Printf("%s - - [%s] \"%s %s\" %d %d \"%s\" \"%s\" %f\n",
			strings.Split(r.RemoteAddr, ":")[0],
			now.Format("02/Jan/2006:15:04:05 -0700"),
			r.Method,
			r.RequestURI,
			lw.status,
			lw.bodyLen,
			r.Header.Get("Referer"),
			r.Header.Get("User-Agent"),
			float64(deltaTime)/1000000000)
	}
	return http.HandlerFunc(mw)
}

func main() {
	mux := goji.NewMux()
	mux.Use(log)
	mux.HandleFunc(pat.Get("/"), home)
	mux.HandleFunc(pat.Get("/hello/:name"), hello)
	http.ListenAndServe("localhost:8080", mux)
}
