package handlers_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"swansea/db"
	"swansea/handlers"
	"swansea/lookup"
	"swansea/store"
)

// roundTripFunc stubs HTTP responses in the lookup package.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func newBooksServer(t *testing.T) (*httptest.Server, *store.Books) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	s := store.NewBooks(database)
	// clear seed data so tests start clean
	books, _ := s.List()
	for _, b := range books {
		s.Delete(b.ID)
	}

	mux := http.NewServeMux()
	handlers.NewBooks(s).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, s
}

func doJSON(t *testing.T, method, url string, body any) *http.Response {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func decodeBook(t *testing.T, resp *http.Response) store.Book {
	t.Helper()
	defer resp.Body.Close()
	var b store.Book
	if err := json.NewDecoder(resp.Body).Decode(&b); err != nil {
		t.Fatalf("decode Book: %v", err)
	}
	return b
}

func decodeBooks(t *testing.T, resp *http.Response) []store.Book {
	t.Helper()
	defer resp.Body.Close()
	var books []store.Book
	if err := json.NewDecoder(resp.Body).Decode(&books); err != nil {
		t.Fatalf("decode []Book: %v", err)
	}
	return books
}

// --- List ---

func TestList_empty(t *testing.T) {
	srv, _ := newBooksServer(t)
	resp := doJSON(t, "GET", srv.URL+"/books", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	books := decodeBooks(t, resp)
	if len(books) != 0 {
		t.Errorf("expected empty list, got %d books", len(books))
	}
}

func TestList_returns_all(t *testing.T) {
	srv, s := newBooksServer(t)
	s.Create(store.BookInput{Title: "Alpha"})
	s.Create(store.BookInput{Title: "Beta"})

	resp := doJSON(t, "GET", srv.URL+"/books", nil)
	books := decodeBooks(t, resp)
	if len(books) != 2 {
		t.Errorf("expected 2 books, got %d", len(books))
	}
}

// --- Create ---

func TestCreate_success(t *testing.T) {
	srv, _ := newBooksServer(t)
	resp := doJSON(t, "POST", srv.URL+"/books", map[string]any{
		"title":   "1984",
		"authors": []string{"George Orwell"},
		"isbn":    "9780141036144",
	})
	if resp.StatusCode != 201 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	book := decodeBook(t, resp)
	if book.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if book.Title != "1984" {
		t.Errorf("Title = %q", book.Title)
	}
	if book.ISBN != "9780141036144" {
		t.Errorf("ISBN = %q", book.ISBN)
	}
}

func TestCreate_missing_title(t *testing.T) {
	srv, _ := newBooksServer(t)
	resp := doJSON(t, "POST", srv.URL+"/books", map[string]any{"isbn": "123"})
	if resp.StatusCode != 400 {
		t.Fatalf("missing title: status = %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestCreate_invalid_json(t *testing.T) {
	srv, _ := newBooksServer(t)
	req, _ := http.NewRequest("POST", srv.URL+"/books", strings.NewReader("not json"))
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

func TestGet_success(t *testing.T) {
	srv, s := newBooksServer(t)
	book, _ := s.Create(store.BookInput{Title: "Dune", Authors: []string{"Frank Herbert"}})

	resp := doJSON(t, "GET", fmt.Sprintf("%s/books/%d", srv.URL, book.ID), nil)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	got := decodeBook(t, resp)
	if got.Title != "Dune" {
		t.Errorf("Title = %q", got.Title)
	}
	if len(got.Authors) != 1 || got.Authors[0] != "Frank Herbert" {
		t.Errorf("Authors = %v", got.Authors)
	}
}

func TestGet_not_found(t *testing.T) {
	srv, _ := newBooksServer(t)
	resp := doJSON(t, "GET", srv.URL+"/books/99999", nil)
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestGet_invalid_id(t *testing.T) {
	srv, _ := newBooksServer(t)
	resp := doJSON(t, "GET", srv.URL+"/books/notanid", nil)
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// --- Update ---

func TestUpdate_success(t *testing.T) {
	srv, s := newBooksServer(t)
	book, _ := s.Create(store.BookInput{Title: "Old Title"})

	resp := doJSON(t, "PUT", fmt.Sprintf("%s/books/%d", srv.URL, book.ID), map[string]any{
		"title":   "New Title",
		"authors": []string{"New Author"},
	})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	got := decodeBook(t, resp)
	if got.Title != "New Title" {
		t.Errorf("Title = %q", got.Title)
	}
}

func TestUpdate_not_found(t *testing.T) {
	srv, _ := newBooksServer(t)
	resp := doJSON(t, "PUT", srv.URL+"/books/99999", map[string]any{"title": "Ghost"})
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestUpdate_missing_title(t *testing.T) {
	srv, s := newBooksServer(t)
	book, _ := s.Create(store.BookInput{Title: "Something"})

	resp := doJSON(t, "PUT", fmt.Sprintf("%s/books/%d", srv.URL, book.ID), map[string]any{})
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// --- Delete ---

func TestDelete_success(t *testing.T) {
	srv, s := newBooksServer(t)
	book, _ := s.Create(store.BookInput{Title: "To Delete"})

	resp := doJSON(t, "DELETE", fmt.Sprintf("%s/books/%d", srv.URL, book.ID), nil)
	resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}

	_, err := s.Get(book.ID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Error("book should be gone after delete")
	}
}

func TestDelete_not_found(t *testing.T) {
	srv, _ := newBooksServer(t)
	resp := doJSON(t, "DELETE", srv.URL+"/books/99999", nil)
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// --- Lookup ---

var fakeGoogleResp = `{"items":[{"volumeInfo":{"title":"1984","authors":["George Orwell"],"pageCount":328,"industryIdentifiers":[{"type":"ISBN_13","identifier":"9780141036144"}]}}]}`

func stubLookup(t *testing.T, statusCode int, body string) {
	t.Helper()
	orig := lookup.HTTPClient
	lookup.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: statusCode,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	}
	t.Cleanup(func() { lookup.HTTPClient = orig })
}

func TestLookupISBN_success(t *testing.T) {
	srv, _ := newBooksServer(t)
	stubLookup(t, 200, fakeGoogleResp)

	resp := doJSON(t, "GET", srv.URL+"/lookup/9780141036144", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var result store.BookInput
	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Title != "1984" {
		t.Errorf("Title = %q", result.Title)
	}
}

func TestLookupISBN_not_found(t *testing.T) {
	srv, _ := newBooksServer(t)
	stubLookup(t, 200, `{"items":[]}`)

	resp := doJSON(t, "GET", srv.URL+"/lookup/0000000000000", nil)
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestLookupISBN_upstream_error(t *testing.T) {
	srv, _ := newBooksServer(t)
	stubLookup(t, 429, `{"error":"rate limited"}`)

	resp := doJSON(t, "GET", srv.URL+"/lookup/9780141036144", nil)
	resp.Body.Close()
	if resp.StatusCode != 502 {
		t.Fatalf("upstream 429: status = %d, want 502", resp.StatusCode)
	}
}

func TestAddByISBN_success(t *testing.T) {
	srv, s := newBooksServer(t)
	stubLookup(t, 200, fakeGoogleResp)

	resp := doJSON(t, "POST", srv.URL+"/books/isbn/9780141036144", nil)
	if resp.StatusCode != 201 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	book := decodeBook(t, resp)
	if book.Title != "1984" {
		t.Errorf("Title = %q", book.Title)
	}

	// persisted in DB
	got, err := s.GetByISBN("9780141036144")
	if err != nil {
		t.Fatalf("GetByISBN: %v", err)
	}
	if got.ID != book.ID {
		t.Errorf("ID mismatch: %d vs %d", got.ID, book.ID)
	}
}

func TestAddByISBN_not_found(t *testing.T) {
	srv, _ := newBooksServer(t)
	stubLookup(t, 200, `{"items":[]}`)

	resp := doJSON(t, "POST", srv.URL+"/books/isbn/0000000000000", nil)
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// --- Content-Type ---

func TestJSON_content_type(t *testing.T) {
	srv, _ := newBooksServer(t)
	resp := doJSON(t, "GET", srv.URL+"/books", nil)
	defer resp.Body.Close()
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
