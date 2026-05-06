package lookup

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"swansea/store"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

type googleResponse struct {
	Items []struct {
		VolumeInfo struct {
			Title      string   `json:"title"`
			Authors    []string `json:"authors"`
			Publisher  string   `json:"publisher"`
			PublishedDate string `json:"publishedDate"`
			Description   string `json:"description"`
			PageCount     int    `json:"pageCount"`
			Categories    []string `json:"categories"`
			ImageLinks struct {
				Thumbnail string `json:"thumbnail"`
			} `json:"imageLinks"`
			IndustryIdentifiers []struct {
				Type       string `json:"type"`
				Identifier string `json:"identifier"`
			} `json:"industryIdentifiers"`
		} `json:"volumeInfo"`
	} `json:"items"`
}

func ByISBN(isbn string) (*store.BookInput, error) {
	isbn = strings.ReplaceAll(isbn, "-", "")
	url := fmt.Sprintf("https://www.googleapis.com/books/v1/volumes?q=isbn:%s", isbn)
	if key := os.Getenv("GOOGLE_BOOKS_API_KEY"); key != "" {
		url += "&key=" + key
	}

	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("google books request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google books returned %d", resp.StatusCode)
	}

	var gr googleResponse
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return nil, fmt.Errorf("decoding google books response: %w", err)
	}
	if len(gr.Items) == 0 {
		return nil, fmt.Errorf("no results found for ISBN %s", isbn)
	}

	vi := gr.Items[0].VolumeInfo

	// prefer ISBN-13, fall back to whatever was searched
	resolvedISBN := isbn
	for _, id := range vi.IndustryIdentifiers {
		if id.Type == "ISBN_13" {
			resolvedISBN = id.Identifier
			break
		}
	}

	// Google sometimes returns http thumbnail URLs; upgrade to https
	cover := strings.Replace(vi.ImageLinks.Thumbnail, "http://", "https://", 1)

	return &store.BookInput{
		ISBN:          resolvedISBN,
		Title:         vi.Title,
		Authors:       vi.Authors,
		Publisher:     vi.Publisher,
		PublishedDate: vi.PublishedDate,
		Description:   vi.Description,
		PageCount:     vi.PageCount,
		CoverURL:      cover,
		Categories:    vi.Categories,
	}, nil
}
