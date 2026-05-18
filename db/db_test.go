package db_test

import (
	"path/filepath"
	"testing"

	"swansea/db"
)

func TestOpen_creates_schema_and_migrates(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer database.Close()

	var version int
	if err := database.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatalf("PRAGMA user_version: %v", err)
	}
	if version != 7 {
		t.Errorf("user_version = %d, want 7", version)
	}
}

func TestOpen_idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	for i := range 2 {
		database, err := db.Open(path)
		if err != nil {
			t.Fatalf("db.Open attempt %d: %v", i+1, err)
		}
		database.Close()
	}
}

func TestMigrate_seed_book_present(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer database.Close()

	var title string
	err = database.QueryRow(`SELECT title FROM books WHERE isbn = '9780684801223'`).Scan(&title)
	if err != nil {
		t.Fatalf("seed book query: %v", err)
	}
	if title != "The Old Man and the Sea" {
		t.Errorf("seed title = %q, want 'The Old Man and the Sea'", title)
	}
}

func TestMigrate_fts_table_exists(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer database.Close()

	var n int
	err = database.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='books_fts'`,
	).Scan(&n)
	if err != nil {
		t.Fatalf("fts table query: %v", err)
	}
	if n != 1 {
		t.Error("books_fts virtual table not found")
	}
}

func TestMigrate_seed_in_fts(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer database.Close()

	var n int
	err = database.QueryRow(`SELECT COUNT(*) FROM books_fts WHERE title MATCH '"Old Man"*'`).Scan(&n)
	if err != nil {
		t.Fatalf("fts search: %v", err)
	}
	if n != 1 {
		t.Errorf("FTS match count = %d, want 1", n)
	}
}
