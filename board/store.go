// Package board owns the kanban board's durable state: the card collection,
// the three fixed columns, per-column ordering, and persistence. It is a flat
// record store over one SQLite table (ADR-002 conventions carried over) — the
// domain is one record type plus one invariant: within each column, positions
// are exactly 0..n-1, top-to-bottom = priority.
package board

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // pure-Go, cgo-free SQLite driver
)

// Column is one of the board's three fixed columns. The set is this module's
// enumeration; display titles belong to the contract layer, not here.
type Column string

// The fixed column set, in board order.
const (
	Todo       Column = "todo"
	InProgress Column = "in_progress"
	Done       Column = "done"
)

// columns is the board's fixed column order. List always answers in it.
var columns = []Column{Todo, InProgress, Done}

// Card is a single board record. The fields are exactly the contract's Card
// model — nothing more is stored.
type Card struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Column   Column `json:"column"`
	Position int    `json:"position"`
}

// ColumnCards is one column of a listed board: its name and its cards
// top-to-bottom by position.
type ColumnCards struct {
	Name  Column
	Cards []Card
}

// schema is created on open (create-if-not-exists); single table, no migration
// framework. autoincrement rowids are never reused, even after delete. The
// title CHECK is the minimal storage-level guard (non-blank, ≤500 chars); the
// store's full validation behavior lands with the create-rejection cards.
// "column" is quoted because COLUMN is a reserved word in SQLite.
const schema = `CREATE TABLE IF NOT EXISTS cards (
	id integer primary key autoincrement,
	title text not null check (trim(title) <> '' and length(title) <= 500),
	"column" text not null check ("column" in ('todo','in_progress','done')),
	position integer not null check (position >= 0)
);
CREATE INDEX IF NOT EXISTS cards_column_position ON cards ("column", position)`

// Store is an open handle on the board data file. One process opens one file
// at a time; the composition root owns the handle's lifecycle and the path.
type Store struct {
	db *sql.DB
}

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

// Create inserts a card at the bottom of the todo column — position equals the
// current todo count, so the column's positions stay contiguous 0..n-1 — with
// a fresh store-assigned identifier, and returns the Card. The count and the
// insert run in one transaction, so the append is all-or-nothing. The text is
// stored as given; full text validation lands with the create-rejection cards
// (board/03, board/04) — until then only the schema CHECK bounds it.
func (s *Store) Create(text string) (Card, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Card{}, fmt.Errorf("create card: %w", err)
	}
	defer tx.Rollback() // no-op after Commit

	var position int
	if err := tx.QueryRow(`SELECT count(*) FROM cards WHERE "column" = 'todo'`).Scan(&position); err != nil {
		return Card{}, fmt.Errorf("create card: %w", err)
	}
	res, err := tx.Exec(
		`INSERT INTO cards (title, "column", position) VALUES (?, 'todo', ?)`,
		text, position,
	)
	if err != nil {
		return Card{}, fmt.Errorf("create card: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Card{}, fmt.Errorf("create card: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Card{}, fmt.Errorf("create card: %w", err)
	}
	return Card{ID: id, Title: text, Column: Todo, Position: position}, nil
}

// List returns the board: the three fixed columns in the order todo,
// in_progress, done — all three always present, empty ones holding no cards —
// each column's cards top-to-bottom by position. The ordered read is grouped
// by the module into this fixed shape, so callers never see column order as a
// variable.
func (s *Store) List() ([]ColumnCards, error) {
	board := make([]ColumnCards, 0, len(columns))
	for _, col := range columns {
		rows, err := s.db.Query(
			`SELECT id, title, "column", position FROM cards WHERE "column" = ? ORDER BY position ASC`,
			string(col),
		)
		if err != nil {
			return nil, fmt.Errorf("list board: %w", err)
		}
		var cards []Card
		for rows.Next() {
			var c Card
			if err := rows.Scan(&c.ID, &c.Title, &c.Column, &c.Position); err != nil {
				rows.Close()
				return nil, fmt.Errorf("list board: %w", err)
			}
			cards = append(cards, c)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("list board: %w", err)
		}
		rows.Close()
		board = append(board, ColumnCards{Name: col, Cards: cards})
	}
	return board, nil
}
