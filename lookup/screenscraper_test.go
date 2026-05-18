package lookup

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

var fakeSSSearchResponse = map[string]any{
	"response": map[string]any{
		"jeux": []any{
			map[string]any{
				"noms":        []any{map[string]any{"region": "wor", "text": "The Legend of Zelda: Breath of the Wild"}},
				"systeme":     map[string]any{"text": "Nintendo Switch"},
				"developpeur": map[string]any{"text": "Nintendo EPD"},
				"editeur":     map[string]any{"text": "Nintendo"},
				"dates":       []any{map[string]any{"region": "wor", "text": "2017-03-03"}},
				"genres": []any{
					map[string]any{"noms": []any{map[string]any{"langue": "en", "text": "Action"}}},
					map[string]any{"noms": []any{map[string]any{"langue": "en", "text": "Adventure"}}},
				},
				"medias": []any{
					map[string]any{"type": "box-2D", "url": "https://screenscraper.fr/media/botw.jpg"},
				},
				"synopsis": []any{
					map[string]any{"langue": "en", "text": "Open world action adventure game"},
				},
			},
		},
	},
}

func withSSCreds(t *testing.T) {
	t.Helper()
	t.Setenv("SCREENSCRAPER_USERNAME", "testuser")
	t.Setenv("SCREENSCRAPER_PASSWORD", "testpass")
}

func TestScreenScraperByTitle_success(t *testing.T) {
	withSSCreds(t)
	setClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       fakeJSON(t, fakeSSSearchResponse),
			Header:     make(http.Header),
		}, nil
	}))

	result, err := ScreenScraperByTitle("Zelda Breath of the Wild")
	if err != nil {
		t.Fatalf("ScreenScraperByTitle: %v", err)
	}
	if result.Title != "The Legend of Zelda: Breath of the Wild" {
		t.Errorf("Title = %q", result.Title)
	}
	if result.Platform != "Nintendo Switch" {
		t.Errorf("Platform = %q, want Nintendo Switch", result.Platform)
	}
	if len(result.Developers) != 1 || result.Developers[0] != "Nintendo EPD" {
		t.Errorf("Developers = %v, want [Nintendo EPD]", result.Developers)
	}
	if result.Publisher != "Nintendo" {
		t.Errorf("Publisher = %q, want Nintendo", result.Publisher)
	}
	if len(result.Genres) != 2 {
		t.Errorf("Genres = %v, want 2", result.Genres)
	}
	if result.CoverURL == "" {
		t.Error("CoverURL should not be empty")
	}
	if result.Description == "" {
		t.Error("Description should not be empty")
	}
}

func TestScreenScraperByTitle_not_found(t *testing.T) {
	withSSCreds(t)
	setClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       fakeJSON(t, map[string]any{"response": map[string]any{"jeux": []any{}}}),
			Header:     make(http.Header),
		}, nil
	}))

	_, err := ScreenScraperByTitle("zzznomatch")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("empty results: got %v, want ErrNotFound", err)
	}
}

func TestScreenScraperByTitle_http_error(t *testing.T) {
	withSSCreds(t)
	setClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 503,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	}))

	_, err := ScreenScraperByTitle("Zelda")
	if err == nil {
		t.Fatal("expected error for 503, got nil")
	}
}

func TestScreenScraperByTitle_missing_credentials(t *testing.T) {
	t.Setenv("SCREENSCRAPER_USERNAME", "")
	t.Setenv("SCREENSCRAPER_PASSWORD", "")

	_, err := ScreenScraperByTitle("Zelda")
	if err == nil {
		t.Fatal("expected error when credentials missing, got nil")
	}
}

func TestScreenScraperByTitle_name_fallback(t *testing.T) {
	withSSCreds(t)
	resp := map[string]any{
		"response": map[string]any{
			"jeux": []any{
				map[string]any{
					// no "wor" region — should fall back to first name
					"noms": []any{map[string]any{"region": "jp", "text": "ゼルダの伝説"}},
				},
			},
		},
	}
	setClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: fakeJSON(t, resp), Header: make(http.Header)}, nil
	}))

	result, err := ScreenScraperByTitle("Zelda")
	if err != nil {
		t.Fatalf("ScreenScraperByTitle: %v", err)
	}
	if result.Title != "ゼルダの伝説" {
		t.Errorf("Title fallback = %q, want ゼルダの伝説", result.Title)
	}
}
