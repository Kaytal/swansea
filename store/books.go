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

const PageSize = 25

func (s *Books) ListPage(limit, offset int) ([]*Book, error) {
	rows, err := s.db.Query(fmt.Sprintf(
		`SELECT %s FROM books ORDER BY title LIMIT ? OFFSET ?`, selectCols,
	), limit, offset)
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

func (s *Books) Count() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM books`).Scan(&n)
	return n, err
}

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

func (s *Books) ListFiltered(field, value string, limit, offset int) ([]*Book, error) {
	var query string
	switch field {
	case "category":
		query = fmt.Sprintf(`SELECT %s FROM books WHERE EXISTS (
			SELECT 1 FROM json_each(categories) WHERE LOWER(json_each.value) = LOWER(?)
		) ORDER BY title LIMIT ? OFFSET ?`, selectCols)
	case "author":
		query = fmt.Sprintf(`SELECT %s FROM books WHERE EXISTS (
			SELECT 1 FROM json_each(authors) WHERE LOWER(json_each.value) = LOWER(?)
		) ORDER BY title LIMIT ? OFFSET ?`, selectCols)
	case "year":
		query = fmt.Sprintf(`SELECT %s FROM books WHERE substr(published_date,1,4) = ? ORDER BY title LIMIT ? OFFSET ?`, selectCols)
	default:
		return s.ListPage(limit, offset)
	}
	rows, err := s.db.Query(query, value, limit, offset)
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

func (s *Books) CountFiltered(field, value string) (int, error) {
	var query string
	switch field {
	case "category":
		query = `SELECT COUNT(*) FROM books WHERE EXISTS (
			SELECT 1 FROM json_each(categories) WHERE LOWER(json_each.value) = LOWER(?))`
	case "author":
		query = `SELECT COUNT(*) FROM books WHERE EXISTS (
			SELECT 1 FROM json_each(authors) WHERE LOWER(json_each.value) = LOWER(?))`
	case "year":
		query = `SELECT COUNT(*) FROM books WHERE substr(published_date,1,4) = ?`
	default:
		return s.Count()
	}
	var n int
	err := s.db.QueryRow(query, value).Scan(&n)
	return n, err
}

type FilterValues struct {
	Categories []string
	Authors    []string
	Years      []string
}

func (s *Books) FilterValues() (*FilterValues, error) {
	fv := &FilterValues{}

	catRows, err := s.db.Query(`
		SELECT DISTINCT json_each.value FROM books, json_each(books.categories)
		WHERE json_each.value != '' ORDER BY json_each.value`)
	if err != nil {
		return nil, err
	}
	defer catRows.Close()
	for catRows.Next() {
		var v string
		if err := catRows.Scan(&v); err != nil {
			return nil, err
		}
		fv.Categories = append(fv.Categories, v)
	}
	if err := catRows.Err(); err != nil {
		return nil, err
	}

	authorRows, err := s.db.Query(`
		SELECT DISTINCT json_each.value FROM books, json_each(books.authors)
		WHERE json_each.value != '' ORDER BY json_each.value`)
	if err != nil {
		return nil, err
	}
	defer authorRows.Close()
	for authorRows.Next() {
		var v string
		if err := authorRows.Scan(&v); err != nil {
			return nil, err
		}
		fv.Authors = append(fv.Authors, v)
	}
	if err := authorRows.Err(); err != nil {
		return nil, err
	}

	yearRows, err := s.db.Query(`
		SELECT DISTINCT substr(published_date,1,4) as yr FROM books
		WHERE yr != '' AND yr IS NOT NULL AND yr GLOB '[0-9][0-9][0-9][0-9]'
		ORDER BY yr DESC`)
	if err != nil {
		return nil, err
	}
	defer yearRows.Close()
	for yearRows.Next() {
		var v string
		if err := yearRows.Scan(&v); err != nil {
			return nil, err
		}
		fv.Years = append(fv.Years, v)
	}
	if err := yearRows.Err(); err != nil {
		return nil, err
	}

	return fv, nil
}

func ftsQuery(q string) string {
	q = strings.ReplaceAll(q, `"`, `""`)
	return `"` + q + `"*`
}

func (s *Books) Search(query, field string, limit, offset int) ([]*Book, error) {
	fts := ftsQuery(query)
	var col string
	switch field {
	case "title":
		col = "title"
	case "author":
		col = "authors"
	default:
		col = "books_fts"
	}
	q := fmt.Sprintf(`SELECT %s FROM books WHERE id IN (
		SELECT rowid FROM books_fts WHERE %s MATCH ?
	) ORDER BY title LIMIT ? OFFSET ?`, selectCols, col)
	rows, err := s.db.Query(q, fts, limit, offset)
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

func (s *Books) SearchCount(query, field string) (int, error) {
	fts := ftsQuery(query)
	var col string
	switch field {
	case "title":
		col = "title"
	case "author":
		col = "authors"
	default:
		col = "books_fts"
	}
	q := fmt.Sprintf(`SELECT COUNT(*) FROM books WHERE id IN (
		SELECT rowid FROM books_fts WHERE %s MATCH ?
	)`, col)
	var n int
	err := s.db.QueryRow(q, fts).Scan(&n)
	return n, err
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
