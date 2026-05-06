package db

import (
	"database/sql"
	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, err
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS books (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			isbn          TEXT UNIQUE,
			title         TEXT NOT NULL,
			authors       TEXT,
			publisher     TEXT,
			published_date TEXT,
			description   TEXT,
			page_count    INTEGER,
			cover_url     TEXT,
			categories    TEXT,
			created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TRIGGER IF NOT EXISTS books_updated_at
		AFTER UPDATE ON books
		BEGIN
			UPDATE books SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
		END;
	`)
	return err
}
