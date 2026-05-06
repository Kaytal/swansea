package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"swansea/db"
	"swansea/handlers"
	"swansea/store"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "/data/swansea.db"
	}

	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	bookStore := store.NewBooks(database)

	// JSON API
	handlers.NewBooks(bookStore).Register(mux)

	// HTML UI
	ui, err := handlers.NewUI(bookStore, assets)
	if err != nil {
		log.Fatalf("failed to load templates: %v", err)
	}
	ui.Register(mux, assets)

	log.Printf("server listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
