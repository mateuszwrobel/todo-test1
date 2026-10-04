// Package todos owns the todo collection's durable state and the rules that
// keep it valid. It is flat CRUD over one SQLite table (ADR-002) — the domain
// is one record type with three fields.
package todos

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // pure-Go, cgo-free SQLite driver
)

// Todo is a single todo record.
type Todo struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

// Store is an open handle on the todo data file. One process opens one file at
// a time; the composition root owns the handle's lifecycle.
type Store struct {
	db *sql.DB
}

// schema is created on open (create-if-not-exists); single table, no
// migration framework. autoincrement rowids are never reused, even after
// delete.
const schema = `CREATE TABLE IF NOT EXISTS todos (
	id integer primary key autoincrement,
	title text not null,
	done integer not null default 0
)`

// Open opens (creating when absent) the SQLite data file at path and ensures
// the schema. The path is supplied by the composition root; this module
// chooses nothing about deployment location.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open data file %q: %w", path, err)
	}
	// Single-writer assumption: one pooled connection keeps SQLite locking
	// trivial inside one process.
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("open data file %q: %w", path, err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema at %q: %w", path, err)
	}
	return &Store{db: db}, nil
}

// Close releases the data file. Completed operations are already committed.
func (s *Store) Close() error {
	return s.db.Close()
}

// Create inserts a todo with the given title, done false, and a fresh
// identifier. The write is committed before the result returns.
func (s *Store) Create(title string) (Todo, error) {
	res, err := s.db.Exec(`INSERT INTO todos (title) VALUES (?)`, title)
	if err != nil {
		return Todo{}, fmt.Errorf("create todo: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Todo{}, fmt.Errorf("create todo: %w", err)
	}
	return Todo{ID: id, Title: title, Done: false}, nil
}

// List returns every todo in creation order — ascending identifier, oldest
// first.
func (s *Store) List() ([]Todo, error) {
	rows, err := s.db.Query(`SELECT id, title, done FROM todos ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list todos: %w", err)
	}
	defer rows.Close()

	var todos []Todo
	for rows.Next() {
		var t Todo
		if err := rows.Scan(&t.ID, &t.Title, &t.Done); err != nil {
			return nil, fmt.Errorf("list todos: %w", err)
		}
		todos = append(todos, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list todos: %w", err)
	}
	return todos, nil
}
