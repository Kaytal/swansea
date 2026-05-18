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

func tmdbGet(reqURL, apiKey string) (*http.Response, error) {
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	return HTTPClient.Do(req)
}

// TMDBSearchResult is a lightweight result for displaying TMDB search matches.
type TMDBSearchResult struct {
	ID          int64
	Title       string
	ReleaseYear string
	PosterURL   string
}

// TMDBSearch searches The Movie Database for movies matching title and returns a list of results.
// Requires the TMDB_API_KEY environment variable.
func TMDBSearch(title string) ([]TMDBSearchResult, error) {
	apiKey := os.Getenv("TMDB_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("TMDB_API_KEY must be set")
	}

	searchURL := fmt.Sprintf("%s/search/movie?query=%s", tmdbBase, url.QueryEscape(title))
	resp, err := tmdbGet(searchURL, apiKey)
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

	results := make([]TMDBSearchResult, len(sr.Results))
	for i, r := range sr.Results {
		year := r.ReleaseDate
		if len(year) > 4 {
			year = year[:4]
		}
		var poster string
		if r.PosterPath != "" {
			poster = tmdbImageBase + r.PosterPath
		}
		results[i] = TMDBSearchResult{ID: r.ID, Title: r.Title, ReleaseYear: year, PosterURL: poster}
	}
	return results, nil
}

// TMDBByID fetches full movie details (including credits) for a specific TMDB movie ID.
// Requires the TMDB_API_KEY environment variable.
func TMDBByID(id int64) (*store.MovieInput, error) {
	apiKey := os.Getenv("TMDB_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("TMDB_API_KEY must be set")
	}

	detailURL := fmt.Sprintf("%s/movie/%d?append_to_response=credits", tmdbBase, id)
	dresp, err := tmdbGet(detailURL, apiKey)
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

// TMDBByTitle searches TMDB for a movie by title and returns full details for the top result.
func TMDBByTitle(title string) (*store.MovieInput, error) {
	results, err := TMDBSearch(title)
	if err != nil {
		return nil, err
	}
	return TMDBByID(results[0].ID)
}
