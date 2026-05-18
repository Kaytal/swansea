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

func newMusicUIServer(t *testing.T) (*httptest.Server, *store.MusicAlbums) {
	t.Helper()
	assetsFS := os.DirFS(repoRoot())

	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	s := store.NewMusicAlbums(database)
	ui, err := handlers.NewMusicUI(s, assetsFS, nil)
	if err != nil {
		t.Fatalf("NewMusicUI: %v", err)
	}

	mux := http.NewServeMux()
	ui.Register(mux)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, s
}

func TestMusicUI_page(t *testing.T) {
	srv, _ := newMusicUIServer(t)
	resp, err := http.Get(srv.URL + "/music")
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

func TestMusicUI_list_empty(t *testing.T) {
	srv, _ := newMusicUIServer(t)
	resp, err := http.Get(srv.URL + "/ui/music")
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

func TestMusicUI_list_shows_albums(t *testing.T) {
	srv, s := newMusicUIServer(t)
	s.Create(store.MusicAlbumInput{Title: "Nevermind", Artists: []string{"Nirvana"}})

	resp, err := http.Get(srv.URL + "/ui/music")
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if !strings.Contains(body, "Nevermind") {
		t.Error("list should show album title")
	}
	if !strings.Contains(body, "Nirvana") {
		t.Error("list should show artist")
	}
}

func TestMusicUI_create_success(t *testing.T) {
	srv, _ := newMusicUIServer(t)
	form := url.Values{"title": {"Dark Side of the Moon"}, "artists": {"Pink Floyd"}}
	resp, err := http.PostForm(srv.URL+"/ui/music", form)
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
	if !strings.Contains(body, "Dark Side of the Moon") {
		t.Error("response should contain new album")
	}
}

func TestMusicUI_create_missing_title(t *testing.T) {
	srv, _ := newMusicUIServer(t)
	form := url.Values{"artists": {"Someone"}}
	resp, err := http.PostForm(srv.URL+"/ui/music", form)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("missing title: status = %d, want 400", resp.StatusCode)
	}
}

func TestMusicUI_editForm(t *testing.T) {
	srv, s := newMusicUIServer(t)
	album, _ := s.Create(store.MusicAlbumInput{Title: "Abbey Road"})

	resp, err := http.Get(srv.URL + "/ui/music/" + itoa(album.ID) + "/edit")
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !strings.Contains(body, "Abbey Road") {
		t.Error("edit form should contain album title")
	}
}

func TestMusicUI_update_success(t *testing.T) {
	srv, s := newMusicUIServer(t)
	album, _ := s.Create(store.MusicAlbumInput{Title: "Old Title"})

	form := url.Values{"title": {"New Title"}, "track_count": {"10"}}
	req, _ := http.NewRequest("PUT", srv.URL+"/ui/music/"+itoa(album.ID), strings.NewReader(form.Encode()))
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

func TestMusicUI_update_missing_title(t *testing.T) {
	srv, s := newMusicUIServer(t)
	album, _ := s.Create(store.MusicAlbumInput{Title: "Something"})

	form := url.Values{"track_count": {"5"}}
	req, _ := http.NewRequest("PUT", srv.URL+"/ui/music/"+itoa(album.ID), strings.NewReader(form.Encode()))
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

func TestMusicUI_delete(t *testing.T) {
	srv, s := newMusicUIServer(t)
	album, _ := s.Create(store.MusicAlbumInput{Title: "To Delete"})

	req, _ := http.NewRequest("DELETE", srv.URL+"/ui/music/"+itoa(album.ID), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestMusicUI_filters(t *testing.T) {
	srv, s := newMusicUIServer(t)
	s.Create(store.MusicAlbumInput{Title: "Thriller", Artists: []string{"Michael Jackson"}, Genres: []string{"Pop"}})

	resp, err := http.Get(srv.URL + "/ui/music/filters")
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !strings.Contains(body, "Michael Jackson") {
		t.Error("filters should list artist")
	}
	if !strings.Contains(body, "Pop") {
		t.Error("filters should list genre")
	}
}

func TestMusicUI_search(t *testing.T) {
	srv, s := newMusicUIServer(t)
	s.Create(store.MusicAlbumInput{Title: "Nevermind"})
	s.Create(store.MusicAlbumInput{Title: "OK Computer"})

	resp, err := http.Get(srv.URL + "/ui/music/search?q=never")
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if !strings.Contains(body, "Nevermind") {
		t.Error("search should return matching album")
	}
	if strings.Contains(body, "OK Computer") {
		t.Error("search should not return non-matching album")
	}
}

func TestMusicUI_addForm(t *testing.T) {
	srv, _ := newMusicUIServer(t)
	resp, err := http.Get(srv.URL + "/ui/music/add")
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !strings.Contains(body, "Add Album") {
		t.Error("add form should say 'Add Album'")
	}
}
