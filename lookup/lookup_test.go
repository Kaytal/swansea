package lookup

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// roundTripFunc lets us stub HTTP responses without a running server.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func fakeJSON(t *testing.T, v any) io.ReadCloser {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return io.NopCloser(strings.NewReader(string(b)))
}

func setClient(t *testing.T, transport http.RoundTripper) {
	orig := HTTPClient
	HTTPClient = &http.Client{Transport: transport}
	t.Cleanup(func() { HTTPClient = orig })
}

// --- For dispatcher ---

func TestFor_defaultsToGoogle(t *testing.T) {
	fn := For("")
	if fn == nil {
		t.Fatal("For('') returned nil")
	}
	// can't compare funcs, but For("unknown") and For("") should not be openlibrary
	fn2 := For("unknown")
	if fn2 == nil {
		t.Fatal("For('unknown') returned nil")
	}
}

func TestFor_openlibrary(t *testing.T) {
	fn := For("openlibrary")
	if fn == nil {
		t.Fatal("For('openlibrary') returned nil")
	}
}

// --- Google Books ---

var fakeGoogleResponse = map[string]any{
	"items": []any{
		map[string]any{
			"volumeInfo": map[string]any{
				"title":         "1984",
				"authors":       []string{"George Orwell"},
				"publisher":     "Secker & Warburg",
				"publishedDate": "1949",
				"description":   "Dystopian novel",
				"pageCount":     328,
				"categories":    []string{"Fiction"},
				"imageLinks":    map[string]any{"thumbnail": "https://books.google.com/covers/1984.jpg"},
				"industryIdentifiers": []any{
					map[string]any{"type": "ISBN_13", "identifier": "9780141036144"},
				},
			},
		},
	},
}

func TestGoogleByISBN_success(t *testing.T) {
	setClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       fakeJSON(t, fakeGoogleResponse),
			Header:     make(http.Header),
		}, nil
	}))

	result, err := GoogleByISBN("9780141036144")
	if err != nil {
		t.Fatalf("GoogleByISBN: %v", err)
	}
	if result.Title != "1984" {
		t.Errorf("Title = %q, want 1984", result.Title)
	}
	if result.ISBN != "9780141036144" {
		t.Errorf("ISBN = %q (should prefer ISBN_13)", result.ISBN)
	}
	if len(result.Authors) != 1 || result.Authors[0] != "George Orwell" {
		t.Errorf("Authors = %v", result.Authors)
	}
	if result.PageCount != 328 {
		t.Errorf("PageCount = %d, want 328", result.PageCount)
	}
	if result.PublishedDate != "1949" {
		t.Errorf("PublishedDate = %q", result.PublishedDate)
	}
}

func TestGoogleByISBN_upgrades_http_cover(t *testing.T) {
	resp := map[string]any{
		"items": []any{
			map[string]any{
				"volumeInfo": map[string]any{
					"title":      "Book",
					"imageLinks": map[string]any{"thumbnail": "http://books.google.com/cover.jpg"},
				},
			},
		},
	}
	setClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: fakeJSON(t, resp), Header: make(http.Header)}, nil
	}))

	result, err := GoogleByISBN("1234567890")
	if err != nil {
		t.Fatalf("GoogleByISBN: %v", err)
	}
	if !strings.HasPrefix(result.CoverURL, "https://") {
		t.Errorf("CoverURL should be upgraded to https, got %q", result.CoverURL)
	}
}

func TestGoogleByISBN_strips_hyphens(t *testing.T) {
	var gotURL string
	setClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotURL = r.URL.String()
		return &http.Response{StatusCode: 200, Body: fakeJSON(t, map[string]any{"items": []any{}}), Header: make(http.Header)}, nil
	}))

	GoogleByISBN("978-0-14-103614-4")
	if strings.Contains(gotURL, "-") {
		t.Errorf("hyphens not stripped from ISBN in URL: %s", gotURL)
	}
}

func TestGoogleByISBN_not_found(t *testing.T) {
	setClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       fakeJSON(t, map[string]any{"items": []any{}}),
			Header:     make(http.Header),
		}, nil
	}))

	_, err := GoogleByISBN("0000000000000")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("empty items: got %v, want ErrNotFound", err)
	}
}

func TestGoogleByISBN_http_error(t *testing.T) {
	setClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 429,
			Body:       io.NopCloser(strings.NewReader(`{"error":"rate limited"}`)),
			Header:     make(http.Header),
		}, nil
	}))

	_, err := GoogleByISBN("9780141036144")
	if err == nil {
		t.Fatal("expected error for 429, got nil")
	}
}

// --- Open Library ---

var fakeOLResponse = map[string]any{
	"ISBN:9780141036144": map[string]any{
		"title":           "Nineteen Eighty-Four",
		"number_of_pages": 328,
		"publish_date":    "June 1949",
		"publishers":      []any{map[string]any{"name": "Secker & Warburg"}},
		"authors":         []any{map[string]any{"name": "George Orwell"}},
		"subjects":        []any{map[string]any{"name": "Fiction"}, map[string]any{"name": "Dystopia"}},
		"cover":           map[string]any{"large": "https://covers.openlibrary.org/b/isbn/9780141036144-L.jpg"},
		"identifiers":     map[string]any{"isbn_13": []string{"9780141036144"}},
	},
}

func TestOpenLibraryByISBN_success(t *testing.T) {
	setClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: fakeJSON(t, fakeOLResponse), Header: make(http.Header)}, nil
	}))

	result, err := OpenLibraryByISBN("9780141036144")
	if err != nil {
		t.Fatalf("OpenLibraryByISBN: %v", err)
	}
	if result.Title != "Nineteen Eighty-Four" {
		t.Errorf("Title = %q", result.Title)
	}
	if result.ISBN != "9780141036144" {
		t.Errorf("ISBN = %q", result.ISBN)
	}
	if result.Publisher != "Secker & Warburg" {
		t.Errorf("Publisher = %q", result.Publisher)
	}
	if len(result.Authors) != 1 || result.Authors[0] != "George Orwell" {
		t.Errorf("Authors = %v", result.Authors)
	}
	if len(result.Categories) != 2 {
		t.Errorf("Categories = %v, want 2", result.Categories)
	}
	if result.PageCount != 328 {
		t.Errorf("PageCount = %d", result.PageCount)
	}
	if result.CoverURL == "" {
		t.Error("CoverURL should not be empty")
	}
}

func TestOpenLibraryByISBN_subjects_capped_at_5(t *testing.T) {
	subjects := make([]any, 8)
	for i := range subjects {
		subjects[i] = map[string]any{"name": "Subject"}
	}
	resp := map[string]any{
		"ISBN:1234567890": map[string]any{
			"title":    "Many Subjects",
			"subjects": subjects,
		},
	}
	setClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: fakeJSON(t, resp), Header: make(http.Header)}, nil
	}))

	result, err := OpenLibraryByISBN("1234567890")
	if err != nil {
		t.Fatalf("OpenLibraryByISBN: %v", err)
	}
	if len(result.Categories) > 5 {
		t.Errorf("Categories capped at 5, got %d", len(result.Categories))
	}
}

func TestOpenLibraryByISBN_cover_fallback(t *testing.T) {
	resp := map[string]any{
		"ISBN:1234567890": map[string]any{
			"title": "No Cover Book",
			// no "cover" field
		},
	}
	setClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: fakeJSON(t, resp), Header: make(http.Header)}, nil
	}))

	result, err := OpenLibraryByISBN("1234567890")
	if err != nil {
		t.Fatalf("OpenLibraryByISBN: %v", err)
	}
	if !strings.Contains(result.CoverURL, "1234567890") {
		t.Errorf("fallback CoverURL should include ISBN, got %q", result.CoverURL)
	}
}

func TestOpenLibraryByISBN_not_found(t *testing.T) {
	setClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       fakeJSON(t, map[string]any{}), // empty response map
			Header:     make(http.Header),
		}, nil
	}))

	_, err := OpenLibraryByISBN("0000000000000")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("empty map: got %v, want ErrNotFound", err)
	}
}

func TestOpenLibraryByISBN_http_error(t *testing.T) {
	setClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 503,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	}))

	_, err := OpenLibraryByISBN("9780141036144")
	if err == nil {
		t.Fatal("expected error for 503, got nil")
	}
}
