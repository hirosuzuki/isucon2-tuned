package main

// https://goji.io/

import (
	"database/sql"
	"fmt"
	"net/http"
	"html/template"
	"strconv"

	_ "github.com/go-sql-driver/mysql"
	"goji.io"
	"goji.io/pat"
)

type Artist struct {
	ID   int
	Name string
}

type Ticket struct {
	ID   int
	Name string
	Artist Artist
	Count int
}

type Seat struct {
	ID   string
	State bool
}

type Variation struct {
	ID   int
	Name string
	Vacancy int
	Seats [][]Seat
}

type Sold struct {
	ArtistName string
	TicketName string
	VariationName string
	SeatID string
}

func getDb() (*sql.DB, error) {
	return sql.Open("mysql", "isucon2app:isunageruna@/isucon2")
}

func getRecentSold(db *sql.DB) (soldHistory []Sold) {
	rows, err := db.Query(`
	select
 artist.name,
 ticket.name,
 variation.name,
 stock.seat_id
from order_request, stock, variation, ticket, artist
where order_request.id = stock.order_id and stock.variation_id = variation.id and variation.ticket_id = ticket.id and ticket.artist_id = artist.id
order by order_request.id desc
limit 10
	`)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		for rows.Next() {
			sold := Sold{}
			rows.Scan(&sold.ArtistName, &sold.TicketName, &sold.VariationName, &sold.SeatID)
			soldHistory = append(soldHistory, sold)
		}
	}
	return
}

func getArtists(db *sql.DB) (artists []Artist) {
	rows, err := db.Query("select id, name from artist order by id")
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
	select ticket.id, ticket.name, ticket.artist_id, artist.name,
	(select count(*) from variation, stock where variation.ticket_id = ticket.id and variation.id = stock.variation_id and order_id is null) as available_count
	from ticket left outer join artist on ticket.artist_id = artist.id
	where artist.id = ? order by ticket.id`,
	artistID)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		for rows.Next() {
			ticket := Ticket{}
			rows.Scan(&ticket.ID, &ticket.Name, &ticket.Artist.ID, &ticket.Artist.Name, &ticket.Count)
			tickets = append(tickets, ticket)
		}
	}
	return tickets
}

func getTicket(db *sql.DB, id int) (ticket Ticket) {
	row := db.QueryRow("select ticket.id, ticket.name, ticket.artist_id, artist.name from ticket left outer join artist on ticket.artist_id = artist.id where ticket.id = ?", id)
	err := row.Scan(&ticket.ID, &ticket.Name, &ticket.Artist.ID, &ticket.Artist.Name)
	if err != nil {
		fmt.Println("Error:", err)
	}
	return
}

func getVariations(db *sql.DB, variationID int) (variations []Variation) {
	rows, err := db.Query("select variation.id, variation.name from variation where ticket_id = ? order by variation.id", variationID)
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

func getSeats(db *sql.DB, variationID int) (seats [][]Seat, vacancy int) {
	seats = make([][]Seat, 64)
	vacancy =64 * 64

	for row := 0; row < 64; row ++ {
		seats[row] = make([]Seat, 64)
		for col := 0; col < 64; col ++ {
			seats[row][col] = Seat{fmt.Sprintf("%02d-%02d", row, col), false}
		}
	}

	rows, err := db.Query("select seat_id from stock where variation_id = ? and order_id is not null", variationID)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		for rows.Next() {
			var seatID string
			rows.Scan(&seatID)
			row, _ := strconv.Atoi(seatID[0:2])
			col, _ := strconv.Atoi(seatID[3:5])
			seats[row][col].State = true
			vacancy --
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

	data := map[string]interface{} {
		"recentSold": getRecentSold(db),
		"artists": getArtists(db),
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

	data := map[string]interface{} {
		"recentSold": getRecentSold(db),
		"artist": getArtist(db, id),
		"tickets": getTickets(db, id),
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

	data := map[string]interface{} {
		"recentSold": getRecentSold(db),
		"ticket": getTicket(db, id),
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

	fmt.Println("Order ID: ", orderID)

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

	tx.Commit()

	data := map[string]interface{} {
		"recentSold": getRecentSold(db),
		"seatID": seatID,
		"memberID": memberID,
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
	mux := goji.NewMux()
	mux.Use(log)
	mux.HandleFunc(pat.Get("/"), home)
	mux.Handle(pat.Get("/css/*"), http.FileServer(http.Dir("../staticfiles")))
	mux.Handle(pat.Get("/js/*"), http.FileServer(http.Dir("../staticfiles")))
	mux.Handle(pat.Get("/images/*"), http.FileServer(http.Dir("../staticfiles")))
	mux.HandleFunc(pat.Get("/artist/:id"), artist)
	mux.HandleFunc(pat.Get("/ticket/:id"), ticket)
	mux.HandleFunc(pat.Post("/buy"), buy)
	mux.HandleFunc(pat.Get("/admin"), admin)
	mux.HandleFunc(pat.Post("/admin"), initialize)
	mux.HandleFunc(pat.Get("/admin/order.csv"), csv)
	http.ListenAndServe("0.0.0.0:8080", mux)
}
