package main

// https://goji.io/

import (
	"compress/gzip"
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"
	"bytes"
	// "io/ioutil"

	_ "github.com/go-sql-driver/mysql"
	"github.com/pkg/profile"
	"goji.io"
	"goji.io/pat"
)

// create index stock_order on stock (order_id);
// mysql -e 'set global long_query_time = 1; set global slow_query_log = ON'

type Artist struct {
	ID   int
	Name string
}

type Ticket struct {
	ID        int
	Name      string
	Vacancy   int
	SoldCount int
	Artist    Artist
}

type Variation struct {
	ID        int
	Name      string
	Vacancy   int
	SoldCount int
	Ticket    Ticket
}

type Sold struct {
	ArtistName    string
	TicketName    string
	VariationName string
	SeatID        string
}

var seatIDList []string
var variationMaster map[int]Variation = make(map[int]Variation)
var recentSold []Sold

func getDb() (*sql.DB, error) {
	return sql.Open("mysql", "isucon2app:isunageruna@/isucon2")
}

func loadVariationMaster(db *sql.DB) {
	rows, err := db.Query(`
	SELECT v.id, v.name, t.id, t.name, a.id, a.name
	from variation v
	inner join ticket t on t.id = v.ticket_id
	inner join artist a on a.id = t.artist_id
	`)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		for rows.Next() {
			variation := Variation{}
			rows.Scan(&variation.ID, &variation.Name, &variation.Ticket.ID,
			&variation.Ticket.Name, &variation.Ticket.Artist.ID, &variation.Ticket.Artist.Name)
			variationMaster[variation.ID] = variation
		}
	}
	return
}

func getRecentSold(db *sql.DB) (soldHistory []Sold) {
	rows, err := db.Query(`
	SELECT seat_id, variation_name, ticket_name, artist_name
	FROM history
    ORDER BY id DESC LIMIT 10
	`)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		for rows.Next() {
			sold := Sold{}
			rows.Scan(&sold.SeatID, &sold.VariationName, &sold.TicketName, &sold.ArtistName)
			soldHistory = append(soldHistory, sold)
		}
	}
	return
}

func getArtists(db *sql.DB) (artists []Artist) {
	rows, err := db.Query("select id, name from artist")
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		for rows.Next() {
			artist := Artist{}
			rows.Scan(&artist.ID, &artist.Name)
			artists = append(artists, artist)
		}
	}
	return artists
}

func getArtist(db *sql.DB, id int) (artist Artist) {
	row := db.QueryRow("select id, name from artist where id = ?", id)
	err := row.Scan(&artist.ID, &artist.Name)
	if err != nil {
		fmt.Println("Error:", err)
	}
	return
}

func getTickets(db *sql.DB, artistID int) (tickets []Ticket) {
	rows, err := db.Query(`
	select ticket.id, ticket.name, ticket.sold_count from ticket
	where ticket.artist_id = ?`,
		artistID)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		for rows.Next() {
			ticket := Ticket{}
			rows.Scan(&ticket.ID, &ticket.Name, &ticket.SoldCount)
			ticket.Vacancy = 8192 - ticket.SoldCount
			tickets = append(tickets, ticket)
		}
	}
	return tickets
}

func getTicketCount(db *sql.DB, ticketID int) (result int) {
	row := db.QueryRow(
		`SELECT COUNT(*) AS cnt
		FROM stock
		WHERE stock.variation_id in (select id from variation where ticket_id = ?)
		 AND stock.order_id IS NULL`, ticketID)
	err := row.Scan(&result)
	if err != nil {
		fmt.Println("Error:", err)
	}
	return
}

func getTicket(db *sql.DB, id int) (ticket Ticket) {
	row := db.QueryRow(
		`SELECT t.id, t.name, t.artist_id, a.name AS artist_name FROM ticket t INNER JOIN artist a ON t.artist_id = a.id WHERE t.id = ? LIMIT 1`, id)
	err := row.Scan(&ticket.ID, &ticket.Name, &ticket.Artist.ID, &ticket.Artist.Name)
	if err != nil {
		fmt.Println("Error:", err)
	}
	return
}

func getVariations(db *sql.DB, ticketID int) (variations []Variation) {
	rows, err := db.Query(`SELECT id, name, sold_count FROM variation WHERE ticket_id = ?`, ticketID)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		for rows.Next() {
			variation := Variation{}
			rows.Scan(&variation.ID, &variation.Name, &variation.SoldCount)
			variation.Vacancy = 4096 - variation.SoldCount
			variations = append(variations, variation)
		}
	}
	return variations
}

func getVariation(db *sql.DB, variationID int) (variation Variation) {
	row := db.QueryRow(
		`SELECT v.id v_id, v.name v_name, t.id t_id, t.name t_name, a.id a_id, a.name as a_name
		 FROM variation v
		  INNER JOIN ticket t ON v.ticket_id = t.id
		  INNER JOIN artist a ON t.artist_id = a.id
		WHERE v.id = ? LIMIT 1`, variationID)
	err := row.Scan(&variation.ID, &variation.Name, &variation.Ticket.ID, &variation.Ticket.Name, &variation.Ticket.Artist.ID, &variation.Ticket.Artist.Name)
	if err != nil {
		fmt.Println("Error:", err)
	}
	return
}


func createGzipHTML(filename string, data interface{}) []byte {
	// TODO: Refactor
	tmpl, err := template.ParseFiles("./templates/" + filename)
	if err != nil {
		fmt.Println("Error: ", err)
		return []byte{}
	}
	var buffer bytes.Buffer
	zw := gzip.NewWriter(&buffer)
	err = tmpl.Execute(zw, data)
	if err != nil {
		fmt.Println("Error: ", err)
		return []byte{}
	}
	zw.Close()
	return buffer.Bytes()
}

func createHTML(filename string, data interface{}) []byte {
	tmpl, err := template.ParseFiles("./templates/" + filename)
	if err != nil {
		fmt.Println("Error: ", err)
		return []byte{}
	}
	var buffer bytes.Buffer
	err = tmpl.Execute(&buffer, data)
	if err != nil {
		fmt.Println("Error: ", err)
		return []byte{}
	}
	return buffer.Bytes()
}

func outputTemplate(w http.ResponseWriter, filename string, data interface{}) error {
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	_, err := w.Write(createHTML(filename, data))
	return err
}

func home(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.Write(homeHTML)
}

func artist(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(pat.Param(r, "id"))
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.Write(artistHTML[id])
}

func ticket(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(pat.Param(r, "id"))
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	// w.Header().Add("Content-Encoding", "gzip")
	w.Write(ticketHTML[id])
}

func buy(w http.ResponseWriter, r *http.Request) {
	db, err := getDb()
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	defer db.Close()

	r.ParseForm()
	variationID, _ := strconv.Atoi(r.PostForm.Get("variation_id"))
	memberID := r.PostForm.Get("member_id")
	variation := variationMaster[variationID]

	tx, _ := db.Begin()

	result, err := tx.Exec(`UPDATE variation SET sold_count = last_insert_id(sold_count + 1) WHERE id = ?`, variation.ID)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	soldCount, err := result.LastInsertId()

	if soldCount > 4096 {
		tx.Rollback()
		outputTemplate(w, "soldout.html", nil)
		return
	}

	_, err = tx.Exec(`UPDATE ticket SET sold_count = sold_count + 1 WHERE id = ?`, variation.Ticket.ID)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}

	index := soldCount - 1
	seatID := seatIDList[index]

	_, err = tx.Exec(`
	INSERT INTO history
	(member_id, variation_id, variation_name, ticket_id, ticket_name, artist_id, artist_name, seat_id)
	values
	(?, ?, ?, ?, ?, ?, ?, ?)`,
		memberID, variationID, variation.Name, variation.Ticket.ID, variation.Ticket.Name, variation.Ticket.Artist.ID, variation.Ticket.Artist.Name, seatID)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}

	tx.Commit()

	data := map[string]interface{}{
		"recentSold": recentSold,
		"seatID":     seatID,
		"memberID":   memberID,
	}

	outputTemplate(w, "complete.html", data)

}

func admin(w http.ResponseWriter, r *http.Request) {
	outputTemplate(w, "admin.html", nil)
}

func initMaster() {
	seatIDList = make([]string, 4096)
	for i := 0; i < 4096; i++ {
		seatIDList[i] = fmt.Sprintf("%02d-%02d", i/64, i%64)
	}

	db, err := getDb()
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	defer db.Close()

	loadVariationMaster(db)
}

func initialize(w http.ResponseWriter, r *http.Request) {
	db, err := getDb()
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	defer db.Close()

	db.Exec(`update stock set order_id = null`)
	db.Exec(`delete from order_request`)
	db.Exec(`delete from history`)
	db.Exec(`update ticket set sold_count = 0`)
	db.Exec(`update variation set sold_count = 0`)

	updateHTML()

	w.WriteHeader(302)
}

func csv(w http.ResponseWriter, r *http.Request) {
	db, err := getDb()
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	defer db.Close()

	rows, err := db.Query(`
	SELECT id, member_id, seat_id, variation_id, updated_at
         FROM history
         ORDER BY id ASC`)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		w.Header().Add("Content-Type", "text/csv")
		for rows.Next() {
			var id string
			var memberID string
			var seatID string
			var variationID string
			var updatedAt string
			rows.Scan(&id, &memberID, &seatID, &variationID, &updatedAt)
			fmt.Fprintf(w, "%s,%s,%s,%s,%s\n", id, memberID, seatID, variationID, updatedAt)
		}
	}
	return
}

var homeHTML []byte
var artistHTML map[int]([]byte) = make(map[int]([]byte))
var ticketHTML map[int]([]byte) = make(map[int]([]byte))

func updateHTML() {
	db, err := getDb()
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	defer db.Close()

	recentSold = getRecentSold(db)
	artists := getArtists(db)

	data := map[string]interface{}{
		"recentSold": recentSold,
		"artists": artists,
	}

	homeHTML = createHTML("index.html", data)

	for _, artist := range artists {
		tickets := getTickets(db, artist.ID)

		data := map[string]interface{}{
			"recentSold": recentSold,
			"artist":     artist,
			"tickets":    tickets,
		}
		artistHTML[artist.ID] = createHTML("artist.html", data)

		for _, ticket := range tickets {

			variations := getVariations(db, ticket.ID)

			var buf = make([]byte, 0, 100000)
			for _, variation := range variations {
				buf = append(buf, "<h4>"...)
				buf = append(buf, variation.Name...)
				buf = append(buf, "</h4>\n<table class=\"seats\" data-variationid=\""...)
				buf = append(buf, strconv.Itoa(variation.ID)...)
				buf = append(buf, "\">\n"...)
				for row := 0; row < 64; row++ {
					buf = append(buf, "<tr>\n"...)
					for col := 0; col < 64; col++ {
						seatID := seatIDList[row*64+col]
						state := "available"
						if col+row*64 < variation.SoldCount {
							state = "unavailable"
						}
						buf = append(buf, "<td id=\""...)
						buf = append(buf, seatID...)
						buf = append(buf, "\" class=\""...)
						buf = append(buf, state...)
						buf = append(buf, "\"></td>\n"...)
					}
					buf = append(buf, "</tr>\n"...)
				}
				buf = append(buf, "</html>\n"...)
			}
			html := string(buf)
		
			data := map[string]interface{}{
				"recentSold": recentSold,
				"ticket":     ticket,
				"variations": variations,
				"seatHTML":   template.HTML(html),
			}
			ticketHTML[ticket.ID] = createHTML("ticket.html", data)
		}

	}

}

func serveGzFile(filename string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//conts, _ := ioutil.ReadFile(filename)
		//w.Write(conts)
		http.ServeFile(w, r, filename)
		w.Header().Add("Content-Encoding", "gzip")
	})
}

func main() {
	defer profile.Start(profile.ProfilePath("."), profile.CPUProfile).Stop()
	mux := goji.NewMux()
	mux.Use(log)
	mux.HandleFunc(pat.Get("/"), home)
	mux.Handle(pat.Get("/js/jquery-1.8.2.min.js"), delay(serveGzFile("./js/jquery-1.8.2.min.js.gz")))
	mux.Handle(pat.Get("/js/jquery-ui-1.8.24.custom.min.js"), delay(serveGzFile("./js/jquery-ui-1.8.24.custom.min.js.gz")))
	mux.Handle(pat.Get("/css/*"), delay(http.FileServer(http.Dir("../staticfiles"))))
	mux.Handle(pat.Get("/js/*"), delay(http.FileServer(http.Dir("../staticfiles"))))
	mux.Handle(pat.Get("/images/*"), delay(http.FileServer(http.Dir("../staticfiles"))))
	mux.HandleFunc(pat.Get("/artist/:id"), artist)
	mux.HandleFunc(pat.Get("/ticket/:id"), ticket)
	mux.HandleFunc(pat.Post("/buy"), buy)
	mux.HandleFunc(pat.Get("/admin"), admin)
	mux.HandleFunc(pat.Post("/admin"), initialize)
	mux.HandleFunc(pat.Get("/admin/order.csv"), csv)
	initMaster()
	updateHTML()
	go func() {
		for true {
			time.Sleep(time.Millisecond * 750)
			updateHTML()
		}
	}()
	http.ListenAndServe("0.0.0.0:8080", mux)
}
