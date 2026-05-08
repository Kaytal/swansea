package db

import (
	"database/sql"
	"fmt"
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
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("reading user_version: %w", err)
	}

	if version < 1 {
		if _, err := db.Exec(`
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
		`); err != nil {
			return err
		}
		if _, err := db.Exec(`PRAGMA user_version = 1`); err != nil {
			return err
		}
	}

	if version < 2 {
		if err := migrateFTS(db); err != nil {
			return err
		}
		if _, err := db.Exec(`PRAGMA user_version = 2`); err != nil {
			return err
		}
	}

	if version < 3 {
		if _, err := db.Exec(`
			INSERT OR IGNORE INTO books (isbn, title, authors, publisher, published_date, page_count, categories)
			VALUES (
				'9780684801223',
				'The Old Man and the Sea',
				'["Ernest Hemingway"]',
				'Scribner',
				'1952',
				127,
				'["Fiction","Classics"]'
			)`,
		); err != nil {
			return err
		}
		if _, err := db.Exec(`PRAGMA user_version = 3`); err != nil {
			return err
		}
	}

	if version < 4 {
		// Rebuild FTS index — the original backfill check used COUNT(*) which on a
		// content='books' FTS5 table reads from the books table (not the index),
		// so it was always non-zero and the backfill never ran.
		if _, err := db.Exec(`INSERT INTO books_fts(books_fts) VALUES('rebuild')`); err != nil {
			return err
		}
		if _, err := db.Exec(`PRAGMA user_version = 4`); err != nil {
			return err
		}
	}

	return nil
}

func migrateFTS(db *sql.DB) error {
	stmts := []string{
		`CREATE VIRTUAL TABLE IF NOT EXISTS books_fts USING fts5(
			title, authors, content='books', content_rowid='id'
		)`,
		`CREATE TRIGGER IF NOT EXISTS books_fts_insert AFTER INSERT ON books BEGIN
			INSERT INTO books_fts(rowid, title, authors) VALUES (NEW.id, NEW.title, NEW.authors);
		END`,
		`CREATE TRIGGER IF NOT EXISTS books_fts_update AFTER UPDATE ON books BEGIN
			INSERT INTO books_fts(books_fts, rowid, title, authors) VALUES ('delete', OLD.id, OLD.title, OLD.authors);
			INSERT INTO books_fts(rowid, title, authors) VALUES (NEW.id, NEW.title, NEW.authors);
		END`,
		`CREATE TRIGGER IF NOT EXISTS books_fts_delete AFTER DELETE ON books BEGIN
			INSERT INTO books_fts(books_fts, rowid, title, authors) VALUES ('delete', OLD.id, OLD.title, OLD.authors);
		END`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}

	// Backfill index from any rows that existed before FTS was added.
	// Note: COUNT(*) on a content='books' FTS5 table reads from the content table,
	// not the index, so we use rebuild instead of a count guard.
	if _, err := db.Exec(`INSERT INTO books_fts(books_fts) VALUES('rebuild')`); err != nil {
		return err
	}
	return nil
}
