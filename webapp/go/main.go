package main

// https://goji.io/

import (
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	// "cloud.google.com/go/profiler"
	"github.com/pkg/profile"
	_ "github.com/go-sql-driver/mysql"
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
	ID     int
	Name   string
	Count  int
	Artist Artist
}

type Seat struct {
	ID    string
	State bool
}

type Variation struct {
	ID      int
	Name    string
	Vacancy int
	Ticket Ticket
	Seats   [][]Seat
}

type Sold struct {
	ArtistName    string
	TicketName    string
	VariationName string
	SeatID        string
}

func getDb() (*sql.DB, error) {
	return sql.Open("mysql", "isucon2app:isunageruna@/isucon2")
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
	select ticket.id, ticket.name from ticket
	where ticket.artist_id = ?`,
		artistID)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		for rows.Next() {
			ticket := Ticket{}
			rows.Scan(&ticket.ID, &ticket.Name)
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
	rows, err := db.Query(`SELECT id, name FROM variation WHERE ticket_id = ?`, ticketID)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		for rows.Next() {
			variation := Variation{}
			rows.Scan(&variation.ID, &variation.Name)
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

func getSeats(db *sql.DB, variationID int) (seats [][]Seat, vacancy int) {
	seats = make([][]Seat, 64)
	vacancy = 64 * 64

	for row := 0; row < 64; row++ {
		seats[row] = make([]Seat, 64)
		for col := 0; col < 64; col++ {
			seats[row][col] = Seat{fmt.Sprintf("%02d-%02d", row, col), false}
		}
	}

	rows, err := db.Query(`SELECT seat_id, order_id FROM stock WHERE variation_id = ?`, variationID)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		for rows.Next() {
			var seatID string
			var orderID string
			rows.Scan(&seatID, &orderID)
			row, _ := strconv.Atoi(seatID[0:2])
			col, _ := strconv.Atoi(seatID[3:5])
			if orderID != "" {
				seats[row][col].State = true
				vacancy--
			}
		}
	}
	return
}

func outputTemplate(w http.ResponseWriter, filename string, data interface{}) error {
	tmpl, err := template.ParseFiles("./templates/" + filename)
	if err != nil {
		fmt.Println("Error: ", err)
		return err
	}
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	err = tmpl.Execute(w, data)
	if err != nil {
		fmt.Println("Error: ", err)
		return err
	}
	return nil
}

func home(w http.ResponseWriter, r *http.Request) {
	db, err := getDb()
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	defer db.Close()

	data := map[string]interface{}{
		"recentSold": getRecentSold(db),
		"artists":    getArtists(db),
	}

	outputTemplate(w, "index.html", data)
}

func artist(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(pat.Param(r, "id"))

	db, err := getDb()
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	defer db.Close()

	tickets := getTickets(db, id)
	for i := range tickets {
		tickets[i].Count = getTicketCount(db, tickets[i].ID)
	}

	data := map[string]interface{}{
		"recentSold": getRecentSold(db),
		"artist":     getArtist(db, id),
		"tickets":    tickets,
	}

	outputTemplate(w, "artist.html", data)
}

func ticket(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(pat.Param(r, "id"))

	db, err := getDb()
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	defer db.Close()

	variations := getVariations(db, id)
	for i := range variations {
		seats, vacancy := getSeats(db, variations[i].ID)
		variations[i].Vacancy = vacancy
		variations[i].Seats = seats
	}

	data := map[string]interface{}{
		"recentSold": getRecentSold(db),
		"ticket":     getTicket(db, id),
		"variations": variations,
	}
	outputTemplate(w, "ticket.html", data)
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
	variation := getVariation(db, variationID)

	tx, _ := db.Begin()

	result, err := tx.Exec(`INSERT INTO order_request (member_id) VALUES (?)`, memberID)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}

	orderID, err := result.LastInsertId()
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}

	result, err = tx.Exec(`
	UPDATE stock SET order_id = ?
	WHERE variation_id = ? AND order_id IS NULL
	ORDER BY id
	LIMIT 1`, orderID, variationID)

	if err != nil {
		fmt.Println("Error: ", err)
		return
	}

	rowAffected, _ := result.RowsAffected()

	if rowAffected < 1 {
		tx.Rollback()
		outputTemplate(w, "soldout.html", nil)
		return
	}

	row := tx.QueryRow(`SELECT seat_id FROM stock WHERE order_id = ? LIMIT 1`, orderID)
	var seatID string
	err = row.Scan(&seatID)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}


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
		"recentSold": getRecentSold(db),
		"seatID":     seatID,
		"memberID":   memberID,
	}

	outputTemplate(w, "complete.html", data)

}

func admin(w http.ResponseWriter, r *http.Request) {
	outputTemplate(w, "admin.html", nil)
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
	SELECT order_request.*, stock.seat_id, stock.variation_id, stock.updated_at
         FROM order_request JOIN stock ON order_request.id = stock.order_id
         ORDER BY order_request.id ASC`)
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

func main() {
	defer profile.Start(profile.ProfilePath("."), profile.CPUProfile).Stop()
	/*
	profiler.Start(profiler.Config{
		Service:        "isucon2-0001",
		ServiceVersion: "1.0.0",
		// ProjectID must be set if not running on GCP.
		// ProjectID: "my-project",
	})
	*/
	mux := goji.NewMux()
	mux.Use(log)
	mux.HandleFunc(pat.Get("/"), home)
	mux.Handle(pat.Get("/css/*"), delay(http.FileServer(http.Dir("../staticfiles"))))
	mux.Handle(pat.Get("/js/*"), delay(http.FileServer(http.Dir("../staticfiles"))))
	mux.Handle(pat.Get("/images/*"), delay(http.FileServer(http.Dir("../staticfiles"))))
	mux.HandleFunc(pat.Get("/artist/:id"), artist)
	mux.HandleFunc(pat.Get("/ticket/:id"), ticket)
	mux.HandleFunc(pat.Post("/buy"), buy)
	mux.HandleFunc(pat.Get("/admin"), admin)
	mux.HandleFunc(pat.Post("/admin"), initialize)
	mux.HandleFunc(pat.Get("/admin/order.csv"), csv)
	http.ListenAndServe("0.0.0.0:8080", mux)
}
