package handlers_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"swansea/db"
	"swansea/handlers"
	"swansea/store"
)

func newMoviesServer(t *testing.T) (*httptest.Server, *store.Movies) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	s := store.NewMovies(database)
	mux := http.NewServeMux()
	handlers.NewMovies(s).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, s
}

func decodeMovie(t *testing.T, resp *http.Response) store.Movie {
	t.Helper()
	defer resp.Body.Close()
	var m store.Movie
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatalf("decode Movie: %v", err)
	}
	return m
}

func decodeMovies(t *testing.T, resp *http.Response) []store.Movie {
	t.Helper()
	defer resp.Body.Close()
	var movies []store.Movie
	if err := json.NewDecoder(resp.Body).Decode(&movies); err != nil {
		t.Fatalf("decode []Movie: %v", err)
	}
	return movies
}

// --- List ---

func TestMovies_List_empty(t *testing.T) {
	srv, _ := newMoviesServer(t)
	resp := doJSON(t, "GET", srv.URL+"/api/movies", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	movies := decodeMovies(t, resp)
	if len(movies) != 0 {
		t.Errorf("expected empty list, got %d movies", len(movies))
	}
}

func TestMovies_List_returns_all(t *testing.T) {
	srv, s := newMoviesServer(t)
	s.Create(store.MovieInput{Title: "Movie A"})
	s.Create(store.MovieInput{Title: "Movie B"})

	resp := doJSON(t, "GET", srv.URL+"/api/movies", nil)
	movies := decodeMovies(t, resp)
	if len(movies) != 2 {
		t.Errorf("expected 2 movies, got %d", len(movies))
	}
}

// --- Create ---

func TestMovies_Create_success(t *testing.T) {
	srv, _ := newMoviesServer(t)
	resp := doJSON(t, "POST", srv.URL+"/api/movies", map[string]any{
		"title":       "Inception",
		"directors":   []string{"Christopher Nolan"},
		"runtime":     148,
		"genres":      []string{"Sci-Fi", "Thriller"},
		"release_date": "2010",
	})
	if resp.StatusCode != 201 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	movie := decodeMovie(t, resp)
	if movie.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if movie.Title != "Inception" {
		t.Errorf("Title = %q", movie.Title)
	}
	if len(movie.Directors) != 1 || movie.Directors[0] != "Christopher Nolan" {
		t.Errorf("Directors = %v", movie.Directors)
	}
	if movie.Runtime != 148 {
		t.Errorf("Runtime = %d, want 148", movie.Runtime)
	}
}

func TestMovies_Create_missing_title(t *testing.T) {
	srv, _ := newMoviesServer(t)
	resp := doJSON(t, "POST", srv.URL+"/api/movies", map[string]any{"runtime": 90})
	if resp.StatusCode != 400 {
		t.Fatalf("missing title: status = %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestMovies_Create_invalid_json(t *testing.T) {
	srv, _ := newMoviesServer(t)
	req, _ := http.NewRequest("POST", srv.URL+"/api/movies", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("invalid JSON: status = %d, want 400", resp.StatusCode)
	}
}

// --- Get ---

func TestMovies_Get_success(t *testing.T) {
	srv, s := newMoviesServer(t)
	movie, _ := s.Create(store.MovieInput{Title: "Jaws", Directors: []string{"Steven Spielberg"}, Runtime: 124})

	resp := doJSON(t, "GET", fmt.Sprintf("%s/api/movies/%d", srv.URL, movie.ID), nil)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	got := decodeMovie(t, resp)
	if got.Title != "Jaws" {
		t.Errorf("Title = %q", got.Title)
	}
	if got.Runtime != 124 {
		t.Errorf("Runtime = %d, want 124", got.Runtime)
	}
}

func TestMovies_Get_not_found(t *testing.T) {
	srv, _ := newMoviesServer(t)
	resp := doJSON(t, "GET", srv.URL+"/api/movies/99999", nil)
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestMovies_Get_invalid_id(t *testing.T) {
	srv, _ := newMoviesServer(t)
	resp := doJSON(t, "GET", srv.URL+"/api/movies/notanid", nil)
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// --- Update ---

func TestMovies_Update_success(t *testing.T) {
	srv, s := newMoviesServer(t)
	movie, _ := s.Create(store.MovieInput{Title: "Old Title", Runtime: 90})

	resp := doJSON(t, "PUT", fmt.Sprintf("%s/api/movies/%d", srv.URL, movie.ID), map[string]any{
		"title":   "New Title",
		"runtime": 120,
	})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	got := decodeMovie(t, resp)
	if got.Title != "New Title" {
		t.Errorf("Title = %q", got.Title)
	}
	if got.Runtime != 120 {
		t.Errorf("Runtime = %d, want 120", got.Runtime)
	}
}

func TestMovies_Update_not_found(t *testing.T) {
	srv, _ := newMoviesServer(t)
	resp := doJSON(t, "PUT", srv.URL+"/api/movies/99999", map[string]any{"title": "Ghost"})
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestMovies_Update_missing_title(t *testing.T) {
	srv, s := newMoviesServer(t)
	movie, _ := s.Create(store.MovieInput{Title: "Something"})

	resp := doJSON(t, "PUT", fmt.Sprintf("%s/api/movies/%d", srv.URL, movie.ID), map[string]any{})
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// --- Delete ---

func TestMovies_Delete_success(t *testing.T) {
	srv, s := newMoviesServer(t)
	movie, _ := s.Create(store.MovieInput{Title: "To Delete"})

	resp := doJSON(t, "DELETE", fmt.Sprintf("%s/api/movies/%d", srv.URL, movie.ID), nil)
	resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}

	_, err := s.Get(movie.ID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Error("movie should be gone after delete")
	}
}

func TestMovies_Delete_not_found(t *testing.T) {
	srv, _ := newMoviesServer(t)
	resp := doJSON(t, "DELETE", srv.URL+"/api/movies/99999", nil)
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// --- Content-Type ---

func TestMovies_JSON_content_type(t *testing.T) {
	srv, _ := newMoviesServer(t)
	resp := doJSON(t, "GET", srv.URL+"/api/movies", nil)
	defer resp.Body.Close()
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
