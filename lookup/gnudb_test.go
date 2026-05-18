package lookup

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// CDDB protocol responses are line-based text, not JSON.
func gnudbFindResponse(lines ...string) io.ReadCloser {
	return io.NopCloser(strings.NewReader(strings.Join(lines, "\n") + "\n"))
}

func gnudbReadResponse(lines ...string) io.ReadCloser {
	return io.NopCloser(strings.NewReader(strings.Join(lines, "\n") + "\n"))
}

func TestGnudbByArtistTitle_success(t *testing.T) {
	callCount := 0
	setClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		callCount++
		var body io.ReadCloser
		cmd := r.URL.Query().Get("cmd")
		if strings.HasPrefix(cmd, "cddb find") {
			// 211 = inexact matches, list follows
			body = gnudbFindResponse(
				"211 Found inexact matches, list follows (until terminating `.`)",
				"rock b2109d0c 12 Nirvana / Nevermind",
				".",
			)
		} else {
			// cddb read response
			body = gnudbReadResponse(
				"210 rock b2109d0c CD database entry follows (until terminating `.`)",
				"DISCID=b2109d0c",
				"DTITLE=Nirvana / Nevermind",
				"DYEAR=1991",
				"DGENRE=Rock",
				"TTITLE0=Smells Like Teen Spirit",
				"TTITLE1=In Bloom",
				"TTITLE2=Come as You Are",
				"EXTD=",
				".",
			)
		}
		return &http.Response{StatusCode: 200, Body: body, Header: make(http.Header)}, nil
	}))

	result, err := GnudbByArtistTitle("Nirvana Nevermind")
	if err != nil {
		t.Fatalf("GnudbByArtistTitle: %v", err)
	}
	if result.Title != "Nevermind" {
		t.Errorf("Title = %q, want Nevermind", result.Title)
	}
	if len(result.Artists) != 1 || result.Artists[0] != "Nirvana" {
		t.Errorf("Artists = %v, want [Nirvana]", result.Artists)
	}
	if result.ReleaseDate != "1991" {
		t.Errorf("ReleaseDate = %q, want 1991", result.ReleaseDate)
	}
	if len(result.Genres) != 1 || result.Genres[0] != "Rock" {
		t.Errorf("Genres = %v, want [Rock]", result.Genres)
	}
	if result.TrackCount != 3 {
		t.Errorf("TrackCount = %d, want 3", result.TrackCount)
	}
	if callCount != 2 {
		t.Errorf("expected 2 HTTP calls (find + read), got %d", callCount)
	}
}

func TestGnudbByArtistTitle_exact_match(t *testing.T) {
	setClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		cmd := r.URL.Query().Get("cmd")
		if strings.HasPrefix(cmd, "cddb find") {
			// 200 = single exact match on status line
			return &http.Response{
				StatusCode: 200,
				Body:       gnudbFindResponse("200 rock b2109d0c Nirvana / Nevermind"),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: 200,
			Body: gnudbReadResponse(
				"210 rock b2109d0c CD database entry follows (until terminating `.`)",
				"DTITLE=Nirvana / Nevermind",
				"DYEAR=1991",
				"DGENRE=Grunge",
				".",
			),
			Header: make(http.Header),
		}, nil
	}))

	result, err := GnudbByArtistTitle("Nevermind")
	if err != nil {
		t.Fatalf("GnudbByArtistTitle: %v", err)
	}
	if result.Title != "Nevermind" {
		t.Errorf("Title = %q, want Nevermind", result.Title)
	}
}

func TestGnudbByArtistTitle_not_found_empty_list(t *testing.T) {
	setClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       gnudbFindResponse("211 Found inexact matches, list follows (until terminating `.`)", "."),
			Header:     make(http.Header),
		}, nil
	}))

	_, err := GnudbByArtistTitle("zzznomatch")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("empty list: got %v, want ErrNotFound", err)
	}
}

func TestGnudbByArtistTitle_not_found_status(t *testing.T) {
	setClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       gnudbFindResponse("202 No match found"),
			Header:     make(http.Header),
		}, nil
	}))

	_, err := GnudbByArtistTitle("zzznomatch")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("202 status: got %v, want ErrNotFound", err)
	}
}

func TestGnudbByArtistTitle_http_error(t *testing.T) {
	setClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 503,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	}))

	_, err := GnudbByArtistTitle("Nirvana")
	if err == nil {
		t.Fatal("expected error for 503, got nil")
	}
}

func TestGnudbByArtistTitle_no_artist_separator(t *testing.T) {
	setClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		cmd := r.URL.Query().Get("cmd")
		if strings.HasPrefix(cmd, "cddb find") {
			return &http.Response{
				StatusCode: 200,
				Body:       gnudbFindResponse("200 misc abc123de Various Compilation"),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: 200,
			Body: gnudbReadResponse(
				"210 misc abc123de CD database entry follows (until terminating `.`)",
				"DTITLE=Greatest Hits",
				".",
			),
			Header: make(http.Header),
		}, nil
	}))

	result, err := GnudbByArtistTitle("Greatest Hits")
	if err != nil {
		t.Fatalf("GnudbByArtistTitle: %v", err)
	}
	if result.Title != "Greatest Hits" {
		t.Errorf("Title = %q, want Greatest Hits", result.Title)
	}
	if len(result.Artists) != 0 {
		t.Errorf("Artists should be empty when no separator, got %v", result.Artists)
	}
}

func TestGnudbByArtistTitle_multiline_title(t *testing.T) {
	// CDDB allows multi-line values by repeating the key
	setClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		cmd := r.URL.Query().Get("cmd")
		if strings.HasPrefix(cmd, "cddb find") {
			return &http.Response{
				StatusCode: 200,
				Body:       gnudbFindResponse("200 rock b2109d0c Nirvana / Nevermind"),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: 200,
			Body: gnudbReadResponse(
				"210 rock b2109d0c CD database entry follows (until terminating `.`)",
				"DTITLE=Nirvana / Never",
				"DTITLE=mind",
				".",
			),
			Header: make(http.Header),
		}, nil
	}))

	result, err := GnudbByArtistTitle("Nevermind")
	if err != nil {
		t.Fatalf("GnudbByArtistTitle: %v", err)
	}
	// DTITLE lines are concatenated: "Nirvana / Never" + "mind" = "Nirvana / Nevermind"
	if result.Title != "Nevermind" {
		t.Errorf("multiline DTITLE: Title = %q, want Nevermind", result.Title)
	}
	if len(result.Artists) == 0 || result.Artists[0] != "Nirvana" {
		t.Errorf("multiline DTITLE: Artists = %v, want [Nirvana]", result.Artists)
	}
}
