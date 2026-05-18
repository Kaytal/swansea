package store

import (
	"database/sql"
	"errors"
	"fmt"
)

type MusicAlbum struct {
	ID            int64    `json:"id"`
	Title         string   `json:"title"`
	Artists       []string `json:"artists"`
	Label         string   `json:"label"`
	ReleaseDate   string   `json:"release_date"`
	Description   string   `json:"description"`
	Genres        []string `json:"genres"`
	TrackCount    int      `json:"track_count"`
	CoverURL      string   `json:"cover_url"`
	CatalogNumber string   `json:"catalog_number"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
}

type MusicAlbumInput struct {
	Title         string   `json:"title"`
	Artists       []string `json:"artists"`
	Label         string   `json:"label"`
	ReleaseDate   string   `json:"release_date"`
	Description   string   `json:"description"`
	Genres        []string `json:"genres"`
	TrackCount    int      `json:"track_count"`
	CoverURL      string   `json:"cover_url"`
	CatalogNumber string   `json:"catalog_number"`
}

type MusicAlbums struct {
	db *sql.DB
}

func NewMusicAlbums(db *sql.DB) *MusicAlbums {
	return &MusicAlbums{db: db}
}

func scanMusicAlbum(row interface{ Scan(...any) error }) (*MusicAlbum, error) {
	var a MusicAlbum
	var artists, genres string
	err := row.Scan(
		&a.ID, &a.Title, &artists, &a.Label,
		&a.ReleaseDate, &a.Description, &genres, &a.TrackCount,
		&a.CoverURL, &a.CatalogNumber, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	a.Artists = unmarshalStrings(artists)
	a.Genres = unmarshalStrings(genres)
	return &a, nil
}

const musicSelectCols = `id, title, COALESCE(artists,'[]'), COALESCE(label,''),
	COALESCE(release_date,''), COALESCE(description,''),
	COALESCE(genres,'[]'), COALESCE(track_count,0),
	COALESCE(cover_url,''), COALESCE(catalog_number,''),
	created_at, updated_at`

func (s *MusicAlbums) ListPage(limit, offset int) ([]*MusicAlbum, error) {
	rows, err := s.db.Query(fmt.Sprintf(
		`SELECT %s FROM music ORDER BY title LIMIT ? OFFSET ?`, musicSelectCols,
	), limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var albums []*MusicAlbum
	for rows.Next() {
		a, err := scanMusicAlbum(rows)
		if err != nil {
			return nil, err
		}
		albums = append(albums, a)
	}
	return albums, rows.Err()
}

func (s *MusicAlbums) Count() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM music`).Scan(&n)
	return n, err
}

func (s *MusicAlbums) List() ([]*MusicAlbum, error) {
	rows, err := s.db.Query(fmt.Sprintf(`SELECT %s FROM music ORDER BY title`, musicSelectCols))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var albums []*MusicAlbum
	for rows.Next() {
		a, err := scanMusicAlbum(rows)
		if err != nil {
			return nil, err
		}
		albums = append(albums, a)
	}
	return albums, rows.Err()
}

func (s *MusicAlbums) ListFiltered(field, value string, limit, offset int) ([]*MusicAlbum, error) {
	var query string
	switch field {
	case "genre":
		query = fmt.Sprintf(`SELECT %s FROM music WHERE EXISTS (
			SELECT 1 FROM json_each(genres) WHERE LOWER(json_each.value) = LOWER(?)
		) ORDER BY title LIMIT ? OFFSET ?`, musicSelectCols)
	case "artist":
		query = fmt.Sprintf(`SELECT %s FROM music WHERE EXISTS (
			SELECT 1 FROM json_each(artists) WHERE LOWER(json_each.value) = LOWER(?)
		) ORDER BY title LIMIT ? OFFSET ?`, musicSelectCols)
	case "year":
		query = fmt.Sprintf(`SELECT %s FROM music WHERE substr(release_date,1,4) = ? ORDER BY title LIMIT ? OFFSET ?`, musicSelectCols)
	default:
		return s.ListPage(limit, offset)
	}
	rows, err := s.db.Query(query, value, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var albums []*MusicAlbum
	for rows.Next() {
		a, err := scanMusicAlbum(rows)
		if err != nil {
			return nil, err
		}
		albums = append(albums, a)
	}
	return albums, rows.Err()
}

func (s *MusicAlbums) CountFiltered(field, value string) (int, error) {
	var query string
	switch field {
	case "genre":
		query = `SELECT COUNT(*) FROM music WHERE EXISTS (
			SELECT 1 FROM json_each(genres) WHERE LOWER(json_each.value) = LOWER(?))`
	case "artist":
		query = `SELECT COUNT(*) FROM music WHERE EXISTS (
			SELECT 1 FROM json_each(artists) WHERE LOWER(json_each.value) = LOWER(?))`
	case "year":
		query = `SELECT COUNT(*) FROM music WHERE substr(release_date,1,4) = ?`
	default:
		return s.Count()
	}
	var n int
	err := s.db.QueryRow(query, value).Scan(&n)
	return n, err
}

type MusicFilterValues struct {
	Genres  []string
	Artists []string
	Years   []string
}

func (s *MusicAlbums) FilterValues() (*MusicFilterValues, error) {
	fv := &MusicFilterValues{}

	genreRows, err := s.db.Query(`
		SELECT DISTINCT json_each.value FROM music, json_each(music.genres)
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

	artistRows, err := s.db.Query(`
		SELECT DISTINCT json_each.value FROM music, json_each(music.artists)
		WHERE json_each.value != '' ORDER BY json_each.value`)
	if err != nil {
		return nil, err
	}
	defer artistRows.Close()
	for artistRows.Next() {
		var v string
		if err := artistRows.Scan(&v); err != nil {
			return nil, err
		}
		fv.Artists = append(fv.Artists, v)
	}
	if err := artistRows.Err(); err != nil {
		return nil, err
	}

	yearRows, err := s.db.Query(`
		SELECT DISTINCT substr(release_date,1,4) as yr FROM music
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

func (s *MusicAlbums) Search(query, field string, limit, offset int) ([]*MusicAlbum, error) {
	fts := ftsQuery(query)
	var col string
	switch field {
	case "title":
		col = "title"
	case "artist":
		col = "artists"
	default:
		col = "music_fts"
	}
	q := fmt.Sprintf(`SELECT %s FROM music WHERE id IN (
		SELECT rowid FROM music_fts WHERE %s MATCH ?
	) ORDER BY title LIMIT ? OFFSET ?`, musicSelectCols, col)
	rows, err := s.db.Query(q, fts, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var albums []*MusicAlbum
	for rows.Next() {
		a, err := scanMusicAlbum(rows)
		if err != nil {
			return nil, err
		}
		albums = append(albums, a)
	}
	return albums, rows.Err()
}

func (s *MusicAlbums) SearchCount(query, field string) (int, error) {
	fts := ftsQuery(query)
	var col string
	switch field {
	case "title":
		col = "title"
	case "artist":
		col = "artists"
	default:
		col = "music_fts"
	}
	q := fmt.Sprintf(`SELECT COUNT(*) FROM music WHERE id IN (
		SELECT rowid FROM music_fts WHERE %s MATCH ?
	)`, col)
	var n int
	err := s.db.QueryRow(q, fts).Scan(&n)
	return n, err
}

func (s *MusicAlbums) Get(id int64) (*MusicAlbum, error) {
	row := s.db.QueryRow(fmt.Sprintf(`SELECT %s FROM music WHERE id = ?`, musicSelectCols), id)
	a, err := scanMusicAlbum(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return a, err
}

func (s *MusicAlbums) Create(in MusicAlbumInput) (*MusicAlbum, error) {
	res, err := s.db.Exec(`
		INSERT INTO music (title, artists, label, release_date, description, genres, track_count, cover_url, catalog_number)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.Title, marshalStrings(in.Artists), in.Label, in.ReleaseDate,
		in.Description, marshalStrings(in.Genres), in.TrackCount, in.CoverURL, in.CatalogNumber,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.Get(id)
}

func (s *MusicAlbums) Update(id int64, in MusicAlbumInput) (*MusicAlbum, error) {
	res, err := s.db.Exec(`
		UPDATE music SET title=?, artists=?, label=?, release_date=?,
		description=?, genres=?, track_count=?, cover_url=?, catalog_number=?
		WHERE id=?`,
		in.Title, marshalStrings(in.Artists), in.Label, in.ReleaseDate,
		in.Description, marshalStrings(in.Genres), in.TrackCount, in.CoverURL, in.CatalogNumber, id,
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

func (s *MusicAlbums) UpdateCoverURL(id int64, coverURL string) error {
	_, err := s.db.Exec(`UPDATE music SET cover_url = ? WHERE id = ?`, coverURL, id)
	return err
}

func (s *MusicAlbums) Delete(id int64) error {
	res, err := s.db.Exec(`DELETE FROM music WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
