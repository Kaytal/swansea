package store

import (
	"database/sql"
	"errors"
	"fmt"
)

type Movie struct {
	ID          int64    `json:"id"`
	Title       string   `json:"title"`
	Directors   []string `json:"directors"`
	Studio      string   `json:"studio"`
	ReleaseDate string   `json:"release_date"`
	Description string   `json:"description"`
	Runtime     int      `json:"runtime"`
	Genres      []string `json:"genres"`
	CoverURL    string   `json:"cover_url"`
	TmdbID      int64    `json:"tmdb_id"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

type MovieInput struct {
	Title       string   `json:"title"`
	Directors   []string `json:"directors"`
	Studio      string   `json:"studio"`
	ReleaseDate string   `json:"release_date"`
	Description string   `json:"description"`
	Runtime     int      `json:"runtime"`
	Genres      []string `json:"genres"`
	CoverURL    string   `json:"cover_url"`
	TmdbID      int64    `json:"tmdb_id"`
}

type Movies struct {
	db *sql.DB
}

func NewMovies(db *sql.DB) *Movies {
	return &Movies{db: db}
}

func scanMovie(row interface{ Scan(...any) error }) (*Movie, error) {
	var m Movie
	var directors, genres string
	err := row.Scan(
		&m.ID, &m.Title, &directors, &m.Studio,
		&m.ReleaseDate, &m.Description, &m.Runtime, &genres,
		&m.CoverURL, &m.TmdbID, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	m.Directors = unmarshalStrings(directors)
	m.Genres = unmarshalStrings(genres)
	return &m, nil
}

const movieSelectCols = `id, title, COALESCE(directors,'[]'), COALESCE(studio,''),
	COALESCE(release_date,''), COALESCE(description,''),
	COALESCE(runtime,0), COALESCE(genres,'[]'),
	COALESCE(cover_url,''), COALESCE(tmdb_id,0),
	created_at, updated_at`

func (s *Movies) ListPage(limit, offset int) ([]*Movie, error) {
	rows, err := s.db.Query(fmt.Sprintf(
		`SELECT %s FROM movies ORDER BY title LIMIT ? OFFSET ?`, movieSelectCols,
	), limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var movies []*Movie
	for rows.Next() {
		m, err := scanMovie(rows)
		if err != nil {
			return nil, err
		}
		movies = append(movies, m)
	}
	return movies, rows.Err()
}

func (s *Movies) Count() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM movies`).Scan(&n)
	return n, err
}

func (s *Movies) List() ([]*Movie, error) {
	rows, err := s.db.Query(fmt.Sprintf(`SELECT %s FROM movies ORDER BY title`, movieSelectCols))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var movies []*Movie
	for rows.Next() {
		m, err := scanMovie(rows)
		if err != nil {
			return nil, err
		}
		movies = append(movies, m)
	}
	return movies, rows.Err()
}

func (s *Movies) ListFiltered(field, value string, limit, offset int) ([]*Movie, error) {
	var query string
	switch field {
	case "genre":
		query = fmt.Sprintf(`SELECT %s FROM movies WHERE EXISTS (
			SELECT 1 FROM json_each(genres) WHERE LOWER(json_each.value) = LOWER(?)
		) ORDER BY title LIMIT ? OFFSET ?`, movieSelectCols)
	case "director":
		query = fmt.Sprintf(`SELECT %s FROM movies WHERE EXISTS (
			SELECT 1 FROM json_each(directors) WHERE LOWER(json_each.value) = LOWER(?)
		) ORDER BY title LIMIT ? OFFSET ?`, movieSelectCols)
	case "year":
		query = fmt.Sprintf(`SELECT %s FROM movies WHERE substr(release_date,1,4) = ? ORDER BY title LIMIT ? OFFSET ?`, movieSelectCols)
	default:
		return s.ListPage(limit, offset)
	}
	rows, err := s.db.Query(query, value, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var movies []*Movie
	for rows.Next() {
		m, err := scanMovie(rows)
		if err != nil {
			return nil, err
		}
		movies = append(movies, m)
	}
	return movies, rows.Err()
}

func (s *Movies) CountFiltered(field, value string) (int, error) {
	var query string
	switch field {
	case "genre":
		query = `SELECT COUNT(*) FROM movies WHERE EXISTS (
			SELECT 1 FROM json_each(genres) WHERE LOWER(json_each.value) = LOWER(?))`
	case "director":
		query = `SELECT COUNT(*) FROM movies WHERE EXISTS (
			SELECT 1 FROM json_each(directors) WHERE LOWER(json_each.value) = LOWER(?))`
	case "year":
		query = `SELECT COUNT(*) FROM movies WHERE substr(release_date,1,4) = ?`
	default:
		return s.Count()
	}
	var n int
	err := s.db.QueryRow(query, value).Scan(&n)
	return n, err
}

type MovieFilterValues struct {
	Genres    []string
	Directors []string
	Years     []string
}

func (s *Movies) FilterValues() (*MovieFilterValues, error) {
	fv := &MovieFilterValues{}

	genreRows, err := s.db.Query(`
		SELECT DISTINCT json_each.value FROM movies, json_each(movies.genres)
		WHERE json_each.value != '' ORDER BY json_each.value`)
	if err != nil {
		return nil, err
	}
	defer genreRows.Close()
	for genreRows.Next() {
		var v string
		if err := genreRows.Scan(&v); err != nil {
			return nil, err
		}
		fv.Genres = append(fv.Genres, v)
	}
	if err := genreRows.Err(); err != nil {
		return nil, err
	}

	directorRows, err := s.db.Query(`
		SELECT DISTINCT json_each.value FROM movies, json_each(movies.directors)
		WHERE json_each.value != '' ORDER BY json_each.value`)
	if err != nil {
		return nil, err
	}
	defer directorRows.Close()
	for directorRows.Next() {
		var v string
		if err := directorRows.Scan(&v); err != nil {
			return nil, err
		}
		fv.Directors = append(fv.Directors, v)
	}
	if err := directorRows.Err(); err != nil {
		return nil, err
	}

	yearRows, err := s.db.Query(`
		SELECT DISTINCT substr(release_date,1,4) as yr FROM movies
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

func (s *Movies) Search(query, field string, limit, offset int) ([]*Movie, error) {
	fts := ftsQuery(query)
	var col string
	switch field {
	case "title":
		col = "title"
	case "director":
		col = "directors"
	default:
		col = "movies_fts"
	}
	q := fmt.Sprintf(`SELECT %s FROM movies WHERE id IN (
		SELECT rowid FROM movies_fts WHERE %s MATCH ?
	) ORDER BY title LIMIT ? OFFSET ?`, movieSelectCols, col)
	rows, err := s.db.Query(q, fts, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var movies []*Movie
	for rows.Next() {
		m, err := scanMovie(rows)
		if err != nil {
			return nil, err
		}
		movies = append(movies, m)
	}
	return movies, rows.Err()
}

func (s *Movies) SearchCount(query, field string) (int, error) {
	fts := ftsQuery(query)
	var col string
	switch field {
	case "title":
		col = "title"
	case "director":
		col = "directors"
	default:
		col = "movies_fts"
	}
	q := fmt.Sprintf(`SELECT COUNT(*) FROM movies WHERE id IN (
		SELECT rowid FROM movies_fts WHERE %s MATCH ?
	)`, col)
	var n int
	err := s.db.QueryRow(q, fts).Scan(&n)
	return n, err
}

func (s *Movies) Get(id int64) (*Movie, error) {
	row := s.db.QueryRow(fmt.Sprintf(`SELECT %s FROM movies WHERE id = ?`, movieSelectCols), id)
	m, err := scanMovie(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return m, err
}

func (s *Movies) Create(in MovieInput) (*Movie, error) {
	res, err := s.db.Exec(`
		INSERT INTO movies (title, directors, studio, release_date, description, runtime, genres, cover_url, tmdb_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.Title, marshalStrings(in.Directors), in.Studio, in.ReleaseDate,
		in.Description, in.Runtime, marshalStrings(in.Genres), in.CoverURL, in.TmdbID,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.Get(id)
}

func (s *Movies) Update(id int64, in MovieInput) (*Movie, error) {
	res, err := s.db.Exec(`
		UPDATE movies SET title=?, directors=?, studio=?, release_date=?,
		description=?, runtime=?, genres=?, cover_url=?, tmdb_id=?
		WHERE id=?`,
		in.Title, marshalStrings(in.Directors), in.Studio, in.ReleaseDate,
		in.Description, in.Runtime, marshalStrings(in.Genres), in.CoverURL, in.TmdbID, id,
	)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, ErrNotFound
	}
	return s.Get(id)
}

func (s *Movies) UpdateCoverURL(id int64, coverURL string) error {
	_, err := s.db.Exec(`UPDATE movies SET cover_url = ? WHERE id = ?`, coverURL, id)
	return err
}

func (s *Movies) Delete(id int64) error {
	res, err := s.db.Exec(`DELETE FROM movies WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
