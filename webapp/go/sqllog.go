package main

import (
	"fmt"
	"context"
	"database/sql"
	"database/sql/driver"
	_ "github.com/mattn/go-sqlite3"
	"time"
	"github.com/go-sql-driver/mysql"
	"github.com/shogo82148/go-sql-proxy"
)

func sqlPreQueryExec(_ context.Context, _ *proxy.Stmt, _ []driver.NamedValue) (interface{}, error) {
	return time.Now(), nil
}

func sqlPostQuery(_ context.Context, ctx interface{}, stmt *proxy.Stmt, args []driver.NamedValue, _ driver.Rows, _ error) error {
	//fmt.Printf("Query: %s; args = %v (%s)\n", stmt.QueryString, args, time.Since(ctx.(time.Time)))
	db.Exec(`INSERT INTO sqllog (stmt, args, time) VALUES (?, ?, ?)`,
	stmt.QueryString, fmt.Sprintf("%v", args), time.Since(ctx.(time.Time)).Nanoseconds(),
	)
	return nil
}

func sqlPostExec(_ context.Context, ctx interface{}, stmt *proxy.Stmt, args []driver.NamedValue, _ driver.Result, _ error) error {
	//fmt.Printf("Query: %s; args = %v (%s)\n", stmt.QueryString, args, time.Since(ctx.(time.Time)))
	db.Exec(`INSERT INTO sqllog (stmt, args, time) VALUES (?, ?, ?)`,
	stmt.QueryString, fmt.Sprintf("%v", args), time.Since(ctx.(time.Time)).Nanoseconds(),
	)
	return nil
}


// https://github.com/shogo82148/go-sql-proxy/blob/master/hooks.go
// preQuery(c context.Context, stmt *Stmt, args []driver.NamedValue) (interface{}, error)
// preExec(c context.Context, stmt *Stmt, args []driver.NamedValue) (interface{}, error)
// postQuery(c context.Context, ctx interface{}, stmt *Stmt, args []driver.NamedValue, rows driver.Rows, err error) error
// postExec(c context.Context, ctx interface{}, stmt *Stmt, args []driver.NamedValue, result driver.Result, err error) error

var db *sql.DB

func RegisterMySQLTrace() {
	fmt.Println("SQL Log")
	
	db, _ = sql.Open("sqlite3", "file:trace.db")

	sql.Register("mysql-proxy", proxy.NewProxyContext(&mysql.MySQLDriver{}, &proxy.HooksContext{
		PreQuery: sqlPreQueryExec,
		PreExec: sqlPreQueryExec,
		PostQuery: sqlPostQuery,
		PostExec: sqlPostExec,
	}))

	db.Exec(`CREATE TABLE IF NOT EXISTS sqllog (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		starttime DEFAULT CURRENT_TIMESTAMP,
		stmt TEXT,
		args TEXT,
		time INTEGER
	)`)
	db.Exec(`DELETE FROM sqllog`)

}