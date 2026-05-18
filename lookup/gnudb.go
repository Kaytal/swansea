package lookup

import (
	"bufio"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"swansea/store"
)

const gnudbBase = "https://gnudb.gnudb.org/~cddb/cddb.cgi"
const gnudbHello = "anonymous gnudb.gnudb.org swansea 1.0"
const gnudbProto = "6"

// GnudbSearchResult is a lightweight result from a gnudb find query.
type GnudbSearchResult struct {
	Category string
	DiscID   string
	RawTitle string // "Artist / Album" as returned by gnudb
}

// GnudbSearch searches gnudb.org for music matching query and returns all results.
// Uses the CDDB-over-HTTP protocol (no auth required).
func GnudbSearch(query string) ([]GnudbSearchResult, error) {
	params := url.Values{}
	params.Set("cmd", "cddb find "+query)
	params.Set("hello", gnudbHello)
	params.Set("proto", gnudbProto)

	apiURL := gnudbBase + "?" + params.Encode()
	resp, err := HTTPClient.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("gnudb find request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gnudb returned %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	if !scanner.Scan() {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, query)
	}
	statusLine := scanner.Text()

	if len(statusLine) < 3 {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, query)
	}
	code := statusLine[:3]

	var results []GnudbSearchResult
	switch code {
	case "200":
		// single exact match: "200 category discid Artist / Title"
		fields := strings.Fields(statusLine)
		if len(fields) < 3 {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, query)
		}
		rawTitle := strings.Join(fields[3:], " ")
		results = append(results, GnudbSearchResult{Category: fields[1], DiscID: fields[2], RawTitle: rawTitle})
	case "211", "210":
		// multiple matches: one per line, terminated by "."
		for scanner.Scan() {
			line := scanner.Text()
			if line == "." {
				break
			}
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			rawTitle := strings.Join(fields[2:], " ")
			results = append(results, GnudbSearchResult{Category: fields[0], DiscID: fields[1], RawTitle: rawTitle})
		}
		if len(results) == 0 {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, query)
		}
	default:
		return nil, fmt.Errorf("%w: %s", ErrNotFound, query)
	}
	return results, nil
}

// GnudbByDiscID fetches full album details for a specific gnudb category and disc ID.
func GnudbByDiscID(category, discID string) (*store.MusicAlbumInput, error) {
	return gnudbRead(category, discID)
}

// GnudbByArtistTitle searches gnudb.org for a music album by artist and title.
// Uses the CDDB-over-HTTP protocol (no auth required).
// The query parameter should be "Artist / Album Title" or just a title.
func GnudbByArtistTitle(query string) (*store.MusicAlbumInput, error) {
	results, err := GnudbSearch(query)
	if err != nil {
		return nil, err
	}
	return gnudbRead(results[0].Category, results[0].DiscID)
}

func isAlphanumeric(s string) bool {
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return len(s) > 0
}

func gnudbRead(category, discID string) (*store.MusicAlbumInput, error) {
	if !isAlphanumeric(category) || !isAlphanumeric(discID) {
		return nil, fmt.Errorf("%w: unexpected gnudb field format", ErrNotFound)
	}
	params := url.Values{}
	params.Set("cmd", fmt.Sprintf("cddb read %s %s", category, discID))
	params.Set("hello", gnudbHello)
	params.Set("proto", gnudbProto)

	apiURL := gnudbBase + "?" + params.Encode()
	resp, err := HTTPClient.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("gnudb read request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gnudb read returned %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	if !scanner.Scan() {
		return nil, fmt.Errorf("empty gnudb read response")
	}
	statusLine := scanner.Text()
	if !strings.HasPrefix(statusLine, "210") {
		return nil, fmt.Errorf("gnudb read failed: %s", statusLine)
	}

	entry := map[string]string{}
	var trackCount int
	for scanner.Scan() {
		line := scanner.Text()
		if line == "." {
			break
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		// count TTITLE lines for track count
		if strings.HasPrefix(line, "TTITLE") {
			trackCount++
		}
		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue
		}
		key := line[:idx]
		val := line[idx+1:]
		if existing, ok := entry[key]; ok {
			entry[key] = existing + val
		} else {
			entry[key] = val
		}
	}

	dtitle := entry["DTITLE"]
	var artist, title string
	if idx := strings.Index(dtitle, " / "); idx >= 0 {
		artist = strings.TrimSpace(dtitle[:idx])
		title = strings.TrimSpace(dtitle[idx+3:])
	} else {
		title = strings.TrimSpace(dtitle)
	}

	var artists []string
	if artist != "" {
		artists = []string{artist}
	}

	year := entry["DYEAR"]
	genre := entry["DGENRE"]
	var genres []string
	if genre != "" {
		genres = []string{genre}
	}

	return &store.MusicAlbumInput{
		Title:       title,
		Artists:     artists,
		ReleaseDate: year,
		Genres:      genres,
		TrackCount:  trackCount,
	}, nil
}
