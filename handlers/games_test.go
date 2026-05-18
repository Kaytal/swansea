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

func newGamesServer(t *testing.T) (*httptest.Server, *store.VideoGames) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	s := store.NewVideoGames(database)
	mux := http.NewServeMux()
	handlers.NewGames(s).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, s
}

func decodeGame(t *testing.T, resp *http.Response) store.VideoGame {
	t.Helper()
	defer resp.Body.Close()
	var g store.VideoGame
	if err := json.NewDecoder(resp.Body).Decode(&g); err != nil {
		t.Fatalf("decode VideoGame: %v", err)
	}
	return g
}

func decodeGames(t *testing.T, resp *http.Response) []store.VideoGame {
	t.Helper()
	defer resp.Body.Close()
	var games []store.VideoGame
	if err := json.NewDecoder(resp.Body).Decode(&games); err != nil {
		t.Fatalf("decode []VideoGame: %v", err)
	}
	return games
}

// --- List ---

func TestGames_List_empty(t *testing.T) {
	srv, _ := newGamesServer(t)
	resp := doJSON(t, "GET", srv.URL+"/api/games", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	games := decodeGames(t, resp)
	if len(games) != 0 {
		t.Errorf("expected empty list, got %d games", len(games))
	}
}

func TestGames_List_returns_all(t *testing.T) {
	srv, s := newGamesServer(t)
	s.Create(store.VideoGameInput{Title: "Game A", Platform: "PS5"})
	s.Create(store.VideoGameInput{Title: "Game B", Platform: "Xbox"})

	resp := doJSON(t, "GET", srv.URL+"/api/games", nil)
	games := decodeGames(t, resp)
	if len(games) != 2 {
		t.Errorf("expected 2 games, got %d", len(games))
	}
}

// --- Create ---

func TestGames_Create_success(t *testing.T) {
	srv, _ := newGamesServer(t)
	resp := doJSON(t, "POST", srv.URL+"/api/games", map[string]any{
		"title":      "Elden Ring",
		"platform":   "PS5",
		"developers": []string{"FromSoftware"},
		"genres":     []string{"Action", "RPG"},
	})
	if resp.StatusCode != 201 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	game := decodeGame(t, resp)
	if game.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if game.Title != "Elden Ring" {
		t.Errorf("Title = %q", game.Title)
	}
	if game.Platform != "PS5" {
		t.Errorf("Platform = %q", game.Platform)
	}
	if len(game.Developers) != 1 || game.Developers[0] != "FromSoftware" {
		t.Errorf("Developers = %v", game.Developers)
	}
}

func TestGames_Create_missing_title(t *testing.T) {
	srv, _ := newGamesServer(t)
	resp := doJSON(t, "POST", srv.URL+"/api/games", map[string]any{"platform": "PS5"})
	if resp.StatusCode != 400 {
		t.Fatalf("missing title: status = %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestGames_Create_invalid_json(t *testing.T) {
	srv, _ := newGamesServer(t)
	req, _ := http.NewRequest("POST", srv.URL+"/api/games", strings.NewReader("not json"))
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

func TestGames_Get_success(t *testing.T) {
	srv, s := newGamesServer(t)
	game, _ := s.Create(store.VideoGameInput{Title: "Minecraft", Platform: "PC", Developers: []string{"Mojang"}})

	resp := doJSON(t, "GET", fmt.Sprintf("%s/api/games/%d", srv.URL, game.ID), nil)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	got := decodeGame(t, resp)
	if got.Title != "Minecraft" {
		t.Errorf("Title = %q", got.Title)
	}
	if len(got.Developers) != 1 || got.Developers[0] != "Mojang" {
		t.Errorf("Developers = %v", got.Developers)
	}
}

func TestGames_Get_not_found(t *testing.T) {
	srv, _ := newGamesServer(t)
	resp := doJSON(t, "GET", srv.URL+"/api/games/99999", nil)
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestGames_Get_invalid_id(t *testing.T) {
	srv, _ := newGamesServer(t)
	resp := doJSON(t, "GET", srv.URL+"/api/games/notanid", nil)
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// --- Update ---

func TestGames_Update_success(t *testing.T) {
	srv, s := newGamesServer(t)
	game, _ := s.Create(store.VideoGameInput{Title: "Old Title", Platform: "PS4"})

	resp := doJSON(t, "PUT", fmt.Sprintf("%s/api/games/%d", srv.URL, game.ID), map[string]any{
		"title":    "New Title",
		"platform": "PS5",
	})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	got := decodeGame(t, resp)
	if got.Title != "New Title" {
		t.Errorf("Title = %q", got.Title)
	}
	if got.Platform != "PS5" {
		t.Errorf("Platform = %q", got.Platform)
	}
}

func TestGames_Update_not_found(t *testing.T) {
	srv, _ := newGamesServer(t)
	resp := doJSON(t, "PUT", srv.URL+"/api/games/99999", map[string]any{"title": "Ghost"})
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestGames_Update_missing_title(t *testing.T) {
	srv, s := newGamesServer(t)
	game, _ := s.Create(store.VideoGameInput{Title: "Something"})

	resp := doJSON(t, "PUT", fmt.Sprintf("%s/api/games/%d", srv.URL, game.ID), map[string]any{})
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// --- Delete ---

func TestGames_Delete_success(t *testing.T) {
	srv, s := newGamesServer(t)
	game, _ := s.Create(store.VideoGameInput{Title: "To Delete"})

	resp := doJSON(t, "DELETE", fmt.Sprintf("%s/api/games/%d", srv.URL, game.ID), nil)
	resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}

	_, err := s.Get(game.ID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Error("game should be gone after delete")
	}
}

func TestGames_Delete_not_found(t *testing.T) {
	srv, _ := newGamesServer(t)
	resp := doJSON(t, "DELETE", srv.URL+"/api/games/99999", nil)
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// --- Content-Type ---

func TestGames_JSON_content_type(t *testing.T) {
	srv, _ := newGamesServer(t)
	resp := doJSON(t, "GET", srv.URL+"/api/games", nil)
	defer resp.Body.Close()
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
