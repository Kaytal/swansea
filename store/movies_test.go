package store_test

import (
	"errors"
	"path/filepath"
	"testing"

	"swansea/db"
	"swansea/store"
)

func newMovieStore(t *testing.T) *store.Movies {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return store.NewMovies(database)
}

var sampleMovie = store.MovieInput{
	Title:       "Inception",
	Directors:   []string{"Christopher Nolan"},
	Studio:      "Warner Bros.",
	ReleaseDate: "2010",
	Description: "A thief who steals corporate secrets through dreams",
	Runtime:     148,
	Genres:      []string{"Sci-Fi", "Thriller"},
	CoverURL:    "https://example.com/inception.jpg",
	TmdbID:      27205,
}

// --- CRUD ---

func TestMovie_Create_and_Get(t *testing.T) {
	ms := newMovieStore(t)
	movie, err := ms.Create(sampleMovie)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if movie.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if movie.Title != sampleMovie.Title {
		t.Errorf("Title = %q, want %q", movie.Title, sampleMovie.Title)
	}
	if len(movie.Directors) != 1 || movie.Directors[0] != "Christopher Nolan" {
		t.Errorf("Directors = %v, want [Christopher Nolan]", movie.Directors)
	}
	if movie.Runtime != 148 {
		t.Errorf("Runtime = %d, want 148", movie.Runtime)
	}
	if len(movie.Genres) != 2 {
		t.Errorf("Genres = %v, want 2 entries", movie.Genres)
	}
	if movie.TmdbID != 27205 {
		t.Errorf("TmdbID = %d, want 27205", movie.TmdbID)
	}
	if movie.CreatedAt == "" {
		t.Error("CreatedAt should be set")
	}

	got, err := ms.Get(movie.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != movie.Title {
		t.Errorf("Get Title = %q, want %q", got.Title, movie.Title)
	}
}

func TestMovie_Get_notFound(t *testing.T) {
	ms := newMovieStore(t)
	_, err := ms.Get(99999)
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Get missing: got %v, want ErrNotFound", err)
	}
}

func TestMovie_Update(t *testing.T) {
	ms := newMovieStore(t)
	movie, err := ms.Create(sampleMovie)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	updated, err := ms.Update(movie.ID, store.MovieInput{
		Title:     "Inception (Director's Cut)",
		Directors: []string{"Christopher Nolan"},
		Runtime:   160,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Title != "Inception (Director's Cut)" {
		t.Errorf("Updated title = %q", updated.Title)
	}
	if updated.Runtime != 160 {
		t.Errorf("Updated Runtime = %d, want 160", updated.Runtime)
	}
	if updated.Studio != "" {
		t.Errorf("Updated Studio should be empty, got %q", updated.Studio)
	}
}

func TestMovie_Update_notFound(t *testing.T) {
	ms := newMovieStore(t)
	_, err := ms.Update(99999, store.MovieInput{Title: "Ghost"})
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Update missing: got %v, want ErrNotFound", err)
	}
}

func TestMovie_Delete(t *testing.T) {
	ms := newMovieStore(t)
	movie, err := ms.Create(sampleMovie)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := ms.Delete(movie.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = ms.Get(movie.ID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Get after Delete: got %v, want ErrNotFound", err)
	}
}

func TestMovie_Delete_notFound(t *testing.T) {
	ms := newMovieStore(t)
	err := ms.Delete(99999)
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Delete missing: got %v, want ErrNotFound", err)
	}
}

func TestMovie_UpdateCoverURL(t *testing.T) {
	ms := newMovieStore(t)
	movie, err := ms.Create(sampleMovie)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := ms.UpdateCoverURL(movie.ID, "/metadata/covers/inception.jpg"); err != nil {
		t.Fatalf("UpdateCoverURL: %v", err)
	}
	got, err := ms.Get(movie.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.CoverURL != "/metadata/covers/inception.jpg" {
		t.Errorf("CoverURL = %q, want /metadata/covers/inception.jpg", got.CoverURL)
	}
}

// --- List / Pagination ---

func TestMovie_List(t *testing.T) {
	ms := newMovieStore(t)
	movies, err := ms.List()
	if err != nil {
		t.Fatalf("List empty: %v", err)
	}
	if len(movies) != 0 {
		t.Errorf("List empty: got %d movies", len(movies))
	}

	ms.Create(store.MovieInput{Title: "Movie A"})
	ms.Create(store.MovieInput{Title: "Movie B"})

	movies, err = ms.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(movies) != 2 {
		t.Errorf("List: got %d movies, want 2", len(movies))
	}
	if movies[0].Title != "Movie A" {
		t.Errorf("first movie = %q, want Movie A", movies[0].Title)
	}
}

func TestMovie_Count(t *testing.T) {
	ms := newMovieStore(t)
	n, err := ms.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 0 {
		t.Errorf("Count empty = %d", n)
	}
	ms.Create(store.MovieInput{Title: "X"})
	ms.Create(store.MovieInput{Title: "Y"})
	n, err = ms.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 2 {
		t.Errorf("Count = %d, want 2", n)
	}
}

func TestMovie_ListPage(t *testing.T) {
	ms := newMovieStore(t)
	for i := range 30 {
		ms.Create(store.MovieInput{Title: movieTitles[i%len(movieTitles)] + string(rune('A'+i))})
	}

	page1, err := ms.ListPage(store.PageSize, 0)
	if err != nil {
		t.Fatalf("ListPage p1: %v", err)
	}
	if len(page1) != store.PageSize {
		t.Errorf("page1 len = %d, want %d", len(page1), store.PageSize)
	}

	page2, err := ms.ListPage(store.PageSize, store.PageSize)
	if err != nil {
		t.Fatalf("ListPage p2: %v", err)
	}
	if len(page2) != 5 {
		t.Errorf("page2 len = %d, want 5", len(page2))
	}

	seen := map[int64]bool{}
	for _, m := range append(page1, page2...) {
		if seen[m.ID] {
			t.Errorf("duplicate movie ID %d across pages", m.ID)
		}
		seen[m.ID] = true
	}
}

var movieTitles = []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon"}

// --- Filtering ---

func seedFilterMovies(t *testing.T) *store.Movies {
	t.Helper()
	ms := newMovieStore(t)
	movies := []store.MovieInput{
		{Title: "A", Directors: []string{"Nolan"}, ReleaseDate: "2010", Genres: []string{"Thriller"}},
		{Title: "B", Directors: []string{"Nolan"}, ReleaseDate: "2012", Genres: []string{"Sci-Fi"}},
		{Title: "C", Directors: []string{"Spielberg"}, ReleaseDate: "2012", Genres: []string{"Sci-Fi"}},
		{Title: "D", Directors: []string{"Spielberg"}, ReleaseDate: "2015", Genres: []string{"Thriller"}},
	}
	for _, in := range movies {
		if _, err := ms.Create(in); err != nil {
			t.Fatalf("seed create: %v", err)
		}
	}
	return ms
}

func TestMovie_ListFiltered_genre(t *testing.T) {
	ms := seedFilterMovies(t)
	movies, err := ms.ListFiltered("genre", "Sci-Fi", 10, 0)
	if err != nil {
		t.Fatalf("ListFiltered genre: %v", err)
	}
	if len(movies) != 2 {
		t.Errorf("Sci-Fi count = %d, want 2", len(movies))
	}
}

func TestMovie_ListFiltered_director(t *testing.T) {
	ms := seedFilterMovies(t)
	movies, err := ms.ListFiltered("director", "Spielberg", 10, 0)
	if err != nil {
		t.Fatalf("ListFiltered director: %v", err)
	}
	if len(movies) != 2 {
		t.Errorf("Spielberg count = %d, want 2", len(movies))
	}
}

func TestMovie_ListFiltered_year(t *testing.T) {
	ms := seedFilterMovies(t)
	movies, err := ms.ListFiltered("year", "2012", 10, 0)
	if err != nil {
		t.Fatalf("ListFiltered year: %v", err)
	}
	if len(movies) != 2 {
		t.Errorf("2012 count = %d, want 2", len(movies))
	}
}

func TestMovie_ListFiltered_unknown_falls_back_to_all(t *testing.T) {
	ms := seedFilterMovies(t)
	movies, err := ms.ListFiltered("unknown", "x", 10, 0)
	if err != nil {
		t.Fatalf("ListFiltered unknown: %v", err)
	}
	if len(movies) != 4 {
		t.Errorf("fallback count = %d, want 4", len(movies))
	}
}

func TestMovie_ListFiltered_case_insensitive(t *testing.T) {
	ms := seedFilterMovies(t)
	movies, err := ms.ListFiltered("genre", "sci-fi", 10, 0)
	if err != nil {
		t.Fatalf("ListFiltered lowercase: %v", err)
	}
	if len(movies) != 2 {
		t.Errorf("case-insensitive count = %d, want 2", len(movies))
	}
}

func TestMovie_CountFiltered(t *testing.T) {
	ms := seedFilterMovies(t)

	n, err := ms.CountFiltered("genre", "Thriller")
	if err != nil {
		t.Fatalf("CountFiltered: %v", err)
	}
	if n != 2 {
		t.Errorf("CountFiltered Thriller = %d, want 2", n)
	}

	n, err = ms.CountFiltered("year", "2010")
	if err != nil {
		t.Fatalf("CountFiltered year: %v", err)
	}
	if n != 1 {
		t.Errorf("CountFiltered 2010 = %d, want 1", n)
	}
}

func TestMovie_FilterValues(t *testing.T) {
	ms := seedFilterMovies(t)
	fv, err := ms.FilterValues()
	if err != nil {
		t.Fatalf("FilterValues: %v", err)
	}
	if len(fv.Genres) != 2 {
		t.Errorf("Genres = %v, want 2", fv.Genres)
	}
	if len(fv.Directors) != 2 {
		t.Errorf("Directors = %v, want 2", fv.Directors)
	}
	if len(fv.Years) != 3 {
		t.Errorf("Years = %v, want 3", fv.Years)
	}
	if fv.Years[0] != "2015" {
		t.Errorf("Years[0] = %q, want 2015", fv.Years[0])
	}
}

// --- FTS5 Search ---

func seedSearchMovies(t *testing.T) *store.Movies {
	t.Helper()
	ms := newMovieStore(t)
	ms.Create(store.MovieInput{Title: "The Dark Knight", Directors: []string{"Christopher Nolan"}})
	ms.Create(store.MovieInput{Title: "Interstellar", Directors: []string{"Christopher Nolan"}})
	ms.Create(store.MovieInput{Title: "Jaws", Directors: []string{"Steven Spielberg"}})
	return ms
}

func TestMovie_Search_title(t *testing.T) {
	ms := seedSearchMovies(t)
	results, err := ms.Search("knight", "title", 10, 0)
	if err != nil {
		t.Fatalf("Search title: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("knight search = %d results, want 1", len(results))
	}
	if results[0].Title != "The Dark Knight" {
		t.Errorf("result = %q, want The Dark Knight", results[0].Title)
	}
}

func TestMovie_Search_director(t *testing.T) {
	ms := seedSearchMovies(t)
	results, err := ms.Search("nolan", "director", 10, 0)
	if err != nil {
		t.Fatalf("Search director: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("nolan director search = %d results, want 2", len(results))
	}
}

func TestMovie_Search_all_fields(t *testing.T) {
	ms := seedSearchMovies(t)
	results, err := ms.Search("interstellar", "", 10, 0)
	if err != nil {
		t.Fatalf("Search all: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("interstellar search = %d results, want 1", len(results))
	}
}

func TestMovie_Search_no_results(t *testing.T) {
	ms := seedSearchMovies(t)
	results, err := ms.Search("zzznomatch", "", 10, 0)
	if err != nil {
		t.Fatalf("Search no results: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("no-match search = %d results, want 0", len(results))
	}
}

func TestMovie_SearchCount(t *testing.T) {
	ms := seedSearchMovies(t)
	n, err := ms.SearchCount("nolan", "director")
	if err != nil {
		t.Fatalf("SearchCount: %v", err)
	}
	if n != 2 {
		t.Errorf("SearchCount nolan director = %d, want 2", n)
	}
}

func TestMovie_Search_updates_on_delete(t *testing.T) {
	ms := newMovieStore(t)
	movie, _ := ms.Create(store.MovieInput{Title: "Temporary Movie"})
	ms.Delete(movie.ID)
	results, err := ms.Search("Temporary", "title", 10, 0)
	if err != nil {
		t.Fatalf("Search after delete: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("search after delete = %d results, want 0", len(results))
	}
}

func TestMovie_NilSlices_round_trip(t *testing.T) {
	ms := newMovieStore(t)
	movie, err := ms.Create(store.MovieInput{Title: "No Directors"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := ms.Get(movie.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	_ = got.Directors
	_ = got.Genres
}
