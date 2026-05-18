package lookup

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

var fakeTMDBSearchResponse = map[string]any{
	"results": []any{
		map[string]any{
			"id":           27205,
			"title":        "Inception",
			"overview":     "A thief who steals corporate secrets through dreams",
			"release_date": "2010-07-16",
			"poster_path":  "/9gk7adHYeDvHkCSEqAvQNLV5Uge.jpg",
		},
	},
}

var fakeTMDBMovieResponse = map[string]any{
	"id":           27205,
	"title":        "Inception",
	"overview":     "A thief who steals corporate secrets through dreams",
	"release_date": "2010-07-16",
	"runtime":      148,
	"poster_path":  "/9gk7adHYeDvHkCSEqAvQNLV5Uge.jpg",
	"genres": []any{
		map[string]any{"name": "Sci-Fi"},
		map[string]any{"name": "Thriller"},
	},
	"production_companies": []any{
		map[string]any{"name": "Warner Bros."},
	},
	"credits": map[string]any{
		"crew": []any{
			map[string]any{"job": "Director", "name": "Christopher Nolan"},
			map[string]any{"job": "Producer", "name": "Emma Thomas"},
		},
	},
}

func withTMDBKey(t *testing.T) {
	t.Helper()
	t.Setenv("TMDB_API_KEY", "testkey")
}

func TestTMDBByTitle_success(t *testing.T) {
	withTMDBKey(t)
	callCount := 0
	setClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		callCount++
		var body any
		if strings.Contains(r.URL.Path, "search") {
			body = fakeTMDBSearchResponse
		} else {
			body = fakeTMDBMovieResponse
		}
		return &http.Response{StatusCode: 200, Body: fakeJSON(t, body), Header: make(http.Header)}, nil
	}))

	result, err := TMDBByTitle("Inception")
	if err != nil {
		t.Fatalf("TMDBByTitle: %v", err)
	}
	if result.Title != "Inception" {
		t.Errorf("Title = %q", result.Title)
	}
	if len(result.Directors) != 1 || result.Directors[0] != "Christopher Nolan" {
		t.Errorf("Directors = %v, want [Christopher Nolan]", result.Directors)
	}
	if result.Runtime != 148 {
		t.Errorf("Runtime = %d, want 148", result.Runtime)
	}
	if len(result.Genres) != 2 {
		t.Errorf("Genres = %v, want 2", result.Genres)
	}
	if result.Studio != "Warner Bros." {
		t.Errorf("Studio = %q, want Warner Bros.", result.Studio)
	}
	if result.TmdbID != 27205 {
		t.Errorf("TmdbID = %d, want 27205", result.TmdbID)
	}
	if result.ReleaseDate != "2010" {
		t.Errorf("ReleaseDate = %q, want 2010", result.ReleaseDate)
	}
	if !strings.HasPrefix(result.CoverURL, "https://image.tmdb.org/t/p/w500/") {
		t.Errorf("CoverURL = %q, want tmdb image URL", result.CoverURL)
	}
	if callCount != 2 {
		t.Errorf("expected 2 HTTP calls (search + detail), got %d", callCount)
	}
}

func TestTMDBByTitle_directors_only_not_producers(t *testing.T) {
	withTMDBKey(t)
	setClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body any
		if strings.Contains(r.URL.Path, "search") {
			body = fakeTMDBSearchResponse
		} else {
			body = fakeTMDBMovieResponse
		}
		return &http.Response{StatusCode: 200, Body: fakeJSON(t, body), Header: make(http.Header)}, nil
	}))

	result, err := TMDBByTitle("Inception")
	if err != nil {
		t.Fatalf("TMDBByTitle: %v", err)
	}
	// crew has Director + Producer; only Director should be in result
	for _, d := range result.Directors {
		if d == "Emma Thomas" {
			t.Errorf("Producers should not be in Directors list, found %q", d)
		}
	}
}

func TestTMDBByTitle_not_found(t *testing.T) {
	withTMDBKey(t)
	setClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       fakeJSON(t, map[string]any{"results": []any{}}),
			Header:     make(http.Header),
		}, nil
	}))

	_, err := TMDBByTitle("zzznomatch")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("empty results: got %v, want ErrNotFound", err)
	}
}

func TestTMDBByTitle_http_error(t *testing.T) {
	withTMDBKey(t)
	setClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 503,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	}))

	_, err := TMDBByTitle("Inception")
	if err == nil {
		t.Fatal("expected error for 503, got nil")
	}
}

func TestTMDBByTitle_missing_api_key(t *testing.T) {
	t.Setenv("TMDB_API_KEY", "")

	_, err := TMDBByTitle("Inception")
	if err == nil {
		t.Fatal("expected error when API key missing, got nil")
	}
}

func TestTMDBByTitle_no_poster(t *testing.T) {
	withTMDBKey(t)
	movieResp := map[string]any{
		"id":    1,
		"title": "No Poster",
		// no poster_path
		"credits": map[string]any{"crew": []any{}},
		"genres":  []any{},
	}
	setClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body any
		if strings.Contains(r.URL.Path, "search") {
			body = map[string]any{"results": []any{map[string]any{"id": 1, "title": "No Poster"}}}
		} else {
			body = movieResp
		}
		return &http.Response{StatusCode: 200, Body: fakeJSON(t, body), Header: make(http.Header)}, nil
	}))

	result, err := TMDBByTitle("No Poster")
	if err != nil {
		t.Fatalf("TMDBByTitle: %v", err)
	}
	if result.CoverURL != "" {
		t.Errorf("CoverURL should be empty when no poster, got %q", result.CoverURL)
	}
}
