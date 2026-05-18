package store_test

import (
	"errors"
	"path/filepath"
	"testing"

	"swansea/db"
	"swansea/store"
)

func newMusicStore(t *testing.T) *store.MusicAlbums {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return store.NewMusicAlbums(database)
}

var sampleAlbum = store.MusicAlbumInput{
	Title:         "Nevermind",
	Artists:       []string{"Nirvana"},
	Label:         "DGC Records",
	ReleaseDate:   "1991",
	Description:   "Second studio album by Nirvana",
	Genres:        []string{"Grunge", "Alternative Rock"},
	TrackCount:    12,
	CoverURL:      "https://example.com/nevermind.jpg",
	CatalogNumber: "DGC-24425",
}

// --- CRUD ---

func TestMusic_Create_and_Get(t *testing.T) {
	as := newMusicStore(t)
	album, err := as.Create(sampleAlbum)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if album.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if album.Title != sampleAlbum.Title {
		t.Errorf("Title = %q, want %q", album.Title, sampleAlbum.Title)
	}
	if len(album.Artists) != 1 || album.Artists[0] != "Nirvana" {
		t.Errorf("Artists = %v, want [Nirvana]", album.Artists)
	}
	if album.TrackCount != 12 {
		t.Errorf("TrackCount = %d, want 12", album.TrackCount)
	}
	if len(album.Genres) != 2 {
		t.Errorf("Genres = %v, want 2 entries", album.Genres)
	}
	if album.CatalogNumber != "DGC-24425" {
		t.Errorf("CatalogNumber = %q, want DGC-24425", album.CatalogNumber)
	}
	if album.CreatedAt == "" {
		t.Error("CreatedAt should be set")
	}

	got, err := as.Get(album.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != album.Title {
		t.Errorf("Get Title = %q, want %q", got.Title, album.Title)
	}
}

func TestMusic_Get_notFound(t *testing.T) {
	as := newMusicStore(t)
	_, err := as.Get(99999)
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Get missing: got %v, want ErrNotFound", err)
	}
}

func TestMusic_Update(t *testing.T) {
	as := newMusicStore(t)
	album, err := as.Create(sampleAlbum)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	updated, err := as.Update(album.ID, store.MusicAlbumInput{
		Title:      "Nevermind (Deluxe Edition)",
		Artists:    []string{"Nirvana"},
		TrackCount: 24,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Title != "Nevermind (Deluxe Edition)" {
		t.Errorf("Updated title = %q", updated.Title)
	}
	if updated.TrackCount != 24 {
		t.Errorf("Updated TrackCount = %d, want 24", updated.TrackCount)
	}
	if updated.Label != "" {
		t.Errorf("Updated Label should be empty, got %q", updated.Label)
	}
}

func TestMusic_Update_notFound(t *testing.T) {
	as := newMusicStore(t)
	_, err := as.Update(99999, store.MusicAlbumInput{Title: "Ghost"})
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Update missing: got %v, want ErrNotFound", err)
	}
}

func TestMusic_Delete(t *testing.T) {
	as := newMusicStore(t)
	album, err := as.Create(sampleAlbum)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := as.Delete(album.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = as.Get(album.ID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Get after Delete: got %v, want ErrNotFound", err)
	}
}

func TestMusic_Delete_notFound(t *testing.T) {
	as := newMusicStore(t)
	err := as.Delete(99999)
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Delete missing: got %v, want ErrNotFound", err)
	}
}

func TestMusic_UpdateCoverURL(t *testing.T) {
	as := newMusicStore(t)
	album, err := as.Create(sampleAlbum)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := as.UpdateCoverURL(album.ID, "/metadata/covers/nevermind.jpg"); err != nil {
		t.Fatalf("UpdateCoverURL: %v", err)
	}
	got, err := as.Get(album.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.CoverURL != "/metadata/covers/nevermind.jpg" {
		t.Errorf("CoverURL = %q, want /metadata/covers/nevermind.jpg", got.CoverURL)
	}
}

// --- List / Pagination ---

func TestMusic_List(t *testing.T) {
	as := newMusicStore(t)
	albums, err := as.List()
	if err != nil {
		t.Fatalf("List empty: %v", err)
	}
	if len(albums) != 0 {
		t.Errorf("List empty: got %d albums", len(albums))
	}

	as.Create(store.MusicAlbumInput{Title: "Album A"})
	as.Create(store.MusicAlbumInput{Title: "Album B"})

	albums, err = as.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(albums) != 2 {
		t.Errorf("List: got %d albums, want 2", len(albums))
	}
	if albums[0].Title != "Album A" {
		t.Errorf("first album = %q, want Album A", albums[0].Title)
	}
}

func TestMusic_Count(t *testing.T) {
	as := newMusicStore(t)
	n, err := as.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 0 {
		t.Errorf("Count empty = %d", n)
	}
	as.Create(store.MusicAlbumInput{Title: "X"})
	as.Create(store.MusicAlbumInput{Title: "Y"})
	n, err = as.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 2 {
		t.Errorf("Count = %d, want 2", n)
	}
}

func TestMusic_ListPage(t *testing.T) {
	as := newMusicStore(t)
	for i := range 30 {
		as.Create(store.MusicAlbumInput{Title: albumTitles[i%len(albumTitles)] + string(rune('A'+i))})
	}

	page1, err := as.ListPage(store.PageSize, 0)
	if err != nil {
		t.Fatalf("ListPage p1: %v", err)
	}
	if len(page1) != store.PageSize {
		t.Errorf("page1 len = %d, want %d", len(page1), store.PageSize)
	}

	page2, err := as.ListPage(store.PageSize, store.PageSize)
	if err != nil {
		t.Fatalf("ListPage p2: %v", err)
	}
	if len(page2) != 5 {
		t.Errorf("page2 len = %d, want 5", len(page2))
	}

	seen := map[int64]bool{}
	for _, a := range append(page1, page2...) {
		if seen[a.ID] {
			t.Errorf("duplicate album ID %d across pages", a.ID)
		}
		seen[a.ID] = true
	}
}

var albumTitles = []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon"}

// --- Filtering ---

func seedFilterAlbums(t *testing.T) *store.MusicAlbums {
	t.Helper()
	as := newMusicStore(t)
	albums := []store.MusicAlbumInput{
		{Title: "A", Artists: []string{"Nirvana"}, ReleaseDate: "1991", Genres: []string{"Grunge"}},
		{Title: "B", Artists: []string{"Nirvana"}, ReleaseDate: "1993", Genres: []string{"Alternative"}},
		{Title: "C", Artists: []string{"Pearl Jam"}, ReleaseDate: "1993", Genres: []string{"Alternative"}},
		{Title: "D", Artists: []string{"Pearl Jam"}, ReleaseDate: "1994", Genres: []string{"Grunge"}},
	}
	for _, in := range albums {
		if _, err := as.Create(in); err != nil {
			t.Fatalf("seed create: %v", err)
		}
	}
	return as
}

func TestMusic_ListFiltered_genre(t *testing.T) {
	as := seedFilterAlbums(t)
	albums, err := as.ListFiltered("genre", "Grunge", 10, 0)
	if err != nil {
		t.Fatalf("ListFiltered genre: %v", err)
	}
	if len(albums) != 2 {
		t.Errorf("Grunge count = %d, want 2", len(albums))
	}
}

func TestMusic_ListFiltered_artist(t *testing.T) {
	as := seedFilterAlbums(t)
	albums, err := as.ListFiltered("artist", "Nirvana", 10, 0)
	if err != nil {
		t.Fatalf("ListFiltered artist: %v", err)
	}
	if len(albums) != 2 {
		t.Errorf("Nirvana count = %d, want 2", len(albums))
	}
}

func TestMusic_ListFiltered_year(t *testing.T) {
	as := seedFilterAlbums(t)
	albums, err := as.ListFiltered("year", "1993", 10, 0)
	if err != nil {
		t.Fatalf("ListFiltered year: %v", err)
	}
	if len(albums) != 2 {
		t.Errorf("1993 count = %d, want 2", len(albums))
	}
}

func TestMusic_ListFiltered_unknown_falls_back_to_all(t *testing.T) {
	as := seedFilterAlbums(t)
	albums, err := as.ListFiltered("unknown", "x", 10, 0)
	if err != nil {
		t.Fatalf("ListFiltered unknown: %v", err)
	}
	if len(albums) != 4 {
		t.Errorf("fallback count = %d, want 4", len(albums))
	}
}

func TestMusic_ListFiltered_case_insensitive(t *testing.T) {
	as := seedFilterAlbums(t)
	albums, err := as.ListFiltered("artist", "nirvana", 10, 0)
	if err != nil {
		t.Fatalf("ListFiltered lowercase: %v", err)
	}
	if len(albums) != 2 {
		t.Errorf("case-insensitive artist count = %d, want 2", len(albums))
	}
}

func TestMusic_CountFiltered(t *testing.T) {
	as := seedFilterAlbums(t)

	n, err := as.CountFiltered("genre", "Grunge")
	if err != nil {
		t.Fatalf("CountFiltered: %v", err)
	}
	if n != 2 {
		t.Errorf("CountFiltered Grunge = %d, want 2", n)
	}

	n, err = as.CountFiltered("year", "1991")
	if err != nil {
		t.Fatalf("CountFiltered year: %v", err)
	}
	if n != 1 {
		t.Errorf("CountFiltered 1991 = %d, want 1", n)
	}
}

func TestMusic_FilterValues(t *testing.T) {
	as := seedFilterAlbums(t)
	fv, err := as.FilterValues()
	if err != nil {
		t.Fatalf("FilterValues: %v", err)
	}
	if len(fv.Genres) != 2 {
		t.Errorf("Genres = %v, want 2", fv.Genres)
	}
	if len(fv.Artists) != 2 {
		t.Errorf("Artists = %v, want 2", fv.Artists)
	}
	if len(fv.Years) != 3 {
		t.Errorf("Years = %v, want 3", fv.Years)
	}
	if fv.Years[0] != "1994" {
		t.Errorf("Years[0] = %q, want 1994", fv.Years[0])
	}
}

// --- FTS5 Search ---

func seedSearchAlbums(t *testing.T) *store.MusicAlbums {
	t.Helper()
	as := newMusicStore(t)
	as.Create(store.MusicAlbumInput{Title: "Nevermind", Artists: []string{"Nirvana"}})
	as.Create(store.MusicAlbumInput{Title: "In Utero", Artists: []string{"Nirvana"}})
	as.Create(store.MusicAlbumInput{Title: "Ten", Artists: []string{"Pearl Jam"}})
	return as
}

func TestMusic_Search_title(t *testing.T) {
	as := seedSearchAlbums(t)
	results, err := as.Search("nevermind", "title", 10, 0)
	if err != nil {
		t.Fatalf("Search title: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("nevermind search = %d results, want 1", len(results))
	}
	if results[0].Title != "Nevermind" {
		t.Errorf("result = %q, want Nevermind", results[0].Title)
	}
}

func TestMusic_Search_artist(t *testing.T) {
	as := seedSearchAlbums(t)
	results, err := as.Search("nirvana", "artist", 10, 0)
	if err != nil {
		t.Fatalf("Search artist: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("nirvana artist search = %d results, want 2", len(results))
	}
}

func TestMusic_Search_all_fields(t *testing.T) {
	as := seedSearchAlbums(t)
	results, err := as.Search("utero", "", 10, 0)
	if err != nil {
		t.Fatalf("Search all: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("utero search = %d results, want 1", len(results))
	}
}

func TestMusic_Search_no_results(t *testing.T) {
	as := seedSearchAlbums(t)
	results, err := as.Search("zzznomatch", "", 10, 0)
	if err != nil {
		t.Fatalf("Search no results: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("no-match search = %d results, want 0", len(results))
	}
}

func TestMusic_SearchCount(t *testing.T) {
	as := seedSearchAlbums(t)
	n, err := as.SearchCount("nirvana", "artist")
	if err != nil {
		t.Fatalf("SearchCount: %v", err)
	}
	if n != 2 {
		t.Errorf("SearchCount nirvana artist = %d, want 2", n)
	}
}

func TestMusic_Search_updates_on_delete(t *testing.T) {
	as := newMusicStore(t)
	album, _ := as.Create(store.MusicAlbumInput{Title: "Temporary Album"})
	as.Delete(album.ID)
	results, err := as.Search("Temporary", "title", 10, 0)
	if err != nil {
		t.Fatalf("Search after delete: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("search after delete = %d results, want 0", len(results))
	}
}

func TestMusic_NilSlices_round_trip(t *testing.T) {
	as := newMusicStore(t)
	album, err := as.Create(store.MusicAlbumInput{Title: "No Artists"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := as.Get(album.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	_ = got.Artists
	_ = got.Genres
}
