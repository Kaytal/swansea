package store_test

import (
	"errors"
	"path/filepath"
	"testing"

	"swansea/db"
	"swansea/store"
)

// newStore returns a Books store backed by a fresh temp DB (seed book included).
func newStore(t *testing.T) *store.Books {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return store.NewBooks(database)
}

// newCleanStore returns a Books store with all seed data removed.
func newCleanStore(t *testing.T) *store.Books {
	t.Helper()
	bs := newStore(t)
	books, err := bs.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, b := range books {
		if err := bs.Delete(b.ID); err != nil {
			t.Fatalf("Delete seed: %v", err)
		}
	}
	return bs
}

var sampleInput = store.BookInput{
	ISBN:          "9780141036144",
	Title:         "1984",
	Authors:       []string{"George Orwell"},
	Publisher:     "Penguin",
	PublishedDate: "1949",
	Description:   "Dystopian novel",
	PageCount:     328,
	CoverURL:      "https://example.com/cover.jpg",
	Categories:    []string{"Fiction", "Dystopia"},
}

// --- CRUD ---

func TestCreate_and_Get(t *testing.T) {
	bs := newCleanStore(t)
	book, err := bs.Create(sampleInput)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if book.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if book.Title != sampleInput.Title {
		t.Errorf("Title = %q, want %q", book.Title, sampleInput.Title)
	}
	if book.ISBN != sampleInput.ISBN {
		t.Errorf("ISBN = %q, want %q", book.ISBN, sampleInput.ISBN)
	}
	if len(book.Authors) != 1 || book.Authors[0] != "George Orwell" {
		t.Errorf("Authors = %v, want [George Orwell]", book.Authors)
	}
	if len(book.Categories) != 2 {
		t.Errorf("Categories = %v, want 2 entries", book.Categories)
	}
	if book.CreatedAt == "" {
		t.Error("CreatedAt should be set")
	}

	got, err := bs.Get(book.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != book.Title {
		t.Errorf("Get Title = %q, want %q", got.Title, book.Title)
	}
}

func TestGet_notFound(t *testing.T) {
	bs := newCleanStore(t)
	_, err := bs.Get(99999)
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Get missing: got %v, want ErrNotFound", err)
	}
}

func TestGetByISBN(t *testing.T) {
	bs := newCleanStore(t)
	created, err := bs.Create(sampleInput)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := bs.GetByISBN(sampleInput.ISBN)
	if err != nil {
		t.Fatalf("GetByISBN: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("GetByISBN ID = %d, want %d", got.ID, created.ID)
	}
}

func TestGetByISBN_notFound(t *testing.T) {
	bs := newCleanStore(t)
	_, err := bs.GetByISBN("0000000000000")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetByISBN missing: got %v, want ErrNotFound", err)
	}
}

func TestUpdate(t *testing.T) {
	bs := newCleanStore(t)
	book, err := bs.Create(sampleInput)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	updated, err := bs.Update(book.ID, store.BookInput{
		Title:   "Nineteen Eighty-Four",
		Authors: []string{"George Orwell"},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Title != "Nineteen Eighty-Four" {
		t.Errorf("Updated title = %q", updated.Title)
	}
	if updated.ISBN != "" {
		t.Errorf("Updated ISBN should be empty, got %q", updated.ISBN)
	}
}

func TestUpdate_notFound(t *testing.T) {
	bs := newCleanStore(t)
	_, err := bs.Update(99999, store.BookInput{Title: "Ghost"})
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Update missing: got %v, want ErrNotFound", err)
	}
}

func TestDelete(t *testing.T) {
	bs := newCleanStore(t)
	book, err := bs.Create(sampleInput)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := bs.Delete(book.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = bs.Get(book.ID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Get after Delete: got %v, want ErrNotFound", err)
	}
}

func TestDelete_notFound(t *testing.T) {
	bs := newCleanStore(t)
	err := bs.Delete(99999)
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Delete missing: got %v, want ErrNotFound", err)
	}
}

func TestUpdateCoverURL(t *testing.T) {
	bs := newCleanStore(t)
	book, err := bs.Create(sampleInput)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := bs.UpdateCoverURL(book.ID, "/metadata/covers/test.jpg"); err != nil {
		t.Fatalf("UpdateCoverURL: %v", err)
	}
	got, err := bs.Get(book.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.CoverURL != "/metadata/covers/test.jpg" {
		t.Errorf("CoverURL = %q, want /metadata/covers/test.jpg", got.CoverURL)
	}
}

// --- List / Pagination ---

func TestList(t *testing.T) {
	bs := newCleanStore(t)
	books, err := bs.List()
	if err != nil {
		t.Fatalf("List empty: %v", err)
	}
	if len(books) != 0 {
		t.Errorf("List empty: got %d books", len(books))
	}

	bs.Create(store.BookInput{Title: "Book A"})
	bs.Create(store.BookInput{Title: "Book B"})

	books, err = bs.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(books) != 2 {
		t.Errorf("List: got %d books, want 2", len(books))
	}
	// ordered by title
	if books[0].Title != "Book A" {
		t.Errorf("first book = %q, want Book A", books[0].Title)
	}
}

func TestCount(t *testing.T) {
	bs := newCleanStore(t)
	n, err := bs.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 0 {
		t.Errorf("Count empty = %d", n)
	}
	bs.Create(store.BookInput{Title: "X"})
	bs.Create(store.BookInput{Title: "Y"})
	n, err = bs.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 2 {
		t.Errorf("Count = %d, want 2", n)
	}
}

func TestListPage(t *testing.T) {
	bs := newCleanStore(t)
	for i := range 30 {
		bs.Create(store.BookInput{Title: titles[i%len(titles)] + string(rune('A'+i))})
	}

	page1, err := bs.ListPage(store.PageSize, 0)
	if err != nil {
		t.Fatalf("ListPage p1: %v", err)
	}
	if len(page1) != store.PageSize {
		t.Errorf("page1 len = %d, want %d", len(page1), store.PageSize)
	}

	page2, err := bs.ListPage(store.PageSize, store.PageSize)
	if err != nil {
		t.Fatalf("ListPage p2: %v", err)
	}
	if len(page2) != 5 {
		t.Errorf("page2 len = %d, want 5", len(page2))
	}

	// no overlap
	seen := map[int64]bool{}
	for _, b := range append(page1, page2...) {
		if seen[b.ID] {
			t.Errorf("duplicate book ID %d across pages", b.ID)
		}
		seen[b.ID] = true
	}
}

var titles = []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon"}

// --- Filtering ---

func seedFilterBooks(t *testing.T) *store.Books {
	t.Helper()
	bs := newCleanStore(t)
	books := []store.BookInput{
		{Title: "A", Authors: []string{"Orwell"}, PublishedDate: "1949", Categories: []string{"Fiction"}},
		{Title: "B", Authors: []string{"Orwell"}, PublishedDate: "1954", Categories: []string{"Fantasy"}},
		{Title: "C", Authors: []string{"Tolkien"}, PublishedDate: "1954", Categories: []string{"Fantasy"}},
		{Title: "D", Authors: []string{"Tolkien"}, PublishedDate: "1966", Categories: []string{"Fiction"}},
	}
	for _, in := range books {
		if _, err := bs.Create(in); err != nil {
			t.Fatalf("seed create: %v", err)
		}
	}
	return bs
}

func TestListFiltered_category(t *testing.T) {
	bs := seedFilterBooks(t)
	books, err := bs.ListFiltered("category", "Fantasy", 10, 0)
	if err != nil {
		t.Fatalf("ListFiltered category: %v", err)
	}
	if len(books) != 2 {
		t.Errorf("Fantasy count = %d, want 2", len(books))
	}
}

func TestListFiltered_author(t *testing.T) {
	bs := seedFilterBooks(t)
	books, err := bs.ListFiltered("author", "Tolkien", 10, 0)
	if err != nil {
		t.Fatalf("ListFiltered author: %v", err)
	}
	if len(books) != 2 {
		t.Errorf("Tolkien count = %d, want 2", len(books))
	}
}

func TestListFiltered_year(t *testing.T) {
	bs := seedFilterBooks(t)
	books, err := bs.ListFiltered("year", "1954", 10, 0)
	if err != nil {
		t.Fatalf("ListFiltered year: %v", err)
	}
	if len(books) != 2 {
		t.Errorf("1954 count = %d, want 2", len(books))
	}
}

func TestListFiltered_unknown_falls_back_to_all(t *testing.T) {
	bs := seedFilterBooks(t)
	books, err := bs.ListFiltered("unknown", "x", 10, 0)
	if err != nil {
		t.Fatalf("ListFiltered unknown: %v", err)
	}
	if len(books) != 4 {
		t.Errorf("fallback count = %d, want 4", len(books))
	}
}

func TestListFiltered_case_insensitive(t *testing.T) {
	bs := seedFilterBooks(t)
	books, err := bs.ListFiltered("category", "fiction", 10, 0)
	if err != nil {
		t.Fatalf("ListFiltered lowercase: %v", err)
	}
	if len(books) != 2 {
		t.Errorf("case-insensitive count = %d, want 2", len(books))
	}
}

func TestCountFiltered(t *testing.T) {
	bs := seedFilterBooks(t)

	n, err := bs.CountFiltered("category", "Fantasy")
	if err != nil {
		t.Fatalf("CountFiltered: %v", err)
	}
	if n != 2 {
		t.Errorf("CountFiltered Fantasy = %d, want 2", n)
	}

	n, err = bs.CountFiltered("year", "1949")
	if err != nil {
		t.Fatalf("CountFiltered year: %v", err)
	}
	if n != 1 {
		t.Errorf("CountFiltered 1949 = %d, want 1", n)
	}
}

func TestFilterValues(t *testing.T) {
	bs := seedFilterBooks(t)
	fv, err := bs.FilterValues()
	if err != nil {
		t.Fatalf("FilterValues: %v", err)
	}
	if len(fv.Categories) != 2 {
		t.Errorf("Categories = %v, want 2", fv.Categories)
	}
	if len(fv.Authors) != 2 {
		t.Errorf("Authors = %v, want 2", fv.Authors)
	}
	if len(fv.Years) != 3 {
		t.Errorf("Years = %v, want 3", fv.Years)
	}
	// years should be descending
	if fv.Years[0] != "1966" {
		t.Errorf("Years[0] = %q, want 1966", fv.Years[0])
	}
}

// --- FTS5 Search ---

func seedSearchBooks(t *testing.T) *store.Books {
	t.Helper()
	bs := newCleanStore(t)
	bs.Create(store.BookInput{Title: "The Hobbit", Authors: []string{"Tolkien"}})
	bs.Create(store.BookInput{Title: "Lord of the Rings", Authors: []string{"Tolkien"}})
	bs.Create(store.BookInput{Title: "Dune", Authors: []string{"Frank Herbert"}})
	return bs
}

func TestSearch_title(t *testing.T) {
	bs := seedSearchBooks(t)
	results, err := bs.Search("hobbit", "title", 10, 0)
	if err != nil {
		t.Fatalf("Search title: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("hobbit search = %d results, want 1", len(results))
	}
	if results[0].Title != "The Hobbit" {
		t.Errorf("result = %q, want The Hobbit", results[0].Title)
	}
}

func TestSearch_author(t *testing.T) {
	bs := seedSearchBooks(t)
	results, err := bs.Search("tolkien", "author", 10, 0)
	if err != nil {
		t.Fatalf("Search author: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("tolkien author search = %d results, want 2", len(results))
	}
}

func TestSearch_all_fields(t *testing.T) {
	bs := seedSearchBooks(t)
	results, err := bs.Search("rings", "", 10, 0)
	if err != nil {
		t.Fatalf("Search all: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("rings search = %d results, want 1", len(results))
	}
}

func TestSearch_multi_word(t *testing.T) {
	bs := seedSearchBooks(t)
	results, err := bs.Search("lord rings", "", 10, 0)
	if err != nil {
		t.Fatalf("Search multi: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("'lord rings' search = %d results, want 1", len(results))
	}
}

func TestSearch_no_results(t *testing.T) {
	bs := seedSearchBooks(t)
	results, err := bs.Search("zzznomatch", "", 10, 0)
	if err != nil {
		t.Fatalf("Search no results: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("no-match search = %d results, want 0", len(results))
	}
}

func TestSearchCount(t *testing.T) {
	bs := seedSearchBooks(t)
	n, err := bs.SearchCount("tolkien", "author")
	if err != nil {
		t.Fatalf("SearchCount: %v", err)
	}
	if n != 2 {
		t.Errorf("SearchCount tolkien author = %d, want 2", n)
	}
}

func TestSearch_updates_on_create(t *testing.T) {
	bs := newCleanStore(t)
	bs.Create(store.BookInput{Title: "New Book", Authors: []string{"New Author"}})

	results, err := bs.Search("new", "title", 10, 0)
	if err != nil {
		t.Fatalf("Search after create: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("search after create = %d results, want 1", len(results))
	}
}

func TestSearch_updates_on_delete(t *testing.T) {
	bs := newCleanStore(t)
	book, _ := bs.Create(store.BookInput{Title: "Temporary Book"})
	bs.Delete(book.ID)

	results, err := bs.Search("Temporary", "title", 10, 0)
	if err != nil {
		t.Fatalf("Search after delete: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("search after delete = %d results, want 0", len(results))
	}
}

// --- JSON array round-trip ---

func TestNilSlices_round_trip(t *testing.T) {
	bs := newCleanStore(t)
	book, err := bs.Create(store.BookInput{Title: "No Authors"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := bs.Get(book.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	// nil authors/categories should come back as nil, not panicking
	_ = got.Authors
	_ = got.Categories
}
