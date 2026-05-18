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

func newMusicServer(t *testing.T) (*httptest.Server, *store.MusicAlbums) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	s := store.NewMusicAlbums(database)
	mux := http.NewServeMux()
	handlers.NewMusic(s).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, s
}

func decodeAlbum(t *testing.T, resp *http.Response) store.MusicAlbum {
	t.Helper()
	defer resp.Body.Close()
	var a store.MusicAlbum
	if err := json.NewDecoder(resp.Body).Decode(&a); err != nil {
		t.Fatalf("decode MusicAlbum: %v", err)
	}
	return a
}

func decodeAlbums(t *testing.T, resp *http.Response) []store.MusicAlbum {
	t.Helper()
	defer resp.Body.Close()
	var albums []store.MusicAlbum
	if err := json.NewDecoder(resp.Body).Decode(&albums); err != nil {
		t.Fatalf("decode []MusicAlbum: %v", err)
	}
	return albums
}

// --- List ---

func TestMusic_List_empty(t *testing.T) {
	srv, _ := newMusicServer(t)
	resp := doJSON(t, "GET", srv.URL+"/api/music", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	albums := decodeAlbums(t, resp)
	if len(albums) != 0 {
		t.Errorf("expected empty list, got %d albums", len(albums))
	}
}

func TestMusic_List_returns_all(t *testing.T) {
	srv, s := newMusicServer(t)
	s.Create(store.MusicAlbumInput{Title: "Album A"})
	s.Create(store.MusicAlbumInput{Title: "Album B"})

	resp := doJSON(t, "GET", srv.URL+"/api/music", nil)
	albums := decodeAlbums(t, resp)
	if len(albums) != 2 {
		t.Errorf("expected 2 albums, got %d", len(albums))
	}
}

// --- Create ---

func TestMusic_Create_success(t *testing.T) {
	srv, _ := newMusicServer(t)
	resp := doJSON(t, "POST", srv.URL+"/api/music", map[string]any{
		"title":          "Nevermind",
		"artists":        []string{"Nirvana"},
		"label":          "DGC Records",
		"track_count":    12,
		"genres":         []string{"Grunge", "Alternative Rock"},
		"catalog_number": "DGC-24425",
	})
	if resp.StatusCode != 201 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	album := decodeAlbum(t, resp)
	if album.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if album.Title != "Nevermind" {
		t.Errorf("Title = %q", album.Title)
	}
	if len(album.Artists) != 1 || album.Artists[0] != "Nirvana" {
		t.Errorf("Artists = %v", album.Artists)
	}
	if album.TrackCount != 12 {
		t.Errorf("TrackCount = %d, want 12", album.TrackCount)
	}
	if album.CatalogNumber != "DGC-24425" {
		t.Errorf("CatalogNumber = %q", album.CatalogNumber)
	}
}

func TestMusic_Create_missing_title(t *testing.T) {
	srv, _ := newMusicServer(t)
	resp := doJSON(t, "POST", srv.URL+"/api/music", map[string]any{"track_count": 10})
	if resp.StatusCode != 400 {
		t.Fatalf("missing title: status = %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestMusic_Create_invalid_json(t *testing.T) {
	srv, _ := newMusicServer(t)
	req, _ := http.NewRequest("POST", srv.URL+"/api/music", strings.NewReader("not json"))
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

func TestMusic_Get_success(t *testing.T) {
	srv, s := newMusicServer(t)
	album, _ := s.Create(store.MusicAlbumInput{Title: "Nevermind", Artists: []string{"Nirvana"}, TrackCount: 12})

	resp := doJSON(t, "GET", fmt.Sprintf("%s/api/music/%d", srv.URL, album.ID), nil)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	got := decodeAlbum(t, resp)
	if got.Title != "Nevermind" {
		t.Errorf("Title = %q", got.Title)
	}
	if got.TrackCount != 12 {
		t.Errorf("TrackCount = %d, want 12", got.TrackCount)
	}
}

func TestMusic_Get_not_found(t *testing.T) {
	srv, _ := newMusicServer(t)
	resp := doJSON(t, "GET", srv.URL+"/api/music/99999", nil)
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestMusic_Get_invalid_id(t *testing.T) {
	srv, _ := newMusicServer(t)
	resp := doJSON(t, "GET", srv.URL+"/api/music/notanid", nil)
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// --- Update ---

func TestMusic_Update_success(t *testing.T) {
	srv, s := newMusicServer(t)
	album, _ := s.Create(store.MusicAlbumInput{Title: "Old Title", TrackCount: 10})

	resp := doJSON(t, "PUT", fmt.Sprintf("%s/api/music/%d", srv.URL, album.ID), map[string]any{
		"title":       "Nevermind (Deluxe)",
		"track_count": 24,
	})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	got := decodeAlbum(t, resp)
	if got.Title != "Nevermind (Deluxe)" {
		t.Errorf("Title = %q", got.Title)
	}
	if got.TrackCount != 24 {
		t.Errorf("TrackCount = %d, want 24", got.TrackCount)
	}
}

func TestMusic_Update_not_found(t *testing.T) {
	srv, _ := newMusicServer(t)
	resp := doJSON(t, "PUT", srv.URL+"/api/music/99999", map[string]any{"title": "Ghost"})
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestMusic_Update_missing_title(t *testing.T) {
	srv, s := newMusicServer(t)
	album, _ := s.Create(store.MusicAlbumInput{Title: "Something"})

	resp := doJSON(t, "PUT", fmt.Sprintf("%s/api/music/%d", srv.URL, album.ID), map[string]any{})
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// --- Delete ---

func TestMusic_Delete_success(t *testing.T) {
	srv, s := newMusicServer(t)
	album, _ := s.Create(store.MusicAlbumInput{Title: "To Delete"})

	resp := doJSON(t, "DELETE", fmt.Sprintf("%s/api/music/%d", srv.URL, album.ID), nil)
	resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}

	_, err := s.Get(album.ID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Error("album should be gone after delete")
	}
}

func TestMusic_Delete_not_found(t *testing.T) {
	srv, _ := newMusicServer(t)
	resp := doJSON(t, "DELETE", srv.URL+"/api/music/99999", nil)
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// --- Content-Type ---

func TestMusic_JSON_content_type(t *testing.T) {
	srv, _ := newMusicServer(t)
	resp := doJSON(t, "GET", srv.URL+"/api/music", nil)
	defer resp.Body.Close()
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
