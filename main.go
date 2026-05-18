package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"swansea/covers"
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

	metadataPath := os.Getenv("METADATA_PATH")
	if metadataPath == "" {
		metadataPath = "/metadata"
	}

	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()
	database.SetMaxOpenConns(1)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	bookStore := store.NewBooks(database)
	gameStore := store.NewVideoGames(database)
	movieStore := store.NewMovies(database)
	musicStore := store.NewMusicAlbums(database)

	// JSON API
	handlers.NewBooks(bookStore).Register(mux)
	handlers.NewGames(gameStore).Register(mux)
	handlers.NewMovies(movieStore).Register(mux)
	handlers.NewMusic(musicStore).Register(mux)

	// HTML UI
	cv := covers.New(filepath.Join(metadataPath, "covers"))
	ui, err := handlers.NewUI(bookStore, assets, cv, metadataPath)
	if err != nil {
		log.Fatalf("failed to load templates: %v", err)
	}
	ui.Register(mux, assets)

	gamesUI, err := handlers.NewGamesUI(gameStore, assets, cv)
	if err != nil {
		log.Fatalf("failed to load games templates: %v", err)
	}
	gamesUI.Register(mux)

	moviesUI, err := handlers.NewMoviesUI(movieStore, assets, cv)
	if err != nil {
		log.Fatalf("failed to load movies templates: %v", err)
	}
	moviesUI.Register(mux)

	musicUI, err := handlers.NewMusicUI(musicStore, assets, cv)
	if err != nil {
		log.Fatalf("failed to load music templates: %v", err)
	}
	musicUI.Register(mux)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		log.Printf("server listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-quit
	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
