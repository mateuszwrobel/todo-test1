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

	"todo/users" // the roster the card assignee is validated against
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

// Card is a single board record. The fields are the contract's Card model —
// id, title, column, position — plus the card's one optional simulated user.
//
// Assignee is represented as a plain string with the EMPTY STRING meaning
// unassigned, and that choice is deliberate on two counts. First, the roster
// contract (users.IsMember) says the empty string is not a name, so no real
// assignee can ever collide with the sentinel: the representation is exact,
// not lossy. Second, a string keeps Card comparable with ==, which is what
// lets every cell-for-cell pin in this package's tests (boardEqual) compare
// two listings directly.
//
// For the api layer the mapping to the contract's name-or-null is one line:
// Assignee != "" is the name, Assignee == "" is null. The JSON tag is
// omitempty so an unassigned card encodes exactly as it did before assignment
// existed (the field absent); the api lane's contract leg replaces that with
// the explicit null this field maps to.
type Card struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Column   Column `json:"column"`
	Position int    `json:"position"`
	Assignee string `json:"assignee,omitempty"`
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
// assignee is nullable with no CHECK and no foreign key: NULL is unassigned,
// and roster membership is a code rule through the users module, not storage
// data (the cast is built into the program). Open adds the column when an
// older board file lacks it, and every existing row then reads as unassigned.
const schema = `CREATE TABLE IF NOT EXISTS cards (
	id integer primary key autoincrement,
	title text not null check (trim(title) <> '' and length(title) <= 500),
	"column" text not null check ("column" in ('todo','in_progress','done')),
	position integer not null check (position >= 0),
	assignee text
);
CREATE INDEX IF NOT EXISTS cards_column_position ON cards ("column", position);
CREATE TABLE IF NOT EXISTS meta (
	key text primary key,
	value text not null
)`

// ensureAssigneeColumn upgrades a pre-assignment board file in place: cards
// written before KW9 carry no assignee column at all, and Open adds it when
// missing (ALTER TABLE ADD COLUMN of a nullable column with no default, so
// every existing row reads as NULL = unassigned — the whole of the schema
// upgrade, no data rewrite, no row touched). A file created by this code
// already has the column and the check is a no-op read.
func ensureAssigneeColumn(db *sql.DB) error {
	var present int
	if err := db.QueryRow(
		`SELECT count(*) FROM pragma_table_info('cards') WHERE name = 'assignee'`,
	).Scan(&present); err != nil {
		return fmt.Errorf("inspect cards columns: %w", err)
	}
	if present > 0 {
		return nil
	}
	if _, err := db.Exec(`ALTER TABLE cards ADD COLUMN assignee text`); err != nil {
		return fmt.Errorf("add cards.assignee: %w", err)
	}
	return nil
}

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
	if err := ensureAssigneeColumn(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("open data file %q: %w", path, err)
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
// the append is all-or-nothing. A created card starts unassigned: the insert
// names no assignee, so the column stores NULL — assignment happens through
// Change's assignee direction, never through Create.
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

// ErrDoneFrozen reports a title direction OR an assignee direction aimed at a
// card sitting in the Done column — the store's done-frozen outcome, the
// contract-level done freeze (user decision 2026-10-07, superseding the
// pivot's reading that a done card's text is editable; extended to assignment
// by the same date's user decision): a done card changes nothing but its
// column and its existence, and moving it out of Done is the only way to make
// it editable again. The check is the card row read's outcome, made inside
// the transaction strictly before any write, so a refused change leaves the
// board exactly as it was and consumes no identifier (Change inserts nothing
// anyway). Column directions on done cards are unaffected — moving out of
// Done is precisely the unlock, and it unlocks both directions — and Create,
// Seed and Import write fresh rows rather than editing a card, so they stay
// allowed into Done. Setting AND clearing the assignee are both edits: the
// freeze does not grade them. Callers map it with errors.Is.
var ErrDoneFrozen = errors.New("board: card is done; move it out of Done to edit")

// ErrUnknownAssignee reports an assignee name outside the simulated roster —
// the store's unknown-assignee outcome, validated through the users contract
// (users.IsMember), the roster being a code rule rather than stored data. The
// guard runs before the transaction opens, so a rejected name touches no
// storage; and it is the FIRST rule Change consults, outranking the text
// rules, the not-found lookup, and the done freeze: an invalid name is a
// defect of the request itself, and the simulation has rules even where the
// dropdown makes a bad name unselectable. Callers map it with errors.Is.
var ErrUnknownAssignee = errors.New("board: assignee is not on the users roster")

// AssigneeDirection is Change's assignee direction. The nil *AssigneeDirection
// means "no assignee direction — leave the card's assignee alone"; a non-nil
// one either sets the card's one simulated user (Name is a roster name,
// validated through the users contract before anything else is consulted) or
// clears it (Clear true — the card becomes unassigned; Name is then ignored).
// The zero &AssigneeDirection{} is the clear direction, which is why call
// sites read clearest through the constructors below.
//
// The representation this direction writes into Card.Assignee is the same one
// reads out of it: a roster name, or the empty string for unassigned.
// Storing is a NULL column; the read maps NULL back to "".
type AssigneeDirection struct {
	// Name is the roster name to assign. Meaningful only when Clear is false.
	Name string
	// Clear asks for the card's assignee to be removed.
	Clear bool
}

// AssignTo is the set direction: assign the card to roster name. The name is
// not screened here — Change screens it against the users contract as its
// first rule, so one validity source covers every call site.
func AssignTo(name string) *AssigneeDirection { return &AssigneeDirection{Name: name} }

// ClearAssignee is the clear direction: the card ends unassigned.
func ClearAssignee() *AssigneeDirection { return &AssigneeDirection{Clear: true} }

// Change applies the given directions to the card identified by id in one
// transaction and returns the card as it now stands. A nil direction is left
// untouched; at least one direction must be non-nil. The directions are the
// card's text, its column, and — since KW9 — its one simulated user.
//
// The assignee direction carries the card's assignee to a roster name or
// clears it (AssigneeDirection, AssignTo, ClearAssignee). Setting and clearing
// are edits and nothing else: the card keeps its column, its position, and its
// identifier, and a change carrying ONLY an assignee is a complete change in
// its own right. The written value is a roster name or NULL (unassigned);
// Card.Assignee reads NULL back as the empty string.
//
// Rule ORDER inside Change is contract, and the contract's words are these,
// quoted from workplan_board_store.md (Database → Data Flow): "Roster
// membership is validated through the users contract before any write,
// outranking text rules, not-found, and the done freeze. The Done freeze check
// in Change now fires when either the title or the assignee direction is
// present while the card's current column is Done." Concretely, in this
// function's order:
//
//  1. the request-shape check (at least one direction non-nil);
//  2. roster validity through users.IsMember — FIRST, so an unknown assignee
//     answers ErrUnknownAssignee beside blank text, a missing card, AND a done
//     card, every time, decided before Begin (card board/17);
//  3. the text rules through validateText, the same function Create screens
//     through — ErrTextRequired, ErrTextTooLong — with no statement executed;
//  4. the column enum through validColumn — ErrInvalidColumn, no statement
//     executed, never a storage-constraint failure;
//  5. inside the transaction, the existence read — its miss is
//     ErrCardNotFound before any write, so a rejected change leaves the board
//     exactly as it was and consumes no identifier;
//  6. the done freeze — a title OR an assignee direction against a card whose
//     CURRENT column is Done (the row read above, never a requested column) is
//     ErrDoneFrozen before any write.
//
// Everything after that is the writes themselves: title text (trimmed), the
// assignee (name or NULL), and the column move — the card to the bottom of the
// target column with the source gap closed, so every column stays contiguous
// 0..n-1 in the same transaction. A requested column equal to the card's
// current one places nothing: Change carries no position direction, so it
// never reorders within a column; that is Move (board/06, board/07). Column
// membership is the only done state, and it alone freezes the title and
// assignee directions; a column-only move out of Done is not an edit and is
// exactly the unlock, and Move (no title, no assignee) never meets the guard.
func (s *Store) Change(id int64, title *string, column *Column, assignee *AssigneeDirection) (Card, error) {
	if title == nil && column == nil && assignee == nil {
		return Card{}, fmt.Errorf("change card %d: nothing to change (title, column and assignee all nil)", id)
	}

	// Validation precedes every write, in the contract's rank order (see the
	// doc above): roster validity FIRST through the users contract, then the
	// text rule, then the column enum — all of it before Begin, before any
	// SQL. A rejection in any direction is a named store outcome with no
	// statement executed.
	//
	// Clear is exempt from the name check by construction: a direction that
	// clears names nobody, and the empty string is not a roster name (users
	// contract) precisely so no real name can ever be confused with it.
	if assignee != nil && !assignee.Clear && !users.IsMember(assignee.Name) {
		return Card{}, fmt.Errorf("change card %d: assignee %q is not on the users roster: %w",
			id, assignee.Name, ErrUnknownAssignee)
	}
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

	// The existence read, and with it the row's CURRENT column (the freeze's
	// subject) and current assignee (the returned card's, unless this change
	// overwrites it).
	var card Card
	var storedAssignee sql.NullString
	err = tx.QueryRow(
		`SELECT id, title, "column", position, assignee FROM cards WHERE id = ?`, id,
	).Scan(&card.ID, &card.Title, &card.Column, &card.Position, &storedAssignee)
	if errors.Is(err, sql.ErrNoRows) {
		return Card{}, fmt.Errorf("change card %d: %w", id, ErrCardNotFound)
	}
	if err != nil {
		return Card{}, fmt.Errorf("change card %d: %w", id, err)
	}
	card.Assignee = storedAssignee.String

	// Contract-level done freeze (user decision 2026-10-07, extended to the
	// assignee direction the same date): a title direction OR an assignee
	// direction aimed at a card that currently sits in Done is refused, before
	// any write. Ordering, quoted from the workplan amendment: the freeze is
	// checked against the card's CURRENT column — the row read above, the
	// transaction's first statement — not against any requested one. So a
	// combined title+column (or assignee+column) change that would carry the
	// card out of Done still refuses while it sits there; a column-only move
	// out of Done is the allowed unlock for both directions, and Move (no
	// title, no assignee involved) never meets this guard. Setting and
	// clearing are both edits — the freeze does not grade them.
	if (title != nil || assignee != nil) && card.Column == Done {
		return Card{}, fmt.Errorf("change card %d: %w", id, ErrDoneFrozen)
	}

	if title != nil {
		if _, err := tx.Exec(`UPDATE cards SET title = ? WHERE id = ?`, trimmed, id); err != nil {
			return Card{}, fmt.Errorf("change card %d: %w", id, err)
		}
		card.Title = trimmed
	}

	if assignee != nil {
		// Clear wins over a carried Name (documented on the direction type):
		// the write is NULL, the representation's unassigned.
		next := ""
		var stored any // a nil bind parameter stores SQL NULL
		if !assignee.Clear {
			next = assignee.Name
			stored = next
		}
		if _, err := tx.Exec(`UPDATE cards SET assignee = ? WHERE id = ?`, stored, id); err != nil {
			return Card{}, fmt.Errorf("change card %d: %w", id, err)
		}
		card.Assignee = next
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
// and consumes no identifier. The card's identifier, text and assignee are
// untouched by a move — placement is the only thing a move changes, so an
// assigned card arrives at its new slot still assigned to the same user (the
// returned card carries the assignee through for the caller's benefit).
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
	var storedAssignee sql.NullString
	err = tx.QueryRow(
		`SELECT id, title, "column", position, assignee FROM cards WHERE id = ?`, id,
	).Scan(&card.ID, &card.Title, &card.Column, &card.Position, &storedAssignee)
	if errors.Is(err, sql.ErrNoRows) {
		return Card{}, fmt.Errorf("move card %d: %w", id, ErrCardNotFound)
	}
	if err != nil {
		return Card{}, fmt.Errorf("move card %d: %w", id, err)
	}
	card.Assignee = storedAssignee.String

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
// variable. Each card surfaces its assignee in the Card.Assignee
// representation (roster name, empty string = unassigned) — the read the
// contract's per-card "assignee" field maps to name-or-null from.
func (s *Store) List() ([]ColumnCards, error) {
	board := make([]ColumnCards, 0, len(columns))
	for _, col := range columns {
		rows, err := s.db.Query(
			`SELECT id, title, "column", position, assignee FROM cards WHERE "column" = ? ORDER BY position ASC`,
			string(col),
		)
		if err != nil {
			return nil, fmt.Errorf("list board: %w", err)
		}
		var cards []Card
		for rows.Next() {
			var c Card
			var storedAssignee sql.NullString
			if err := rows.Scan(&c.ID, &c.Title, &c.Column, &c.Position, &storedAssignee); err != nil {
				rows.Close()
				return nil, fmt.Errorf("list board: %w", err)
			}
			c.Assignee = storedAssignee.String
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
