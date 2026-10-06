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

// validColumn reports membership in the module's fixed column enum. Column is
// a string type, so callers can hand the store anything; every column
// direction passes this guard before storage is touched, making a bad value a
// named store outcome (ErrInvalidColumn) rather than a storage-constraint
// failure — the same guarantee validateText gives the text direction.
func validColumn(c Column) bool {
	for _, known := range columns {
		if c == known {
			return true
		}
	}
	return false
}

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

// ErrCardNotFound reports a change or delete that targets an identifier no
// card holds — the store's "no such card" outcome. The existence check is a
// read inside the transaction that runs before any write, so a rejected
// change or delete leaves the board exactly as it was and consumes no
// identifier (neither operation inserts on the miss). Callers map it with
// errors.Is.
var ErrCardNotFound = errors.New("board: no card with that identifier")

// ErrInvalidColumn reports a column direction naming something outside the
// module's fixed enum (todo, in_progress, done) — the store's "column
// invalid" outcome. The guard runs before the transaction opens, so a
// rejected value touches no storage: the board is exactly as it was. Callers
// map it with errors.Is.
var ErrInvalidColumn = errors.New("board: column must be one of todo, in_progress, done")

// Change applies the given directions to the card identified by id in one
// transaction and returns the card as it now stands. A nil direction is left
// untouched; at least one direction must be non-nil.
//
// The title direction runs through validateText — the module's single text
// rule, the same function Create screens through — BEFORE the transaction
// opens: blank text is ErrTextRequired and over-long text ErrTextTooLong, each
// with no statement executed, so a rejected change leaves the board exactly as
// it was. The stored title is the trimmed text. A title-only change touches
// nothing else: the card keeps its column, its position, and its identifier —
// place and identity survive the rename — and done-column cards are fully
// editable, because no frozen state is stored anywhere (column membership is
// the only done state).
//
// The column direction guards the fixed enum (validColumn) BEFORE the
// transaction opens — a value other than todo, in_progress, done is
// ErrInvalidColumn with no statement executed, never a storage-constraint
// failure — then moves the card to the bottom of the target column and
// closes the gap in the source column, so every column's positions stay
// contiguous 0..n-1 inside the same transaction. A requested column equal to
// the card's current one places nothing — Change never reorders within a
// column here; neighbor-ordering semantics (insert at an index, same-column
// reorder) are dedicated operations in their own cards (board/06, board/07,
// KW5).
//
// An identifier no card holds comes back as ErrCardNotFound: the existence
// read is the transaction's first statement, before any write, so a rejected
// change leaves the board exactly as it was and consumes no identifier.
func (s *Store) Change(id int64, title *string, column *Column) (Card, error) {
	if title == nil && column == nil {
		return Card{}, fmt.Errorf("change card %d: nothing to change (title and column both nil)", id)
	}

	// Validation precedes every write: both directions are screened here —
	// text through validateText, column through the fixed enum — before
	// Begin, before any SQL. A rejection in either direction is a named
	// store outcome with no statement executed.
	trimmed := ""
	if title != nil {
		var err error
		trimmed, err = validateText(*title)
		if err != nil {
			return Card{}, fmt.Errorf("change card %d: %w", id, err)
		}
	}
	if column != nil && !validColumn(*column) {
		return Card{}, fmt.Errorf("change card %d: column %q is not todo, in_progress, or done: %w",
			id, string(*column), ErrInvalidColumn)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return Card{}, fmt.Errorf("change card %d: %w", id, err)
	}
	defer tx.Rollback() // no-op after Commit

	var card Card
	err = tx.QueryRow(
		`SELECT id, title, "column", position FROM cards WHERE id = ?`, id,
	).Scan(&card.ID, &card.Title, &card.Column, &card.Position)
	if errors.Is(err, sql.ErrNoRows) {
		return Card{}, fmt.Errorf("change card %d: %w", id, ErrCardNotFound)
	}
	if err != nil {
		return Card{}, fmt.Errorf("change card %d: %w", id, err)
	}

	if title != nil {
		if _, err := tx.Exec(`UPDATE cards SET title = ? WHERE id = ?`, trimmed, id); err != nil {
			return Card{}, fmt.Errorf("change card %d: %w", id, err)
		}
		card.Title = trimmed
	}

	if column != nil && *column != card.Column {
		var position int
		if err := tx.QueryRow(
			`SELECT count(*) FROM cards WHERE "column" = ?`, string(*column),
		).Scan(&position); err != nil {
			return Card{}, fmt.Errorf("change card %d: %w", id, err)
		}
		if _, err := tx.Exec(
			`UPDATE cards SET "column" = ?, position = ? WHERE id = ?`,
			string(*column), position, id,
		); err != nil {
			return Card{}, fmt.Errorf("change card %d: %w", id, err)
		}
		if err := renormalize(tx, card.Column); err != nil {
			return Card{}, fmt.Errorf("change card %d: %w", id, err)
		}
		card.Column, card.Position = *column, position
	}

	if err := tx.Commit(); err != nil {
		return Card{}, fmt.Errorf("change card %d: %w", id, err)
	}
	return card, nil
}

// renormalize rewrites a column's positions to contiguous 0..n-1 in the
// column's current top-to-bottom order, closing the gap a card leaving the
// column opened. It runs inside the caller's transaction, so the gap closes or
// rolls back with the mutation that caused it — the invariant "positions are
// exactly 0..n-1 per column" is never observable as broken.
func renormalize(tx *sql.Tx, col Column) error {
	rows, err := tx.Query(
		`SELECT id FROM cards WHERE "column" = ? ORDER BY position ASC, id ASC`,
		string(col),
	)
	if err != nil {
		return fmt.Errorf("renormalize column %q: %w", col, err)
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("renormalize column %q: %w", col, err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("renormalize column %q: %w", col, err)
	}
	rows.Close()
	for pos, id := range ids {
		if _, err := tx.Exec(`UPDATE cards SET position = ? WHERE id = ?`, pos, id); err != nil {
			return fmt.Errorf("renormalize column %q: %w", col, err)
		}
	}
	return nil
}

// Delete removes the card identified by id and closes the gap its departure
// opened in the card's former column: the survivors' positions are
// renormalized to contiguous 0..n-1 in their current top-to-bottom order,
// while every other column keeps its cards at their current positions. The
// existence read is the transaction's first statement and strictly precedes
// the DELETE, so an identifier no card holds comes back as ErrCardNotFound
// with no write executed — the board is exactly as it was — the same ordering
// Change uses. The delete and the gap close run in one transaction, so no
// caller ever observes the source column holding a gap. A deleted card's
// identifier is never re-issued: ids come from the table's autoincrement,
// which only counts forward, so a later Create hands out a fresh identifier
// greater than every identifier ever assigned — a stale caller can never
// address a different card.
func (s *Store) Delete(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("delete card %d: %w", id, err)
	}
	defer tx.Rollback() // no-op after Commit

	// Existence check before DELETE: this read is the first statement inside
	// the transaction and its outcome decides everything after it. It does
	// double duty — the column it reports names the column the delete opens a
	// gap in (renormalize's target), and its miss is the named store outcome
	// ErrCardNotFound with no write statement ever executed.
	var col Column
	err = tx.QueryRow(`SELECT "column" FROM cards WHERE id = ?`, id).Scan(&col)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("delete card %d: %w", id, ErrCardNotFound)
	}
	if err != nil {
		return fmt.Errorf("delete card %d: %w", id, err)
	}

	if _, err := tx.Exec(`DELETE FROM cards WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete card %d: %w", id, err)
	}
	if err := renormalize(tx, col); err != nil {
		return fmt.Errorf("delete card %d: %w", id, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("delete card %d: %w", id, err)
	}
	return nil
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
