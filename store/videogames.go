package store

import (
	"database/sql"
	"errors"
	"fmt"
)

type VideoGame struct {
	ID          int64    `json:"id"`
	Title       string   `json:"title"`
	Platform    string   `json:"platform"`
	Developers  []string `json:"developers"`
	Publisher   string   `json:"publisher"`
	ReleaseDate string   `json:"release_date"`
	Description string   `json:"description"`
	Genres      []string `json:"genres"`
	CoverURL    string   `json:"cover_url"`
	Rating      string   `json:"rating"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

type VideoGameInput struct {
	Title       string   `json:"title"`
	Platform    string   `json:"platform"`
	Developers  []string `json:"developers"`
	Publisher   string   `json:"publisher"`
	ReleaseDate string   `json:"release_date"`
	Description string   `json:"description"`
	Genres      []string `json:"genres"`
	CoverURL    string   `json:"cover_url"`
	Rating      string   `json:"rating"`
}

type VideoGames struct {
	db *sql.DB
}

func NewVideoGames(db *sql.DB) *VideoGames {
	return &VideoGames{db: db}
}

func scanVideoGame(row interface{ Scan(...any) error }) (*VideoGame, error) {
	var g VideoGame
	var developers, genres string
	err := row.Scan(
		&g.ID, &g.Title, &g.Platform, &developers, &g.Publisher,
		&g.ReleaseDate, &g.Description, &genres, &g.CoverURL,
		&g.Rating, &g.CreatedAt, &g.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	g.Developers = unmarshalStrings(developers)
	g.Genres = unmarshalStrings(genres)
	return &g, nil
}

const gameSelectCols = `id, title, COALESCE(platform,''), COALESCE(developers,'[]'),
	COALESCE(publisher,''), COALESCE(release_date,''),
	COALESCE(description,''), COALESCE(genres,'[]'),
	COALESCE(cover_url,''), COALESCE(rating,''),
	created_at, updated_at`

func (s *VideoGames) ListPage(limit, offset int) ([]*VideoGame, error) {
	rows, err := s.db.Query(fmt.Sprintf(
		`SELECT %s FROM video_games ORDER BY title LIMIT ? OFFSET ?`, gameSelectCols,
	), limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var games []*VideoGame
	for rows.Next() {
		g, err := scanVideoGame(rows)
		if err != nil {
			return nil, err
		}
		games = append(games, g)
	}
	return games, rows.Err()
}

func (s *VideoGames) Count() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM video_games`).Scan(&n)
	return n, err
}

func (s *VideoGames) List() ([]*VideoGame, error) {
	rows, err := s.db.Query(fmt.Sprintf(`SELECT %s FROM video_games ORDER BY title`, gameSelectCols))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var games []*VideoGame
	for rows.Next() {
		g, err := scanVideoGame(rows)
		if err != nil {
			return nil, err
		}
		games = append(games, g)
	}
	return games, rows.Err()
}

func (s *VideoGames) ListFiltered(field, value string, limit, offset int) ([]*VideoGame, error) {
	var query string
	switch field {
	case "genre":
		query = fmt.Sprintf(`SELECT %s FROM video_games WHERE EXISTS (
			SELECT 1 FROM json_each(genres) WHERE LOWER(json_each.value) = LOWER(?)
		) ORDER BY title LIMIT ? OFFSET ?`, gameSelectCols)
	case "platform":
		query = fmt.Sprintf(`SELECT %s FROM video_games WHERE LOWER(platform) = LOWER(?) ORDER BY title LIMIT ? OFFSET ?`, gameSelectCols)
	case "year":
		query = fmt.Sprintf(`SELECT %s FROM video_games WHERE substr(release_date,1,4) = ? ORDER BY title LIMIT ? OFFSET ?`, gameSelectCols)
	default:
		return s.ListPage(limit, offset)
	}
	rows, err := s.db.Query(query, value, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var games []*VideoGame
	for rows.Next() {
		g, err := scanVideoGame(rows)
		if err != nil {
			return nil, err
		}
		games = append(games, g)
	}
	return games, rows.Err()
}

func (s *VideoGames) CountFiltered(field, value string) (int, error) {
	var query string
	switch field {
	case "genre":
		query = `SELECT COUNT(*) FROM video_games WHERE EXISTS (
			SELECT 1 FROM json_each(genres) WHERE LOWER(json_each.value) = LOWER(?))`
	case "platform":
		query = `SELECT COUNT(*) FROM video_games WHERE LOWER(platform) = LOWER(?)`
	case "year":
		query = `SELECT COUNT(*) FROM video_games WHERE substr(release_date,1,4) = ?`
	default:
		return s.Count()
	}
	var n int
	err := s.db.QueryRow(query, value).Scan(&n)
	return n, err
}

type GameFilterValues struct {
	Genres    []string
	Platforms []string
	Years     []string
}

func (s *VideoGames) FilterValues() (*GameFilterValues, error) {
	fv := &GameFilterValues{}

	genreRows, err := s.db.Query(`
		SELECT DISTINCT json_each.value FROM video_games, json_each(video_games.genres)
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

	platformRows, err := s.db.Query(`
		SELECT DISTINCT platform FROM video_games
		WHERE platform != '' AND platform IS NOT NULL ORDER BY platform`)
	if err != nil {
		return nil, err
	}
	defer platformRows.Close()
	for platformRows.Next() {
		var v string
		if err := platformRows.Scan(&v); err != nil {
			return nil, err
		}
		fv.Platforms = append(fv.Platforms, v)
	}
	if err := platformRows.Err(); err != nil {
		return nil, err
	}

	yearRows, err := s.db.Query(`
		SELECT DISTINCT substr(release_date,1,4) as yr FROM video_games
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

func (s *VideoGames) Search(query, field string, limit, offset int) ([]*VideoGame, error) {
	fts := ftsQuery(query)
	var col string
	switch field {
	case "title":
		col = "title"
	case "developer":
		col = "developers"
	default:
		col = "video_games_fts"
	}
	q := fmt.Sprintf(`SELECT %s FROM video_games WHERE id IN (
		SELECT rowid FROM video_games_fts WHERE %s MATCH ?
	) ORDER BY title LIMIT ? OFFSET ?`, gameSelectCols, col)
	rows, err := s.db.Query(q, fts, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var games []*VideoGame
	for rows.Next() {
		g, err := scanVideoGame(rows)
		if err != nil {
			return nil, err
		}
		games = append(games, g)
	}
	return games, rows.Err()
}

func (s *VideoGames) SearchCount(query, field string) (int, error) {
	fts := ftsQuery(query)
	var col string
	switch field {
	case "title":
		col = "title"
	case "developer":
		col = "developers"
	default:
		col = "video_games_fts"
	}
	q := fmt.Sprintf(`SELECT COUNT(*) FROM video_games WHERE id IN (
		SELECT rowid FROM video_games_fts WHERE %s MATCH ?
	)`, col)
	var n int
	err := s.db.QueryRow(q, fts).Scan(&n)
	return n, err
}

func (s *VideoGames) Get(id int64) (*VideoGame, error) {
	row := s.db.QueryRow(fmt.Sprintf(`SELECT %s FROM video_games WHERE id = ?`, gameSelectCols), id)
	g, err := scanVideoGame(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return g, err
}

func (s *VideoGames) Create(in VideoGameInput) (*VideoGame, error) {
	res, err := s.db.Exec(`
		INSERT INTO video_games (title, platform, developers, publisher, release_date, description, genres, cover_url, rating)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.Title, in.Platform, marshalStrings(in.Developers), in.Publisher,
		in.ReleaseDate, in.Description, marshalStrings(in.Genres), in.CoverURL, in.Rating,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.Get(id)
}

func (s *VideoGames) Update(id int64, in VideoGameInput) (*VideoGame, error) {
	res, err := s.db.Exec(`
		UPDATE video_games SET title=?, platform=?, developers=?, publisher=?, release_date=?,
		description=?, genres=?, cover_url=?, rating=?
		WHERE id=?`,
		in.Title, in.Platform, marshalStrings(in.Developers), in.Publisher,
		in.ReleaseDate, in.Description, marshalStrings(in.Genres), in.CoverURL, in.Rating, id,
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

func (s *VideoGames) UpdateCoverURL(id int64, coverURL string) error {
	_, err := s.db.Exec(`UPDATE video_games SET cover_url = ? WHERE id = ?`, coverURL, id)
	return err
}

func (s *VideoGames) Delete(id int64) error {
	res, err := s.db.Exec(`DELETE FROM video_games WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

