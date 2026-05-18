package lookup

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"swansea/store"
)

const tmdbBase = "https://api.themoviedb.org/3"
const tmdbImageBase = "https://image.tmdb.org/t/p/w500"

type tmdbSearchResponse struct {
	Results []struct {
		ID          int64  `json:"id"`
		Title       string `json:"title"`
		Overview    string `json:"overview"`
		ReleaseDate string `json:"release_date"`
		PosterPath  string `json:"poster_path"`
	} `json:"results"`
}

type tmdbMovieResponse struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Overview    string `json:"overview"`
	ReleaseDate string `json:"release_date"`
	Runtime     int    `json:"runtime"`
	PosterPath  string `json:"poster_path"`
	Genres      []struct {
		Name string `json:"name"`
	} `json:"genres"`
	ProductionCompanies []struct {
		Name string `json:"name"`
	} `json:"production_companies"`
	Credits struct {
		Crew []struct {
			Job  string `json:"job"`
			Name string `json:"name"`
		} `json:"crew"`
	} `json:"credits"`
}

// TMDBByTitle searches The Movie Database for a movie by title.
// Requires the TMDB_API_KEY environment variable.
func TMDBByTitle(title string) (*store.MovieInput, error) {
	apiKey := os.Getenv("TMDB_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("TMDB_API_KEY must be set")
	}

	searchURL := fmt.Sprintf("%s/search/movie?query=%s&api_key=%s",
		tmdbBase, url.QueryEscape(title), url.QueryEscape(apiKey))

	resp, err := HTTPClient.Get(searchURL)
	if err != nil {
		return nil, fmt.Errorf("tmdb search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tmdb returned %d", resp.StatusCode)
	}

	var sr tmdbSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("decoding tmdb search response: %w", err)
	}
	if len(sr.Results) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, title)
	}

	movieID := sr.Results[0].ID
	detailURL := fmt.Sprintf("%s/movie/%d?append_to_response=credits&api_key=%s",
		tmdbBase, movieID, url.QueryEscape(apiKey))

	dresp, err := HTTPClient.Get(detailURL)
	if err != nil {
		return nil, fmt.Errorf("tmdb detail request failed: %w", err)
	}
	defer dresp.Body.Close()

	if dresp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tmdb detail returned %d", dresp.StatusCode)
	}

	var mr tmdbMovieResponse
	if err := json.NewDecoder(dresp.Body).Decode(&mr); err != nil {
		return nil, fmt.Errorf("decoding tmdb movie response: %w", err)
	}

	var directors []string
	for _, c := range mr.Credits.Crew {
		if c.Job == "Director" {
			directors = append(directors, c.Name)
		}
	}

	genres := make([]string, len(mr.Genres))
	for i, g := range mr.Genres {
		genres[i] = g.Name
	}

	var studio string
	if len(mr.ProductionCompanies) > 0 {
		studio = mr.ProductionCompanies[0].Name
	}

	var posterURL string
	if mr.PosterPath != "" {
		posterURL = tmdbImageBase + mr.PosterPath
	}

	releaseYear := mr.ReleaseDate
	if len(releaseYear) > 4 && strings.Contains(releaseYear, "-") {
		releaseYear = releaseYear[:4]
	}

	return &store.MovieInput{
		Title:       mr.Title,
		Directors:   directors,
		Studio:      studio,
		ReleaseDate: releaseYear,
		Description: mr.Overview,
		Runtime:     mr.Runtime,
		Genres:      genres,
		CoverURL:    posterURL,
		TmdbID:      mr.ID,
	}, nil
}
