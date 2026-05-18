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

func newGamesUIServer(t *testing.T) (*httptest.Server, *store.VideoGames) {
	t.Helper()
	assetsFS := os.DirFS(repoRoot())

	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	s := store.NewVideoGames(database)
	ui, err := handlers.NewGamesUI(s, assetsFS, nil)
	if err != nil {
		t.Fatalf("NewGamesUI: %v", err)
	}

	mux := http.NewServeMux()
	ui.Register(mux)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, s
}

func TestGamesUI_page(t *testing.T) {
	srv, _ := newGamesUIServer(t)
	resp, err := http.Get(srv.URL + "/games")
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

func TestGamesUI_list_empty(t *testing.T) {
	srv, _ := newGamesUIServer(t)
	resp, err := http.Get(srv.URL + "/ui/games")
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

func TestGamesUI_list_shows_games(t *testing.T) {
	srv, s := newGamesUIServer(t)
	s.Create(store.VideoGameInput{Title: "Super Mario Bros", Platform: "NES"})

	resp, err := http.Get(srv.URL + "/ui/games")
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if !strings.Contains(body, "Super Mario Bros") {
		t.Error("list should show game title")
	}
	if !strings.Contains(body, "NES") {
		t.Error("list should show platform")
	}
}

func TestGamesUI_create_success(t *testing.T) {
	srv, _ := newGamesUIServer(t)
	form := url.Values{"title": {"Zelda"}, "platform": {"SNES"}}
	resp, err := http.PostForm(srv.URL+"/ui/games", form)
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
	if !strings.Contains(body, "Zelda") {
		t.Error("response should contain new game")
	}
}

func TestGamesUI_create_missing_title(t *testing.T) {
	srv, _ := newGamesUIServer(t)
	form := url.Values{"platform": {"PS5"}}
	resp, err := http.PostForm(srv.URL+"/ui/games", form)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("missing title: status = %d, want 400", resp.StatusCode)
	}
}

func TestGamesUI_editForm(t *testing.T) {
	srv, s := newGamesUIServer(t)
	game, _ := s.Create(store.VideoGameInput{Title: "Halo", Platform: "Xbox"})

	resp, err := http.Get(srv.URL + "/ui/games/" + itoa(game.ID) + "/edit")
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !strings.Contains(body, "Halo") {
		t.Error("edit form should contain game title")
	}
}

func TestGamesUI_editForm_not_found(t *testing.T) {
	srv, _ := newGamesUIServer(t)
	resp, err := http.Get(srv.URL + "/ui/games/99999/edit")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestGamesUI_update_success(t *testing.T) {
	srv, s := newGamesUIServer(t)
	game, _ := s.Create(store.VideoGameInput{Title: "Old Title"})

	form := url.Values{"title": {"New Title"}, "platform": {"PC"}}
	req, _ := http.NewRequest("PUT", srv.URL+"/ui/games/"+itoa(game.ID), strings.NewReader(form.Encode()))
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

func TestGamesUI_update_missing_title(t *testing.T) {
	srv, s := newGamesUIServer(t)
	game, _ := s.Create(store.VideoGameInput{Title: "Something"})

	form := url.Values{"platform": {"PC"}}
	req, _ := http.NewRequest("PUT", srv.URL+"/ui/games/"+itoa(game.ID), strings.NewReader(form.Encode()))
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

func TestGamesUI_delete(t *testing.T) {
	srv, s := newGamesUIServer(t)
	game, _ := s.Create(store.VideoGameInput{Title: "To Delete"})

	req, _ := http.NewRequest("DELETE", srv.URL+"/ui/games/"+itoa(game.ID), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestGamesUI_delete_not_found(t *testing.T) {
	srv, _ := newGamesUIServer(t)
	req, _ := http.NewRequest("DELETE", srv.URL+"/ui/games/99999", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200 (idempotent delete)", resp.StatusCode)
	}
}

func TestGamesUI_filters(t *testing.T) {
	srv, s := newGamesUIServer(t)
	s.Create(store.VideoGameInput{Title: "Zelda", Platform: "Switch", Genres: []string{"Action"}})

	resp, err := http.Get(srv.URL + "/ui/games/filters")
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !strings.Contains(body, "Switch") {
		t.Error("filters should list platform")
	}
	if !strings.Contains(body, "Action") {
		t.Error("filters should list genre")
	}
}

func TestGamesUI_search(t *testing.T) {
	srv, s := newGamesUIServer(t)
	s.Create(store.VideoGameInput{Title: "Metroid Prime"})
	s.Create(store.VideoGameInput{Title: "Donkey Kong"})

	resp, err := http.Get(srv.URL + "/ui/games/search?q=metroid")
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if !strings.Contains(body, "Metroid Prime") {
		t.Error("search should return matching game")
	}
	if strings.Contains(body, "Donkey Kong") {
		t.Error("search should not return non-matching game")
	}
}

func TestGamesUI_addForm(t *testing.T) {
	srv, _ := newGamesUIServer(t)
	resp, err := http.Get(srv.URL + "/ui/games/add")
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !strings.Contains(body, "Add Game") {
		t.Error("add form should say 'Add Game'")
	}
}
