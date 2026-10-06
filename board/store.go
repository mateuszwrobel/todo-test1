// Package board owns the kanban board's durable state: the card collection,
// the three fixed columns, per-column ordering, and persistence. It is a flat
// record store over one SQLite table (ADR-002 conventions carried over) — the
// domain is one record type plus one invariant: within each column, positions
// are exactly 0..n-1, top-to-bottom = priority.
package board

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

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
// title CHECK is a storage-level belt only: SQLite's trim() strips spaces but
// not tabs/newlines, so the observable validation behavior lives in Go
// (validateText), which screens every candidate before any insert — a
// constraint failure is never the reported outcome.
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
// a fresh store-assigned identifier, and returns the Card. The text is trimmed
// and screened by the module's text rule before storage is touched: blank text
// comes back as ErrTextRequired and over-long text as ErrTextTooLong, each
// with no insert attempted, so a rejected card neither changes the board nor
// consumes an identifier. The count and the insert run in one transaction, so
// the append is all-or-nothing.
func (s *Store) Create(text string) (Card, error) {
	title, err := validateText(text)
	if err != nil {
		return Card{}, fmt.Errorf("create card: %w", err)
	}

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
		title, position,
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
	return Card{ID: id, Title: title, Column: Todo, Position: position}, nil
}

// ErrTextRequired reports card text that is empty or whitespace-only once
// trimmed — the store's "required" outcome. Callers map it with errors.Is.
var ErrTextRequired = errors.New("board: card text is required (empty or whitespace-only)")

// MaxTextLen is the card text limit in characters — counted as Unicode code
// points (runes), not bytes.
const MaxTextLen = 500

// ErrTextTooLong reports trimmed card text longer than MaxTextLen characters —
// the store's "limit exceeded" outcome. Callers map it with errors.Is.
var ErrTextTooLong = fmt.Errorf("board: card text exceeds the %d character limit", MaxTextLen)

// validateText is the module's single text rule: every entry point into the
// board (Create now, Seed with board/14) runs candidate text through it before
// storage is touched, so one rule source covers all paths and a rejection is
// always a named store outcome, never a storage-constraint failure. The rules:
// non-blank once trimmed, at most MaxTextLen characters (runes) once trimmed.
// It returns the normalized (trimmed) text to store.
func validateText(text string) (string, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", ErrTextRequired
	}
	if utf8.RuneCountInString(trimmed) > MaxTextLen {
		return "", ErrTextTooLong
	}
	return trimmed, nil
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
