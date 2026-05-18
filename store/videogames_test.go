package store_test

import (
	"errors"
	"path/filepath"
	"testing"

	"swansea/db"
	"swansea/store"
)

func newGameStore(t *testing.T) *store.VideoGames {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return store.NewVideoGames(database)
}

var sampleGame = store.VideoGameInput{
	Title:       "The Legend of Zelda: Breath of the Wild",
	Platform:    "Nintendo Switch",
	Developers:  []string{"Nintendo EPD"},
	Publisher:   "Nintendo",
	ReleaseDate: "2017",
	Description: "Open world action adventure",
	Genres:      []string{"Action", "Adventure"},
	CoverURL:    "https://example.com/botw.jpg",
	Rating:      "E10+",
}

// --- CRUD ---

func TestGame_Create_and_Get(t *testing.T) {
	gs := newGameStore(t)
	game, err := gs.Create(sampleGame)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if game.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if game.Title != sampleGame.Title {
		t.Errorf("Title = %q, want %q", game.Title, sampleGame.Title)
	}
	if game.Platform != sampleGame.Platform {
		t.Errorf("Platform = %q, want %q", game.Platform, sampleGame.Platform)
	}
	if len(game.Developers) != 1 || game.Developers[0] != "Nintendo EPD" {
		t.Errorf("Developers = %v, want [Nintendo EPD]", game.Developers)
	}
	if len(game.Genres) != 2 {
		t.Errorf("Genres = %v, want 2 entries", game.Genres)
	}
	if game.CreatedAt == "" {
		t.Error("CreatedAt should be set")
	}

	got, err := gs.Get(game.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != game.Title {
		t.Errorf("Get Title = %q, want %q", got.Title, game.Title)
	}
}

func TestGame_Get_notFound(t *testing.T) {
	gs := newGameStore(t)
	_, err := gs.Get(99999)
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Get missing: got %v, want ErrNotFound", err)
	}
}

func TestGame_Update(t *testing.T) {
	gs := newGameStore(t)
	game, err := gs.Create(sampleGame)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	updated, err := gs.Update(game.ID, store.VideoGameInput{
		Title:    "Zelda: Tears of the Kingdom",
		Platform: "Nintendo Switch",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Title != "Zelda: Tears of the Kingdom" {
		t.Errorf("Updated title = %q", updated.Title)
	}
	if updated.Rating != "" {
		t.Errorf("Updated Rating should be empty, got %q", updated.Rating)
	}
}

func TestGame_Update_notFound(t *testing.T) {
	gs := newGameStore(t)
	_, err := gs.Update(99999, store.VideoGameInput{Title: "Ghost"})
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Update missing: got %v, want ErrNotFound", err)
	}
}

func TestGame_Delete(t *testing.T) {
	gs := newGameStore(t)
	game, err := gs.Create(sampleGame)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := gs.Delete(game.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = gs.Get(game.ID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Get after Delete: got %v, want ErrNotFound", err)
	}
}

func TestGame_Delete_notFound(t *testing.T) {
	gs := newGameStore(t)
	err := gs.Delete(99999)
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Delete missing: got %v, want ErrNotFound", err)
	}
}

func TestGame_UpdateCoverURL(t *testing.T) {
	gs := newGameStore(t)
	game, err := gs.Create(sampleGame)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := gs.UpdateCoverURL(game.ID, "/metadata/covers/botw.jpg"); err != nil {
		t.Fatalf("UpdateCoverURL: %v", err)
	}
	got, err := gs.Get(game.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.CoverURL != "/metadata/covers/botw.jpg" {
		t.Errorf("CoverURL = %q, want /metadata/covers/botw.jpg", got.CoverURL)
	}
}

// --- List / Pagination ---

func TestGame_List(t *testing.T) {
	gs := newGameStore(t)
	games, err := gs.List()
	if err != nil {
		t.Fatalf("List empty: %v", err)
	}
	if len(games) != 0 {
		t.Errorf("List empty: got %d games", len(games))
	}

	gs.Create(store.VideoGameInput{Title: "Game A", Platform: "PS5"})
	gs.Create(store.VideoGameInput{Title: "Game B", Platform: "Xbox"})

	games, err = gs.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(games) != 2 {
		t.Errorf("List: got %d games, want 2", len(games))
	}
	if games[0].Title != "Game A" {
		t.Errorf("first game = %q, want Game A", games[0].Title)
	}
}

func TestGame_Count(t *testing.T) {
	gs := newGameStore(t)
	n, err := gs.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 0 {
		t.Errorf("Count empty = %d", n)
	}
	gs.Create(store.VideoGameInput{Title: "X"})
	gs.Create(store.VideoGameInput{Title: "Y"})
	n, err = gs.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 2 {
		t.Errorf("Count = %d, want 2", n)
	}
}

func TestGame_ListPage(t *testing.T) {
	gs := newGameStore(t)
	for i := range 30 {
		gs.Create(store.VideoGameInput{Title: gameTitles[i%len(gameTitles)] + string(rune('A'+i))})
	}

	page1, err := gs.ListPage(store.PageSize, 0)
	if err != nil {
		t.Fatalf("ListPage p1: %v", err)
	}
	if len(page1) != store.PageSize {
		t.Errorf("page1 len = %d, want %d", len(page1), store.PageSize)
	}

	page2, err := gs.ListPage(store.PageSize, store.PageSize)
	if err != nil {
		t.Fatalf("ListPage p2: %v", err)
	}
	if len(page2) != 5 {
		t.Errorf("page2 len = %d, want 5", len(page2))
	}

	seen := map[int64]bool{}
	for _, g := range append(page1, page2...) {
		if seen[g.ID] {
			t.Errorf("duplicate game ID %d across pages", g.ID)
		}
		seen[g.ID] = true
	}
}

var gameTitles = []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon"}

// --- Filtering ---

func seedFilterGames(t *testing.T) *store.VideoGames {
	t.Helper()
	gs := newGameStore(t)
	games := []store.VideoGameInput{
		{Title: "A", Platform: "PS5", ReleaseDate: "2020", Genres: []string{"Action"}},
		{Title: "B", Platform: "PS5", ReleaseDate: "2021", Genres: []string{"RPG"}},
		{Title: "C", Platform: "Xbox", ReleaseDate: "2021", Genres: []string{"RPG"}},
		{Title: "D", Platform: "Xbox", ReleaseDate: "2022", Genres: []string{"Action"}},
	}
	for _, in := range games {
		if _, err := gs.Create(in); err != nil {
			t.Fatalf("seed create: %v", err)
		}
	}
	return gs
}

func TestGame_ListFiltered_genre(t *testing.T) {
	gs := seedFilterGames(t)
	games, err := gs.ListFiltered("genre", "RPG", 10, 0)
	if err != nil {
		t.Fatalf("ListFiltered genre: %v", err)
	}
	if len(games) != 2 {
		t.Errorf("RPG count = %d, want 2", len(games))
	}
}

func TestGame_ListFiltered_platform(t *testing.T) {
	gs := seedFilterGames(t)
	games, err := gs.ListFiltered("platform", "PS5", 10, 0)
	if err != nil {
		t.Fatalf("ListFiltered platform: %v", err)
	}
	if len(games) != 2 {
		t.Errorf("PS5 count = %d, want 2", len(games))
	}
}

func TestGame_ListFiltered_year(t *testing.T) {
	gs := seedFilterGames(t)
	games, err := gs.ListFiltered("year", "2021", 10, 0)
	if err != nil {
		t.Fatalf("ListFiltered year: %v", err)
	}
	if len(games) != 2 {
		t.Errorf("2021 count = %d, want 2", len(games))
	}
}

func TestGame_ListFiltered_unknown_falls_back_to_all(t *testing.T) {
	gs := seedFilterGames(t)
	games, err := gs.ListFiltered("unknown", "x", 10, 0)
	if err != nil {
		t.Fatalf("ListFiltered unknown: %v", err)
	}
	if len(games) != 4 {
		t.Errorf("fallback count = %d, want 4", len(games))
	}
}

func TestGame_ListFiltered_case_insensitive(t *testing.T) {
	gs := seedFilterGames(t)
	games, err := gs.ListFiltered("platform", "ps5", 10, 0)
	if err != nil {
		t.Fatalf("ListFiltered lowercase: %v", err)
	}
	if len(games) != 2 {
		t.Errorf("case-insensitive platform count = %d, want 2", len(games))
	}
}

func TestGame_CountFiltered(t *testing.T) {
	gs := seedFilterGames(t)

	n, err := gs.CountFiltered("genre", "Action")
	if err != nil {
		t.Fatalf("CountFiltered: %v", err)
	}
	if n != 2 {
		t.Errorf("CountFiltered Action = %d, want 2", n)
	}

	n, err = gs.CountFiltered("year", "2020")
	if err != nil {
		t.Fatalf("CountFiltered year: %v", err)
	}
	if n != 1 {
		t.Errorf("CountFiltered 2020 = %d, want 1", n)
	}
}

func TestGame_FilterValues(t *testing.T) {
	gs := seedFilterGames(t)
	fv, err := gs.FilterValues()
	if err != nil {
		t.Fatalf("FilterValues: %v", err)
	}
	if len(fv.Genres) != 2 {
		t.Errorf("Genres = %v, want 2", fv.Genres)
	}
	if len(fv.Platforms) != 2 {
		t.Errorf("Platforms = %v, want 2", fv.Platforms)
	}
	if len(fv.Years) != 3 {
		t.Errorf("Years = %v, want 3", fv.Years)
	}
	if fv.Years[0] != "2022" {
		t.Errorf("Years[0] = %q, want 2022", fv.Years[0])
	}
}

// --- FTS5 Search ---

func seedSearchGames(t *testing.T) *store.VideoGames {
	t.Helper()
	gs := newGameStore(t)
	gs.Create(store.VideoGameInput{Title: "Elden Ring", Developers: []string{"FromSoftware"}})
	gs.Create(store.VideoGameInput{Title: "Dark Souls", Developers: []string{"FromSoftware"}})
	gs.Create(store.VideoGameInput{Title: "Minecraft", Developers: []string{"Mojang"}})
	return gs
}

func TestGame_Search_title(t *testing.T) {
	gs := seedSearchGames(t)
	results, err := gs.Search("elden", "title", 10, 0)
	if err != nil {
		t.Fatalf("Search title: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("elden search = %d results, want 1", len(results))
	}
	if results[0].Title != "Elden Ring" {
		t.Errorf("result = %q, want Elden Ring", results[0].Title)
	}
}

func TestGame_Search_developer(t *testing.T) {
	gs := seedSearchGames(t)
	results, err := gs.Search("fromsoftware", "developer", 10, 0)
	if err != nil {
		t.Fatalf("Search developer: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("fromsoftware developer search = %d results, want 2", len(results))
	}
}

func TestGame_Search_all_fields(t *testing.T) {
	gs := seedSearchGames(t)
	results, err := gs.Search("souls", "", 10, 0)
	if err != nil {
		t.Fatalf("Search all: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("souls search = %d results, want 1", len(results))
	}
}

func TestGame_Search_no_results(t *testing.T) {
	gs := seedSearchGames(t)
	results, err := gs.Search("zzznomatch", "", 10, 0)
	if err != nil {
		t.Fatalf("Search no results: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("no-match search = %d results, want 0", len(results))
	}
}

func TestGame_SearchCount(t *testing.T) {
	gs := seedSearchGames(t)
	n, err := gs.SearchCount("fromsoftware", "developer")
	if err != nil {
		t.Fatalf("SearchCount: %v", err)
	}
	if n != 2 {
		t.Errorf("SearchCount fromsoftware developer = %d, want 2", n)
	}
}

func TestGame_Search_updates_on_create(t *testing.T) {
	gs := newGameStore(t)
	gs.Create(store.VideoGameInput{Title: "New Game", Developers: []string{"New Studio"}})
	results, err := gs.Search("new", "title", 10, 0)
	if err != nil {
		t.Fatalf("Search after create: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("search after create = %d results, want 1", len(results))
	}
}

func TestGame_Search_updates_on_delete(t *testing.T) {
	gs := newGameStore(t)
	game, _ := gs.Create(store.VideoGameInput{Title: "Temporary Game"})
	gs.Delete(game.ID)
	results, err := gs.Search("Temporary", "title", 10, 0)
	if err != nil {
		t.Fatalf("Search after delete: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("search after delete = %d results, want 0", len(results))
	}
}

func TestGame_NilSlices_round_trip(t *testing.T) {
	gs := newGameStore(t)
	game, err := gs.Create(store.VideoGameInput{Title: "No Developers"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := gs.Get(game.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	_ = got.Developers
	_ = got.Genres
}
