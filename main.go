package main

import (
	"database/sql"
	"fmt"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
)

var DB *sql.DB

func simpleGet(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "You came and you get it!")
}
func main() {
	path := filepath.Join("script.sql")

	c, ioErr := ioutil.ReadFile(path)
	if ioErr != nil {
		// handle error.
	}
	script_string := string(c)

	// Connect to postgreSql db
	DB, err := sql.Open(
		"postgres",
		fmt.Sprint("user=postgres "+
			"password=admin "+
			"dbname=postgres "+
			"host=localhost "+
			"port=5432 "),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer DB.Close()
	if err := DB.Ping(); err != nil {
		log.Fatal(err)
	}

	DB.Exec(script_string)

	r := mux.NewRouter()

	r.HandleFunc("/", simpleGet)
	r.HandleFunc("/get", simpleGet)
	http.ListenAndServe(":7777", r)
}