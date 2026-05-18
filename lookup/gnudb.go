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

// GnudbByArtistTitle searches gnudb.org for a music album by artist and title.
// Uses the CDDB-over-HTTP protocol (no auth required).
// The query parameter should be "Artist / Album Title" or just a title.
func GnudbByArtistTitle(query string) (*store.MusicAlbumInput, error) {
	discID, category, err := gnudbFind(query)
	if err != nil {
		return nil, err
	}
	return gnudbRead(category, discID)
}

func gnudbFind(query string) (discID, category string, err error) {
	params := url.Values{}
	params.Set("cmd", "cddb find "+query)
	params.Set("hello", gnudbHello)
	params.Set("proto", gnudbProto)

	apiURL := gnudbBase + "?" + params.Encode()
	resp, err := HTTPClient.Get(apiURL)
	if err != nil {
		return "", "", fmt.Errorf("gnudb find request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("gnudb returned %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	if !scanner.Scan() {
		return "", "", fmt.Errorf("%w: %s", ErrNotFound, query)
	}
	statusLine := scanner.Text()

	// 200 = exact match, 211 = inexact matches follow
	code := statusLine[:3]
	switch code {
	case "200":
		// single exact match: "200 category discid Artist / Title"
		parts := strings.Fields(statusLine)
		if len(parts) < 3 {
			return "", "", fmt.Errorf("%w: %s", ErrNotFound, query)
		}
		return parts[2], parts[1], nil
	case "211", "210":
		// multiple matches — read the first entry line
		if !scanner.Scan() {
			return "", "", fmt.Errorf("%w: %s", ErrNotFound, query)
		}
		line := scanner.Text()
		if line == "." {
			return "", "", fmt.Errorf("%w: %s", ErrNotFound, query)
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			return "", "", fmt.Errorf("%w: %s", ErrNotFound, query)
		}
		return parts[1], parts[0], nil
	default:
		return "", "", fmt.Errorf("%w: %s", ErrNotFound, query)
	}
}

func gnudbRead(category, discID string) (*store.MusicAlbumInput, error) {
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
