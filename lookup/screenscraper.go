package lookup

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"swansea/store"
)

type ssResponse struct {
	Response struct {
		Jeu ssGame `json:"jeu"`
	} `json:"response"`
}

type ssSearchResponse struct {
	Response struct {
		Jeux []ssGame `json:"jeux"`
	} `json:"response"`
}

type ssGame struct {
	Noms      []ssNom    `json:"noms"`
	Systeme   ssTextVal  `json:"systeme"`
	Developpeur ssTextVal `json:"developpeur"`
	Editeur   ssTextVal  `json:"editeur"`
	Dates     []ssDate   `json:"dates"`
	Genres    []ssGenre  `json:"genres"`
	Medias    []ssMedia  `json:"medias"`
	Synopsis  []ssSynopsis `json:"synopsis"`
}

type ssNom struct {
	Region string `json:"region"`
	Text   string `json:"text"`
}

type ssTextVal struct {
	Text string `json:"text"`
}

type ssDate struct {
	Region string `json:"region"`
	Text   string `json:"text"`
}

type ssGenre struct {
	Noms []ssLang `json:"noms"`
}

type ssLang struct {
	Langue string `json:"langue"`
	Text   string `json:"text"`
}

type ssMedia struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type ssSynopsis struct {
	Langue string `json:"langue"`
	Text   string `json:"text"`
}

// ScreenScraperSearch searches ScreenScraper.fr for games matching title and returns all results.
// Requires SCREENSCRAPER_USERNAME and SCREENSCRAPER_PASSWORD environment variables.
func ScreenScraperSearch(title string) ([]*store.VideoGameInput, error) {
	ssid := os.Getenv("SCREENSCRAPER_USERNAME")
	sspassword := os.Getenv("SCREENSCRAPER_PASSWORD")
	if ssid == "" || sspassword == "" {
		return nil, fmt.Errorf("SCREENSCRAPER_USERNAME and SCREENSCRAPER_PASSWORD must be set")
	}

	params := url.Values{}
	if devid := os.Getenv("SCREENSCRAPER_DEVID"); devid != "" {
		params.Set("devid", devid)
	}
	if devpw := os.Getenv("SCREENSCRAPER_DEVPASSWORD"); devpw != "" {
		params.Set("devpassword", devpw)
	}
	params.Set("softname", "swansea")
	params.Set("ssid", ssid)
	params.Set("sspassword", sspassword)
	params.Set("output", "json")
	params.Set("recherche", title)

	apiURL := "https://www.screenscraper.fr/api2/jeuRecherche.php?" + params.Encode()

	resp, err := HTTPClient.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("screenscraper request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("screenscraper returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading screenscraper response: %w", err)
	}

	var sr ssSearchResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		msg := strings.TrimSpace(string(body))
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return nil, fmt.Errorf("screenscraper error: %s", msg)
	}
	if len(sr.Response.Jeux) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, title)
	}

	results := make([]*store.VideoGameInput, len(sr.Response.Jeux))
	for i, g := range sr.Response.Jeux {
		results[i] = ssGameToInput(g)
	}
	return results, nil
}

// ScreenScraperByTitle returns the top result from ScreenScraper for the given title.
func ScreenScraperByTitle(title string) (*store.VideoGameInput, error) {
	results, err := ScreenScraperSearch(title)
	if err != nil {
		return nil, err
	}
	return results[0], nil
}

func ssGameToInput(g ssGame) *store.VideoGameInput {
	title := ssPickName(g.Noms)
	releaseDate := ssPickDate(g.Dates)
	description := ssPickSynopsis(g.Synopsis)
	genres := ssPickGenres(g.Genres)
	coverURL := ssPickCover(g.Medias)

	var developers []string
	if g.Developpeur.Text != "" {
		developers = []string{g.Developpeur.Text}
	}

	return &store.VideoGameInput{
		Title:       title,
		Platform:    g.Systeme.Text,
		Developers:  developers,
		Publisher:   g.Editeur.Text,
		ReleaseDate: releaseDate,
		Description: description,
		Genres:      genres,
		CoverURL:    coverURL,
	}
}

func ssPickName(noms []ssNom) string {
	for _, n := range noms {
		if n.Region == "wor" || n.Region == "us" {
			return n.Text
		}
	}
	if len(noms) > 0 {
		return noms[0].Text
	}
	return ""
}

func ssPickDate(dates []ssDate) string {
	for _, d := range dates {
		if d.Region == "wor" || d.Region == "us" {
			return d.Text
		}
	}
	if len(dates) > 0 {
		return dates[0].Text
	}
	return ""
}

func ssPickSynopsis(synopses []ssSynopsis) string {
	for _, s := range synopses {
		if s.Langue == "en" {
			return s.Text
		}
	}
	if len(synopses) > 0 {
		return synopses[0].Text
	}
	return ""
}

func ssPickGenres(genres []ssGenre) []string {
	var out []string
	for _, g := range genres {
		for _, n := range g.Noms {
			if n.Langue == "en" {
				out = append(out, n.Text)
				break
			}
		}
	}
	return out
}

func ssPickCover(medias []ssMedia) string {
	for _, m := range medias {
		if m.Type == "box-2D" || m.Type == "box-3D" {
			return m.URL
		}
	}
	return ""
}
