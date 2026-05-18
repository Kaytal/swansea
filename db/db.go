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

	if version < 5 {
		if _, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS video_games (
				id           INTEGER PRIMARY KEY AUTOINCREMENT,
				title        TEXT NOT NULL,
				platform     TEXT,
				developers   TEXT,
				publisher    TEXT,
				release_date TEXT,
				description  TEXT,
				genres       TEXT,
				cover_url    TEXT,
				rating       TEXT,
				created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP
			);

			CREATE TRIGGER IF NOT EXISTS video_games_updated_at
			AFTER UPDATE ON video_games
			BEGIN
				UPDATE video_games SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END;

			CREATE VIRTUAL TABLE IF NOT EXISTS video_games_fts USING fts5(
				title, developers, platform, content='video_games', content_rowid='id'
			);

			CREATE TRIGGER IF NOT EXISTS video_games_fts_insert AFTER INSERT ON video_games BEGIN
				INSERT INTO video_games_fts(rowid, title, developers, platform)
				VALUES (NEW.id, NEW.title, NEW.developers, NEW.platform);
			END;

			CREATE TRIGGER IF NOT EXISTS video_games_fts_update AFTER UPDATE ON video_games BEGIN
				INSERT INTO video_games_fts(video_games_fts, rowid, title, developers, platform)
				VALUES ('delete', OLD.id, OLD.title, OLD.developers, OLD.platform);
				INSERT INTO video_games_fts(rowid, title, developers, platform)
				VALUES (NEW.id, NEW.title, NEW.developers, NEW.platform);
			END;

			CREATE TRIGGER IF NOT EXISTS video_games_fts_delete AFTER DELETE ON video_games BEGIN
				INSERT INTO video_games_fts(video_games_fts, rowid, title, developers, platform)
				VALUES ('delete', OLD.id, OLD.title, OLD.developers, OLD.platform);
			END;
		`); err != nil {
			return err
		}
		if _, err := db.Exec(`PRAGMA user_version = 5`); err != nil {
			return err
		}
	}

	if version < 6 {
		if _, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS movies (
				id           INTEGER PRIMARY KEY AUTOINCREMENT,
				title        TEXT NOT NULL,
				directors    TEXT,
				studio       TEXT,
				release_date TEXT,
				description  TEXT,
				runtime      INTEGER,
				genres       TEXT,
				cover_url    TEXT,
				tmdb_id      INTEGER,
				created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP
			);

			CREATE TRIGGER IF NOT EXISTS movies_updated_at
			AFTER UPDATE ON movies
			BEGIN
				UPDATE movies SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END;

			CREATE VIRTUAL TABLE IF NOT EXISTS movies_fts USING fts5(
				title, directors, content='movies', content_rowid='id'
			);

			CREATE TRIGGER IF NOT EXISTS movies_fts_insert AFTER INSERT ON movies BEGIN
				INSERT INTO movies_fts(rowid, title, directors)
				VALUES (NEW.id, NEW.title, NEW.directors);
			END;

			CREATE TRIGGER IF NOT EXISTS movies_fts_update AFTER UPDATE ON movies BEGIN
				INSERT INTO movies_fts(movies_fts, rowid, title, directors)
				VALUES ('delete', OLD.id, OLD.title, OLD.directors);
				INSERT INTO movies_fts(rowid, title, directors)
				VALUES (NEW.id, NEW.title, NEW.directors);
			END;

			CREATE TRIGGER IF NOT EXISTS movies_fts_delete AFTER DELETE ON movies BEGIN
				INSERT INTO movies_fts(movies_fts, rowid, title, directors)
				VALUES ('delete', OLD.id, OLD.title, OLD.directors);
			END;
		`); err != nil {
			return err
		}
		if _, err := db.Exec(`PRAGMA user_version = 6`); err != nil {
			return err
		}
	}

	if version < 7 {
		if _, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS music (
				id             INTEGER PRIMARY KEY AUTOINCREMENT,
				title          TEXT NOT NULL,
				artists        TEXT,
				label          TEXT,
				release_date   TEXT,
				description    TEXT,
				genres         TEXT,
				track_count    INTEGER,
				cover_url      TEXT,
				catalog_number TEXT,
				created_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at     DATETIME DEFAULT CURRENT_TIMESTAMP
			);

			CREATE TRIGGER IF NOT EXISTS music_updated_at
			AFTER UPDATE ON music
			BEGIN
				UPDATE music SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END;

			CREATE VIRTUAL TABLE IF NOT EXISTS music_fts USING fts5(
				title, artists, content='music', content_rowid='id'
			);

			CREATE TRIGGER IF NOT EXISTS music_fts_insert AFTER INSERT ON music BEGIN
				INSERT INTO music_fts(rowid, title, artists)
				VALUES (NEW.id, NEW.title, NEW.artists);
			END;

			CREATE TRIGGER IF NOT EXISTS music_fts_update AFTER UPDATE ON music BEGIN
				INSERT INTO music_fts(music_fts, rowid, title, artists)
				VALUES ('delete', OLD.id, OLD.title, OLD.artists);
				INSERT INTO music_fts(rowid, title, artists)
				VALUES (NEW.id, NEW.title, NEW.artists);
			END;

			CREATE TRIGGER IF NOT EXISTS music_fts_delete AFTER DELETE ON music BEGIN
				INSERT INTO music_fts(music_fts, rowid, title, artists)
				VALUES ('delete', OLD.id, OLD.title, OLD.artists);
			END;
		`); err != nil {
			return err
		}
		if _, err := db.Exec(`PRAGMA user_version = 7`); err != nil {
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
