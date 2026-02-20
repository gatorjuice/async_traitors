package db

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

// Open opens a SQLite database at the given path and runs migrations.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(wal)&_pragma=foreign_keys(on)")
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)

	if err := RunMigrations(db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}
