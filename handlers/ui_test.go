package handlers_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"swansea/db"
	"swansea/handlers"
	"swansea/store"
)

func repoRoot() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..")
}

func newUIServer(t *testing.T) (*httptest.Server, *store.Books) {
	t.Helper()
	assetsFS := os.DirFS(repoRoot())

	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	s := store.NewBooks(database)
	// remove seed so tests are predictable
	books, _ := s.List()
	for _, b := range books {
		s.Delete(b.ID)
	}

	ui, err := handlers.NewUI(s, assetsFS, nil, "")
	if err != nil {
		t.Fatalf("NewUI: %v", err)
	}

	mux := http.NewServeMux()
	ui.Register(mux, assetsFS)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, s
}

func getText(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}

// --- Index ---

func TestUI_index(t *testing.T) {
	srv, _ := newUIServer(t)
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !strings.Contains(body, "Swansea Library") {
		t.Error("index should contain 'Swansea Library'")
	}
	if !strings.Contains(body, "htmx") {
		t.Error("index should include htmx script")
	}
}

func TestUI_index_unknown_path(t *testing.T) {
	srv, _ := newUIServer(t)
	resp, err := http.Get(srv.URL + "/nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("unknown path: status = %d, want 404", resp.StatusCode)
	}
}

// --- Books grid ---

func TestUI_books_empty(t *testing.T) {
	srv, _ := newUIServer(t)
	resp, err := http.Get(srv.URL + "/ui/books")
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !strings.Contains(body, "empty-state") {
		t.Error("empty library should show empty-state")
	}
}

func TestUI_books_with_content(t *testing.T) {
	srv, s := newUIServer(t)
	s.Create(store.BookInput{Title: "The Hobbit", Authors: []string{"Tolkien"}})

	resp, err := http.Get(srv.URL + "/ui/books")
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if !strings.Contains(body, "The Hobbit") {
		t.Error("books grid should contain book title")
	}
	if !strings.Contains(body, "Tolkien") {
		t.Error("books grid should contain author")
	}
}

func TestUI_books_pagination(t *testing.T) {
	srv, s := newUIServer(t)
	// insert more than one page
	for i := range store.PageSize + 3 {
		s.Create(store.BookInput{Title: "Book " + string(rune('A'+i%26))})
	}
	resp, err := http.Get(srv.URL + "/ui/books?page=2")
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("page 2: status = %d", resp.StatusCode)
	}
	if !strings.Contains(body, "pagination") {
		t.Error("multi-page result should include pagination")
	}
}

// --- Filters ---

func TestUI_filters(t *testing.T) {
	srv, s := newUIServer(t)
	s.Create(store.BookInput{
		Title:      "Dune",
		Authors:    []string{"Frank Herbert"},
		Categories: []string{"Science Fiction"},
		PublishedDate: "1965",
	})

	resp, err := http.Get(srv.URL + "/ui/filters")
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !strings.Contains(body, "Science Fiction") {
		t.Error("filters should list category 'Science Fiction'")
	}
	if !strings.Contains(body, "Frank Herbert") {
		t.Error("filters should list author 'Frank Herbert'")
	}
	if !strings.Contains(body, "1965") {
		t.Error("filters should list year '1965'")
	}
}

// --- Filtered books ---

func TestUI_books_filter_by_category(t *testing.T) {
	srv, s := newUIServer(t)
	s.Create(store.BookInput{Title: "Dune", Categories: []string{"Science Fiction"}})
	s.Create(store.BookInput{Title: "Hobbit", Categories: []string{"Fantasy"}})

	resp, err := http.Get(srv.URL + "/ui/books?category=Fantasy")
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if !strings.Contains(body, "Hobbit") {
		t.Error("filtered result should contain Hobbit")
	}
	if strings.Contains(body, "Dune") {
		t.Error("filtered result should not contain Dune")
	}
}

// --- Search ---

func TestUI_search_results(t *testing.T) {
	srv, s := newUIServer(t)
	s.Create(store.BookInput{Title: "1984", Authors: []string{"Orwell"}})
	s.Create(store.BookInput{Title: "Brave New World", Authors: []string{"Huxley"}})

	resp, err := http.Get(srv.URL + "/ui/search?q=1984&field=title")
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !strings.Contains(body, "1984") {
		t.Error("search should return 1984")
	}
	if strings.Contains(body, "Brave New World") {
		t.Error("search should not return Brave New World")
	}
}

func TestUI_search_empty_query_returns_books(t *testing.T) {
	srv, s := newUIServer(t)
	s.Create(store.BookInput{Title: "Something"})

	resp, err := http.Get(srv.URL + "/ui/search?q=")
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	// empty q delegates to books list
	if !strings.Contains(body, "Something") {
		t.Error("empty search should show all books")
	}
}

// --- Filter page routes ---

func TestUI_category_page(t *testing.T) {
	srv, _ := newUIServer(t)
	resp, err := http.Get(srv.URL + "/category/Fiction")
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !strings.Contains(body, "Swansea Library") {
		t.Error("category page should render full index shell")
	}
	// initial books URL should target the filtered endpoint
	if !strings.Contains(body, "category=Fiction") {
		t.Error("category page should wire up category filter")
	}
}

func TestUI_author_page(t *testing.T) {
	srv, _ := newUIServer(t)
	resp, err := http.Get(srv.URL + "/author/Tolkien")
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !strings.Contains(body, "author=Tolkien") {
		t.Error("author page should wire up author filter")
	}
}

func TestUI_year_page(t *testing.T) {
	srv, _ := newUIServer(t)
	resp, err := http.Get(srv.URL + "/year/1949")
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !strings.Contains(body, "year=1949") {
		t.Error("year page should wire up year filter")
	}
}

// --- Create via form ---

func TestUI_create_book(t *testing.T) {
	srv, s := newUIServer(t)
	form := url.Values{
		"title":   {"Foundation"},
		"authors": {"Isaac Asimov"},
	}
	resp, err := http.PostForm(srv.URL+"/ui/books", form)
	if err != nil {
		t.Fatal(err)
	}
	body := getText(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !strings.Contains(body, "Foundation") {
		t.Error("response should contain new book title")
	}
	if resp.Header.Get("HX-Trigger") != "closeModal" {
		t.Errorf("HX-Trigger = %q, want closeModal", resp.Header.Get("HX-Trigger"))
	}

	books, _ := s.List()
	found := false
	for _, b := range books {
		if b.Title == "Foundation" {
			found = true
		}
	}
	if !found {
		t.Error("book not persisted after form create")
	}
}

// --- Delete ---

func TestUI_delete_book(t *testing.T) {
	srv, s := newUIServer(t)
	book, _ := s.Create(store.BookInput{Title: "Deletable"})

	req, _ := http.NewRequest("DELETE", srv.URL+"/ui/books/"+itoa(book.ID), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	_, err = s.Get(book.ID)
	if !strings.Contains(err.Error(), "not found") {
		t.Error("book should be deleted")
	}
}

func itoa(id int64) string {
	return url.PathEscape(strings.TrimSpace(strings.ReplaceAll(fmt.Sprintf("%d", id), " ", "")))
}
