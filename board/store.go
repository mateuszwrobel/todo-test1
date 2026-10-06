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

// schema is created on open (create-if-not-exists); cards plus the board's
// bookkeeping meta row-store, no migration framework. autoincrement rowids
// are never reused, even after delete. The title CHECK is a storage-level
// belt only: SQLite's trim() strips spaces but not tabs/newlines, so the
// observable validation behavior lives in Go (validateText), which screens
// every candidate before any insert — a constraint failure is never the
// reported outcome.
// "column" is quoted because COLUMN is a reserved word in SQLite.
// The meta table holds the one-time import marker (Imported, MarkImported,
// and Import's atomic marker write): a key/value row whose presence records
// that the board's one-time import decision has been made. The composition
// root's migration guard reads it; the board only stores it.
const schema = `CREATE TABLE IF NOT EXISTS cards (
	id integer primary key autoincrement,
	title text not null check (trim(title) <> '' and length(title) <= 500),
	"column" text not null check ("column" in ('todo','in_progress','done')),
	position integer not null check (position >= 0)
);
CREATE INDEX IF NOT EXISTS cards_column_position ON cards ("column", position);
CREATE TABLE IF NOT EXISTS meta (
	key text primary key,
	value text not null
)`

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
// board (Create, Change, Seed, Import) runs candidate text through it before
// storage is touched, so one rule source covers all paths and a rejection is always
// a named store outcome, never a storage-constraint failure. The rules:
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

// ErrCardNotFound reports a change, move, or delete that targets an
// identifier no card holds — the store's "no such card" outcome. The
// existence check is a read inside the transaction that runs before any
// write, so a rejected operation leaves the board exactly as it was and
// consumes no identifier (none of the three inserts on the miss). Callers map
// it with errors.Is.
var ErrCardNotFound = errors.New("board: no card with that identifier")

// ErrInvalidColumn reports a column value outside the module's fixed enum
// (todo, in_progress, done) — the store's "column invalid" outcome, named by
// Change's column direction and by Move's target alike. The guard runs before
// the transaction opens, so a rejected value touches no storage: the board is
// exactly as it was. Callers map it with errors.Is.
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
// the card's current one places nothing — Change carries no position
// direction, so it never reorders within a column. The neighbor-ordering
// semantics the KW3 note scheduled (insert at an index, same-column reorder)
// arrived as Move, in board/06 and board/07.
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

// Move places the card identified by id at the given index of the target
// column and returns the card as it now stands — one transaction, everything
// decided before the first write or after the last read inside it. This is
// the store's placement operation (board/06, board/07): the card lands at
// exactly index position of column, every other card keeps its relative order
// — the uninvolved neighbors above the index stay above it, those below stay
// below, in their previous order — and a source column the card departs
// closes its gap, so every column's positions stay contiguous 0..n-1.
//
// position indexes the target column AFTER the moved card is removed from it.
// Cross-column (board/06) that is the target as it stands: moving B into
// [X, Y] at 1 yields [X, B, Y]. Same-column (board/07) it is the column minus
// the card, so a move is remove-then-insert: A at position 2 of [A, B, C]
// yields [B, C, A], and a position equal to the card's own index splices it
// back where it was — the column is provably unchanged. Change's column
// direction keeps its bottom-append meaning beside this: Move is the explicit
// index direction, and a position at (or past) the end reproduces exactly
// what Change's append places.
//
// The cards say nothing about out-of-range indexes, so the index clamps into
// the valid insertion range [0, remaining count] instead of failing: negative
// lands at the top, past the end lands at the bottom. Move is a total
// function over positions — one outcome for the drag layer to reason about —
// and the ends of the range are the card's own index-0 and bottom placements.
//
// Validation is board-style: the target column is screened against the fixed
// enum BEFORE the transaction opens (ErrInvalidColumn, no statement
// executed), and the card's existence is the transaction's first read — its
// miss is ErrCardNotFound before any write, so a rejected move touches nothing
// and consumes no identifier. The card's identifier and text are untouched by
// a move — placement is the only thing a move changes.
func (s *Store) Move(id int64, column Column, position int) (Card, error) {
	if !validColumn(column) {
		return Card{}, fmt.Errorf("move card %d: column %q is not todo, in_progress, or done: %w",
			id, string(column), ErrInvalidColumn)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return Card{}, fmt.Errorf("move card %d: %w", id, err)
	}
	defer tx.Rollback() // no-op after Commit

	var card Card
	err = tx.QueryRow(
		`SELECT id, title, "column", position FROM cards WHERE id = ?`, id,
	).Scan(&card.ID, &card.Title, &card.Column, &card.Position)
	if errors.Is(err, sql.ErrNoRows) {
		return Card{}, fmt.Errorf("move card %d: %w", id, ErrCardNotFound)
	}
	if err != nil {
		return Card{}, fmt.Errorf("move card %d: %w", id, err)
	}

	// Departure first for a cross-column move: flipping the column lets the
	// source renormalize see the card as gone and close its gap; the arrival
	// row still carries its old position value, which the placement write
	// below overwrites.
	if column != card.Column {
		if _, err := tx.Exec(`UPDATE cards SET "column" = ? WHERE id = ?`, string(column), id); err != nil {
			return Card{}, fmt.Errorf("move card %d: %w", id, err)
		}
		if err := renormalize(tx, card.Column); err != nil {
			return Card{}, fmt.Errorf("move card %d: %w", id, err)
		}
	}

	// The target's remaining cards top-to-bottom, the moved card excluded —
	// removal precedes insertion in both directions, same-column included.
	rows, err := tx.Query(
		`SELECT id FROM cards WHERE "column" = ? AND id <> ? ORDER BY position ASC, id ASC`,
		string(column), id,
	)
	if err != nil {
		return Card{}, fmt.Errorf("move card %d: %w", id, err)
	}
	var rest []int64
	for rows.Next() {
		var other int64
		if err := rows.Scan(&other); err != nil {
			rows.Close()
			return Card{}, fmt.Errorf("move card %d: %w", id, err)
		}
		rest = append(rest, other)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Card{}, fmt.Errorf("move card %d: %w", id, err)
	}
	rows.Close()

	// Clamp into the valid insertion range, then splice: neighbors keep
	// their relative order around the landing card by construction.
	index := position
	if index < 0 {
		index = 0
	}
	if index > len(rest) {
		index = len(rest)
	}
	ordered := make([]int64, 0, len(rest)+1)
	ordered = append(ordered, rest[:index]...)
	ordered = append(ordered, id)
	ordered = append(ordered, rest[index:]...)

	for pos, placed := range ordered {
		if _, err := tx.Exec(`UPDATE cards SET position = ? WHERE id = ?`, pos, placed); err != nil {
			return Card{}, fmt.Errorf("move card %d: %w", id, err)
		}
	}
	card.Column, card.Position = column, index

	if err := tx.Commit(); err != nil {
		return Card{}, fmt.Errorf("move card %d: %w", id, err)
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

// Seed bulk-places a caller-supplied ordered list of texts into one column —
// the store's seeding operation, the primitive a migration drives (the
// composition root decides WHAT to import; the store only inserts). The
// given order IS the result: the texts land below any cards already in the
// column (a fresh board holds none, which is the scenario's "empty board"),
// in exactly the order the list states them, and positions are assigned in
// that order — the column's first seeded card takes the column's current
// count, the next count+1, and so on — so when the seed commits the column
// still satisfies the module invariant, contiguous 0..n-1 top-to-bottom,
// and the given order is the stored order with nothing between. Each seeded
// card carries a fresh identifier from the same autoincrement Create draws
// from: seeding never imports or preserves identifiers, every card the seed
// writes is new to this board.
//
// Validation is the board's own, and it decides before any write: the target
// column is screened against the fixed enum first (ErrInvalidColumn — the
// guard answers even when the list is empty), then every text runs through
// validateText, the module's single text rule, at its list index. One bad
// entry anywhere refuses the whole seed — ErrTextRequired or ErrTextTooLong
// names the entry — with no statement executed, so the board is exactly as
// it was and no identifier is consumed, the same all-or-nothing contract the
// rejected mutations keep. An accepted seed runs in one transaction (count,
// then one insert per text), so the column gains every card in order or not
// at all. An empty list into a valid column is a no-op answered with nil:
// nothing is read, nothing is written.
func (s *Store) Seed(column Column, texts []string) error {
	if !validColumn(column) {
		return fmt.Errorf("seed column %q is not todo, in_progress, or done: %w",
			string(column), ErrInvalidColumn)
	}
	titles := make([]string, len(texts))
	for i, text := range texts {
		title, err := validateText(text)
		if err != nil {
			return fmt.Errorf("seed entry %d: %w", i, err)
		}
		titles[i] = title
	}
	if len(titles) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("seed column %q: %w", column, err)
	}
	defer tx.Rollback() // no-op after Commit

	var position int
	if err := tx.QueryRow(
		`SELECT count(*) FROM cards WHERE "column" = ?`, string(column),
	).Scan(&position); err != nil {
		return fmt.Errorf("seed column %q: %w", column, err)
	}
	for _, title := range titles {
		if _, err := tx.Exec(
			`INSERT INTO cards (title, "column", position) VALUES (?, ?, ?)`,
			title, string(column), position,
		); err != nil {
			return fmt.Errorf("seed column %q: %w", column, err)
		}
		position++
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("seed column %q: %w", column, err)
	}
	return nil
}

// SeedBatch is one column's slice of an Import: the target column and the
// texts to place in it, top-to-bottom.
type SeedBatch struct {
	Column Column
	Texts  []string
}

// Import bulk-places several ordered lists of texts — one per column — in
// ONE transaction: Seed's multi-column sibling, the primitive a migration
// drives when its material spans columns and the composition root wants the
// whole import to land as a unit (deciding WHAT to import stays in the
// composition root — this is still a plain ordered insert, not a migration
// policy). The given order IS the result per column: each list lands below
// any cards already in its column, in exactly the order the list states,
// with the same placement semantics as Seed — the column's first imported
// card takes the column's count at the moment of arrival, the next count+1,
// and so on — so when the import commits every column still satisfies the
// module invariant, contiguous 0..n-1 top-to-bottom, the imported order
// below any residents. As in Seed, identifiers come from the same
// autoincrement Create draws from: an import never imports or preserves
// identifiers, every card it writes is new to this board. Batches naming the
// same column append below each other in batch order, so the batch list
// alone decides each column's final layout.
//
// Validation is Seed's, widened across the batch dimension, and it all
// decides before any write: every batch's column is screened against the
// fixed enum first (ErrInvalidColumn, naming the batch — the guard answers
// even when the list of texts is empty), then every batch's every text runs
// through validateText, the module's single text rule, at its batch and
// entry index. One bad entry anywhere — bad text or bad column, in any
// batch — refuses the whole import with the typed store error naming the
// batch and entry, with no statement executed, so the board is exactly as it
// was and no identifier is consumed; the column guards answering before any
// text is read means a bad column wins over text faults. An accepted import
// runs in one transaction (each column's count, then one insert per text)
// and commits once: every column gains every card in order, or nothing at
// all. The same transaction writes the one-time import marker (Imported
// reads it): an import that committed is recorded forever, an import that
// never committed — refused, rolled back, or stopped mid-flight — recorded
// nothing, so a migration guard that consults the marker may retry. The
// marker does not gate this operation: Import answers its contract
// regardless of the marker's state, and a successful import leaves the
// marker set exactly as it already was. An empty batches list — or one
// whose batches hold no texts — is a no-op answered with nil: nothing is
// read, nothing is written, not even the marker.
func (s *Store) Import(batches []SeedBatch) error {
	// Enum guard first, across all batches: Seed's "the guard answers
	// before the texts are read" widened to the batch dimension, so a bad
	// column wins over any text fault regardless of batch order.
	for i, batch := range batches {
		if !validColumn(batch.Column) {
			return fmt.Errorf("import batch %d: column %q is not todo, in_progress, or done: %w",
				i, string(batch.Column), ErrInvalidColumn)
		}
	}
	// Then every text through validateText, at its batch and entry index;
	// the trimmed titles are kept so the transaction inserts exactly what
	// the rule normalized, as Seed does.
	titles := make([][]string, len(batches))
	total := 0
	for i, batch := range batches {
		titles[i] = make([]string, len(batch.Texts))
		for j, text := range batch.Texts {
			title, err := validateText(text)
			if err != nil {
				return fmt.Errorf("import batch %d entry %d: %w", i, j, err)
			}
			titles[i][j] = title
		}
		total += len(titles[i])
	}
	if total == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("import: %w", err)
	}
	defer tx.Rollback() // no-op after Commit

	for i, batch := range batches {
		if len(titles[i]) == 0 {
			continue // a batch of no texts reads and writes nothing
		}
		var position int
		if err := tx.QueryRow(
			`SELECT count(*) FROM cards WHERE "column" = ?`, string(batch.Column),
		).Scan(&position); err != nil {
			return fmt.Errorf("import batch %d column %q: %w", i, string(batch.Column), err)
		}
		for _, title := range titles[i] {
			if _, err := tx.Exec(
				`INSERT INTO cards (title, "column", position) VALUES (?, ?, ?)`,
				title, string(batch.Column), position,
			); err != nil {
				return fmt.Errorf("import batch %d column %q: %w", i, string(batch.Column), err)
			}
			position++
		}
	}
	// The marker rides this same transaction: committed cards imply a
	// committed marker, and an import that never committed leaves the
	// marker exactly as it was — the retry arm the migration guard needs.
	// INSERT OR IGNORE keeps the write idempotent for a repeat import.
	if _, err := tx.Exec(
		`INSERT OR IGNORE INTO meta (key, value) VALUES (?, '1')`, importMarkerKey,
	); err != nil {
		return fmt.Errorf("import: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("import: %w", err)
	}
	return nil
}

// importMarkerKey is the meta row that records the board's one-time import
// decision. The name belongs to the storage detail the marker ops hide;
// callers never address it.
const importMarkerKey = "imported"

// Imported reports whether the board's one-time import decision is on
// record — the migration guard's observation, the presence or absence of a
// single meta row. It is the durable answer to "has this board file been
// through the import decision", independent of what the board holds now: a
// board emptied after importing still answers true. The read touches no
// card state; it never fails because the marker is absent — absence is the
// false answer, not an error.
func (s *Store) Imported() (bool, error) {
	var one int
	err := s.db.QueryRow(`SELECT 1 FROM meta WHERE key = ?`, importMarkerKey).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read import marker: %w", err)
	}
	return true, nil
}

// MarkImported records the board's one-time import decision without
// placing any card — the marker write for a first start that ran the
// decision and imported nothing (a source that holds no rows, or none at
// all). The write is idempotent: marking a board that is already marked
// changes nothing and answers nil. Import carries its own marker inside the
// import transaction; this op exists for the decision-complete-without-
// import path, so the guard's answer never depends on card counts.
func (s *Store) MarkImported() error {
	if _, err := s.db.Exec(
		`INSERT OR IGNORE INTO meta (key, value) VALUES (?, '1')`, importMarkerKey,
	); err != nil {
		return fmt.Errorf("mark imported: %w", err)
	}
	return nil
}
