package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var ErrNotFound = errors.New("not found")

type Book struct {
	ID            int64    `json:"id"`
	ISBN          string   `json:"isbn"`
	Title         string   `json:"title"`
	Authors       []string `json:"authors"`
	Publisher     string   `json:"publisher"`
	PublishedDate string   `json:"published_date"`
	Description   string   `json:"description"`
	PageCount     int      `json:"page_count"`
	CoverURL      string   `json:"cover_url"`
	Categories    []string `json:"categories"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
}

type BookInput struct {
	ISBN          string   `json:"isbn"`
	Title         string   `json:"title"`
	Authors       []string `json:"authors"`
	Publisher     string   `json:"publisher"`
	PublishedDate string   `json:"published_date"`
	Description   string   `json:"description"`
	PageCount     int      `json:"page_count"`
	CoverURL      string   `json:"cover_url"`
	Categories    []string `json:"categories"`
}

type Books struct {
	db *sql.DB
}

func NewBooks(db *sql.DB) *Books {
	return &Books{db: db}
}

func marshalStrings(ss []string) string {
	if len(ss) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(ss)
	return string(b)
}

func unmarshalStrings(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		// fallback: treat as comma-separated (legacy)
		return strings.Split(s, ",")
	}
	return out
}

func scanBook(row interface{ Scan(...any) error }) (*Book, error) {
	var b Book
	var authors, categories string
	err := row.Scan(
		&b.ID, &b.ISBN, &b.Title, &authors, &b.Publisher,
		&b.PublishedDate, &b.Description, &b.PageCount,
		&b.CoverURL, &categories, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	b.Authors = unmarshalStrings(authors)
	b.Categories = unmarshalStrings(categories)
	return &b, nil
}

const selectCols = `id, isbn, title, authors, publisher, published_date,
	description, page_count, cover_url, categories, created_at, updated_at`

func (s *Books) List() ([]*Book, error) {
	rows, err := s.db.Query(fmt.Sprintf(`SELECT %s FROM books ORDER BY title`, selectCols))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var books []*Book
	for rows.Next() {
		b, err := scanBook(rows)
		if err != nil {
			return nil, err
		}
		books = append(books, b)
	}
	return books, rows.Err()
}

func (s *Books) Get(id int64) (*Book, error) {
	row := s.db.QueryRow(fmt.Sprintf(`SELECT %s FROM books WHERE id = ?`, selectCols), id)
	b, err := scanBook(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return b, err
}

func (s *Books) GetByISBN(isbn string) (*Book, error) {
	row := s.db.QueryRow(fmt.Sprintf(`SELECT %s FROM books WHERE isbn = ?`, selectCols), isbn)
	b, err := scanBook(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return b, err
}

func (s *Books) Create(in BookInput) (*Book, error) {
	res, err := s.db.Exec(`
		INSERT INTO books (isbn, title, authors, publisher, published_date, description, page_count, cover_url, categories)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.ISBN, in.Title, marshalStrings(in.Authors), in.Publisher,
		in.PublishedDate, in.Description, in.PageCount, in.CoverURL,
		marshalStrings(in.Categories),
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.Get(id)
}

func (s *Books) Update(id int64, in BookInput) (*Book, error) {
	_, err := s.db.Exec(`
		UPDATE books SET isbn=?, title=?, authors=?, publisher=?, published_date=?,
		description=?, page_count=?, cover_url=?, categories=?
		WHERE id=?`,
		in.ISBN, in.Title, marshalStrings(in.Authors), in.Publisher,
		in.PublishedDate, in.Description, in.PageCount, in.CoverURL,
		marshalStrings(in.Categories), id,
	)
	if err != nil {
		return nil, err
	}
	return s.Get(id)
}

func (s *Books) Delete(id int64) error {
	res, err := s.db.Exec(`DELETE FROM books WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
