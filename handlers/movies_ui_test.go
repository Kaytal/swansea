package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"swansea/db"
	"swansea/handlers"
	"swansea/store"
)

func newMoviesUIServer(t *testing.T) (*httptest.Server, *store.Movies) {
	t.Helper()
	assetsFS := os.DirFS(repoRoot())

	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	s := store.NewMovies(database)
	ui, err := handlers.NewMoviesUI(s, assetsFS, nil)
	if err != nil {
		t.Fatalf("NewMoviesUI: %v", err)
	}

	mux := http.NewServeMux()
	ui.Register(mux)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, s
}

func TestMoviesUI_page(t *testing.T) {
	srv, _ := newMoviesUIServer(t)
	resp, err := http.Get(srv.URL + "/movies")
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !strings.Contains(body, "Swansea Library") {
		t.Error("page should contain 'Swansea Library'")
	}
}

func TestMoviesUI_list_empty(t *testing.T) {
	srv, _ := newMoviesUIServer(t)
	resp, err := http.Get(srv.URL + "/ui/movies")
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !strings.Contains(body, "empty") {
		t.Error("empty library should show empty state")
	}
}

func TestMoviesUI_list_shows_movies(t *testing.T) {
	srv, s := newMoviesUIServer(t)
	s.Create(store.MovieInput{Title: "Inception", Directors: []string{"Christopher Nolan"}})

	resp, err := http.Get(srv.URL + "/ui/movies")
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if !strings.Contains(body, "Inception") {
		t.Error("list should show movie title")
	}
	if !strings.Contains(body, "Christopher Nolan") {
		t.Error("list should show director")
	}
}

func TestMoviesUI_create_success(t *testing.T) {
	srv, _ := newMoviesUIServer(t)
	form := url.Values{"title": {"Dune"}, "directors": {"Denis Villeneuve"}}
	resp, err := http.PostForm(srv.URL+"/ui/movies", form)
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if resp.Header.Get("HX-Trigger") != "closeModal" {
		t.Error("HX-Trigger should be closeModal")
	}
	if !strings.Contains(body, "Dune") {
		t.Error("response should contain new movie")
	}
}

func TestMoviesUI_create_missing_title(t *testing.T) {
	srv, _ := newMoviesUIServer(t)
	form := url.Values{"runtime": {"120"}}
	resp, err := http.PostForm(srv.URL+"/ui/movies", form)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("missing title: status = %d, want 400", resp.StatusCode)
	}
}

func TestMoviesUI_editForm(t *testing.T) {
	srv, s := newMoviesUIServer(t)
	movie, _ := s.Create(store.MovieInput{Title: "Blade Runner"})

	resp, err := http.Get(srv.URL + "/ui/movies/" + itoa(movie.ID) + "/edit")
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !strings.Contains(body, "Blade Runner") {
		t.Error("edit form should contain movie title")
	}
}

func TestMoviesUI_update_success(t *testing.T) {
	srv, s := newMoviesUIServer(t)
	movie, _ := s.Create(store.MovieInput{Title: "Old Title"})

	form := url.Values{"title": {"New Title"}, "runtime": {"90"}}
	req, _ := http.NewRequest("PUT", srv.URL+"/ui/movies/"+itoa(movie.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if resp.Header.Get("HX-Trigger") != "closeModal" {
		t.Error("HX-Trigger should be closeModal")
	}
	if !strings.Contains(body, "New Title") {
		t.Error("update response should contain new title")
	}
}

func TestMoviesUI_update_missing_title(t *testing.T) {
	srv, s := newMoviesUIServer(t)
	movie, _ := s.Create(store.MovieInput{Title: "Something"})

	form := url.Values{"runtime": {"100"}}
	req, _ := http.NewRequest("PUT", srv.URL+"/ui/movies/"+itoa(movie.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestMoviesUI_delete(t *testing.T) {
	srv, s := newMoviesUIServer(t)
	movie, _ := s.Create(store.MovieInput{Title: "To Delete"})

	req, _ := http.NewRequest("DELETE", srv.URL+"/ui/movies/"+itoa(movie.ID), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestMoviesUI_filters(t *testing.T) {
	srv, s := newMoviesUIServer(t)
	s.Create(store.MovieInput{Title: "Alien", Directors: []string{"Ridley Scott"}, Genres: []string{"Sci-Fi"}})

	resp, err := http.Get(srv.URL + "/ui/movies/filters")
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !strings.Contains(body, "Ridley Scott") {
		t.Error("filters should list director")
	}
	if !strings.Contains(body, "Sci-Fi") {
		t.Error("filters should list genre")
	}
}

func TestMoviesUI_search(t *testing.T) {
	srv, s := newMoviesUIServer(t)
	s.Create(store.MovieInput{Title: "The Matrix"})
	s.Create(store.MovieInput{Title: "Interstellar"})

	resp, err := http.Get(srv.URL + "/ui/movies/search?q=matrix")
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if !strings.Contains(body, "The Matrix") {
		t.Error("search should return matching movie")
	}
	if strings.Contains(body, "Interstellar") {
		t.Error("search should not return non-matching movie")
	}
}

func TestMoviesUI_addForm(t *testing.T) {
	srv, _ := newMoviesUIServer(t)
	resp, err := http.Get(srv.URL + "/ui/movies/add")
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !strings.Contains(body, "Add Movie") {
		t.Error("add form should say 'Add Movie'")
	}
}
