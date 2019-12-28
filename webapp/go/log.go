package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// https://stackoverflow.com/questions/36706033/go-http-listenandserve-logging-response
type LoggingResponseWriter struct {
	status  int
	bodyLen int
	http.ResponseWriter
}

func (w *LoggingResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *LoggingResponseWriter) Write(body []byte) (int, error) {
	w.bodyLen += len(body)
	return w.ResponseWriter.Write(body)
}

func log(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		lw := &LoggingResponseWriter{
			status:         200,
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

func delay(inner http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Millisecond * 500)
		inner.ServeHTTP(w, r)
	})
}
