package main

// https://goji.io/

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"
	"html/template"

	_ "github.com/go-sql-driver/mysql"
	"goji.io"
	"goji.io/pat"
)

func home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<a href='/hello/goji'>Hello</a> <a href='/db'>db</a>")
}

func hello(w http.ResponseWriter, r *http.Request) {
	name := pat.Param(r, "name")
	time.Sleep(time.Second * 1)
	fmt.Fprintf(w, "Hello, %s!", name)
}

type Artist struct {
	Id   int
	Name string
}

func db(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("mysql", "isucon2app:isunageruna@/isucon2")
	if err != nil {
		fmt.Fprintf(w, "ERR")
		return
	}
	defer db.Close()

	stmt, err := db.Prepare("select id, name from artist")
	if err != nil {
		fmt.Fprintf(w, "ERR SELECT")
		return
	}

	rows, _ := stmt.Query()

	var artists []Artist
	for rows.Next() {
		artist := Artist{}
		rows.Scan(&artist.Id, &artist.Name)
		artists = append(artists, artist)
	}

	tmpl, err := template.ParseFiles("db.html")
	if err != nil {

	}

	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	// https://golang.org/pkg/html/template/
	tmpl.Execute(w, artists)
}

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

func main() {
	mux := goji.NewMux()
	mux.Use(log)
	mux.HandleFunc(pat.Get("/"), home)
	mux.HandleFunc(pat.Get("/db"), db)
	mux.HandleFunc(pat.Get("/hello/:name"), hello)
	http.ListenAndServe("localhost:8080", mux)
}
