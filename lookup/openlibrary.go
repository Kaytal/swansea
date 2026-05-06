package lookup

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"swansea/store"
)

type olResponse map[string]olBook

type olBook struct {
	Title         string              `json:"title"`
	Publishers    []olName            `json:"publishers"`
	PublishDate   string              `json:"publish_date"`
	NumberOfPages int                 `json:"number_of_pages"`
	Authors       []olName            `json:"authors"`
	Subjects      []olName            `json:"subjects"`
	Cover         struct {
		Large  string `json:"large"`
		Medium string `json:"medium"`
	} `json:"cover"`
	Identifiers struct {
		ISBN13 []string `json:"isbn_13"`
		ISBN10 []string `json:"isbn_10"`
	} `json:"identifiers"`
}

type olName struct {
	Name string `json:"name"`
}

func OpenLibraryByISBN(isbn string) (*store.BookInput, error) {
	isbn = strings.ReplaceAll(isbn, "-", "")
	url := fmt.Sprintf("https://openlibrary.org/api/books?bibkeys=ISBN:%s&format=json&jscmd=data", isbn)

	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("open library request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("open library returned %d", resp.StatusCode)
	}

	var olr olResponse
	if err := json.NewDecoder(resp.Body).Decode(&olr); err != nil {
		return nil, fmt.Errorf("decoding open library response: %w", err)
	}

	book, ok := olr["ISBN:"+isbn]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, isbn)
	}

	resolvedISBN := isbn
	if len(book.Identifiers.ISBN13) > 0 {
		resolvedISBN = book.Identifiers.ISBN13[0]
	}

	var publisher string
	if len(book.Publishers) > 0 {
		publisher = book.Publishers[0].Name
	}

	authors := make([]string, len(book.Authors))
	for i, a := range book.Authors {
		authors[i] = a.Name
	}

	subjects := make([]string, 0, 5)
	for i, s := range book.Subjects {
		if i >= 5 {
			break
		}
		subjects = append(subjects, s.Name)
	}

	cover := book.Cover.Large
	if cover == "" {
		cover = book.Cover.Medium
	}
	if cover == "" {
		cover = fmt.Sprintf("https://covers.openlibrary.org/b/isbn/%s-L.jpg", isbn)
	}

	return &store.BookInput{
		ISBN:          resolvedISBN,
		Title:         book.Title,
		Authors:       authors,
		Publisher:     publisher,
		PublishedDate: book.PublishDate,
		PageCount:     book.NumberOfPages,
		CoverURL:      cover,
		Categories:    subjects,
	}, nil
}
