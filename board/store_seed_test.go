package board

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// maxID answers the highest identifier the file ever issued (0 on a virgin
// store) — the reference point for "fresh": a seeded card's identifier must
// exceed every identifier assigned before the seed.
func maxID(t *testing.T, store *Store) int64 {
	t.Helper()
	var max int64
	if err := store.db.QueryRow(`SELECT coalesce(max(id), 0) FROM cards`).Scan(&max); err != nil {
		t.Fatalf("max id: %v", err)
	}
	return max
}

// Card board/14 — Import seeding preserves given order.
// Given an empty board
// When  cards are seeded in bulk — a list of texts with a target column
// Then  the column holds exactly those texts top-to-bottom in the given order
//
//	And each card carries a fresh identifier
//
// The table pins the scenario on each target column — the bulk seed is
// column-generic, todo, in_progress and done all take the list the same way —
// plus a single-text list, a longer list whose position order is the given
// order alone (no creation sequence produced it), and the shared text rule
// working through this door too: seeded texts are stored trimmed, exactly as
// Create stores them. Every row pins the column cell-for-cell at contiguous
// positions 0..n-1 and the identifiers fresh: strictly ascending along the
// given order (autoincrement hands them out in insert order, so a column
// listing that is both title-correct and id-ascending proves the stored
// order equals the given order), every one above every identifier issued
// before the seed. Untouched columns must stay empty — a seed writes one
// column.
func TestSeedPreservesGivenOrder(t *testing.T) {
	tests := []struct {
		name   string
		column Column
		texts  []string
		want   []string // the seeded column top-to-bottom, given order
	}{
		{
			"scenario: the given list lands top-to-bottom in todo",
			Todo,
			[]string{"First", "Second", "Third"},
			[]string{"First", "Second", "Third"},
		},
		{
			"into in_progress — the target column is any column",
			InProgress,
			[]string{"alpha", "beta", "gamma", "delta"},
			[]string{"alpha", "beta", "gamma", "delta"},
		},
		{
			"into done — membership is the only done state, seeding included",
			Done,
			[]string{"old report", "shipped release"},
			[]string{"old report", "shipped release"},
		},
		{
			"a single-text list is a whole bulk",
			Todo,
			[]string{"only one"},
			[]string{"only one"},
		},
		{
			"the given order is the only order — eight texts no creation sequence shaped",
			InProgress,
			[]string{"i7", "i3", "i5", "i1", "i8", "i2", "i6", "i4"},
			[]string{"i7", "i3", "i5", "i1", "i8", "i2", "i6", "i4"},
		},
		{
			"same rule, same trimming — texts are stored normalized",
			Todo,
			[]string{"  Buy milk  ", "\tshipped release\n"},
			[]string{"Buy milk", "shipped release"},
		},
		{
			"exactly the limit once padded away — the limit measures trimmed runes",
			Done,
			[]string{" " + strings.Repeat("z", MaxTextLen) + " "},
			[]string{strings.Repeat("z", MaxTextLen)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := openStore(t)
			before := maxID(t, store)

			if err := store.Seed(tt.column, tt.texts); err != nil {
				t.Fatalf("Seed(%q, %q): %v", tt.column, tt.texts, err)
			}

			board := mustList(t, store)
			seeded := board[columnIndex(t, board, tt.column)]
			if len(seeded.Cards) != len(tt.want) {
				t.Fatalf("column %q holds %s, want %v", tt.column, titles(seeded.Cards), tt.want)
			}
			for pos, wantTitle := range tt.want {
				c := seeded.Cards[pos]
				if c.Title != wantTitle || c.Position != pos || c.Column != tt.column {
					t.Errorf("column %q cell %d = %q@%d (column %q), want %q@%d",
						tt.column, pos, c.Title, c.Position, c.Column, wantTitle, pos)
				}
				if c.ID <= before {
					t.Errorf("card %q carries id %d, want fresh — above every id issued before the seed (%d)",
						c.Title, c.ID, before)
				}
				if pos > 0 && seeded.Cards[pos-1].ID >= c.ID {
					t.Errorf("card %q carries id %d, want above the previous cell's %d — ids follow the given order",
						c.Title, c.ID, seeded.Cards[pos-1].ID)
				}
			}
			for _, col := range board {
				if col.Name == tt.column {
					continue
				}
				if len(col.Cards) != 0 {
					t.Errorf("column %q holds %s, want no cards — a seed writes one column",
						col.Name, titles(col.Cards))
				}
			}
			assertContiguous(t, board)
		})
	}
}

// Order preservation across the three columns with a given interleaving: the
// texts interleave globally (t1, p1, d1, t2, ...) and the seed runs one bulk
// per column; each column must hold exactly its own texts in the relative
// order the global list states them. No column sees another's cards, and the
// three together reconstruct the full interleaved set.
func TestSeedInterleavedAcrossThreeColumns(t *testing.T) {
	store := openStore(t)
	if err := store.Seed(Todo, []string{"t1", "t2", "t3"}); err != nil {
		t.Fatalf("Seed(todo): %v", err)
	}
	if err := store.Seed(InProgress, []string{"p1", "p2"}); err != nil {
		t.Fatalf("Seed(in_progress): %v", err)
	}
	if err := store.Seed(Done, []string{"d1", "d2", "d3"}); err != nil {
		t.Fatalf("Seed(done): %v", err)
	}

	board := mustList(t, store)
	assertPlacements(t, board, []placement{
		{"t1", Todo, 0}, {"t2", Todo, 1}, {"t3", Todo, 2},
		{"p1", InProgress, 0}, {"p2", InProgress, 1},
		{"d1", Done, 0}, {"d2", Done, 1}, {"d3", Done, 2},
	})
}

// The given order extends an existing column: cards already in place keep
// theirs, the seeded texts land below them in exactly the list's order, all
// positions stay contiguous, every seeded identifier is fresh above the
// board's current maximum, and the other columns are cell-for-cell untouched.
func TestSeedAppendsBelowExistingKeepingOrder(t *testing.T) {
	store := openStore(t)
	seedBoard(t, store, []columnFixture{
		{Todo, []string{"t0", "t1"}},
		{Done, []string{"d0"}},
	})
	before := mustList(t, store)
	beforeMax := maxID(t, store)

	if err := store.Seed(Todo, []string{"s0", "s1", "s2"}); err != nil {
		t.Fatalf("Seed(todo, s0..s2): %v", err)
	}

	after := mustList(t, store)
	assertPlacements(t, after, []placement{
		{"t0", Todo, 0}, {"t1", Todo, 1},
		{"s0", Todo, 2}, {"s1", Todo, 3}, {"s2", Todo, 4},
		{"d0", Done, 0},
	})
	todo := after[columnIndex(t, after, Todo)]
	for _, c := range todo.Cards[len(todo.Cards)-3:] {
		if c.ID <= beforeMax {
			t.Errorf("seeded card %q carries id %d, want fresh — above the pre-seed maximum %d",
				c.Title, c.ID, beforeMax)
		}
	}
	untouched := after[columnIndex(t, after, InProgress)]
	if len(untouched.Cards) != 0 {
		t.Errorf("in_progress holds %s, want no cards", titles(untouched.Cards))
	}
	doneBefore := before[columnIndex(t, before, Done)]
	doneAfter := after[columnIndex(t, after, Done)]
	if len(doneAfter.Cards) != len(doneBefore.Cards) {
		t.Fatalf("untouched column done holds %s, want %s", titles(doneAfter.Cards), titles(doneBefore.Cards))
	}
	for i := range doneBefore.Cards {
		if doneAfter.Cards[i] != doneBefore.Cards[i] {
			t.Errorf("untouched column done cell %d = %+v, want %+v — a seed writes one column",
				i, doneAfter.Cards[i], doneBefore.Cards[i])
		}
	}
	assertContiguous(t, after)
}

// A rejected entry refuses the whole seed: the card's contract is that the
// column holds exactly the given list, so half a list is not an outcome —
// one bad text anywhere (blank, whitespace-only, over the limit, at any
// index) answers the typed store error naming the entry with nothing
// written: the board cell-for-cell as it was and no identifier consumed,
// exactly as validation-gated mutations behave. The fixed-column enum guard
// answers before the texts are even read, so a bad column wins when both
// are bad — and even an empty list into a bad column is refused, because
// the guard is structural.
func TestSeedRejectionsRefuseWholeSeed(t *testing.T) {
	tests := []struct {
		name   string
		column Column
		texts  []string
		want   error
	}{
		{"blank entry in the middle", Todo,
			[]string{"good one", "", "another"}, ErrTextRequired},
		{"whitespace-only entry last", Todo,
			[]string{"fine", "   \t\n "}, ErrTextRequired},
		{"over-long entry in the middle", InProgress,
			[]string{"ok", strings.Repeat("x", MaxTextLen+1), "ok too"}, ErrTextTooLong},
		{"over-long entry first — no insert happened before the check", Done,
			[]string{strings.Repeat("y", MaxTextLen+1), "ok"}, ErrTextTooLong},
		{"over-long by one character after trimming", Todo,
			[]string{"ok", " " + strings.Repeat("z", MaxTextLen+1)}, ErrTextTooLong},
		{"empty column value", Column(""),
			[]string{"fine texts"}, ErrInvalidColumn},
		{"wrong case column", Column("Todo"),
			[]string{"fine texts"}, ErrInvalidColumn},
		{"hyphen instead of underscore", Column("in-progress"),
			[]string{"fine texts"}, ErrInvalidColumn},
		{"arbitrary junk column", Column("backlog"),
			[]string{"fine texts"}, ErrInvalidColumn},
		{"bad column and bad text together — the enum guard answers first", Column("nope"),
			[]string{"ok", ""}, ErrInvalidColumn},
		{"empty list into a bad column — the guard precedes the no-op", Column("nope"),
			nil, ErrInvalidColumn},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := openStore(t)
			seedBoard(t, store, []columnFixture{{Todo, []string{"resident"}}})
			before := mustList(t, store)
			lastCreated := maxID(t, store)

			err := store.Seed(tt.column, tt.texts)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Seed(%q, %q) err = %v, want %v", tt.column, tt.texts, err, tt.want)
			}
			after := mustList(t, store)
			if !boardEqual(before, after) {
				t.Errorf("rejected seed changed the board:\n before: %s\n after:  %s",
					flatten(before), flatten(after))
			}
			probe, err := store.Create("id probe")
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			if probe.ID != lastCreated+1 {
				t.Errorf("identifier after rejected seed = %d, want %d — a rejected seed must not consume ids",
					probe.ID, lastCreated+1)
			}
		})
	}
}

// An empty list into a valid column is a no-op answered with nil: no rows,
// no position shifts, no identifiers consumed — on an empty board and on a
// board holding cards alike.
func TestSeedEmptyListIsNoOp(t *testing.T) {
	empty := openStore(t)
	if err := empty.Seed(Todo, nil); err != nil {
		t.Errorf("Seed(todo, nil) on an empty board: %v", err)
	}
	if err := empty.Seed(InProgress, []string{}); err != nil {
		t.Errorf("Seed(in_progress, []) on an empty board: %v", err)
	}
	board := mustList(t, empty)
	for _, col := range board {
		if len(col.Cards) != 0 {
			t.Errorf("column %q holds %s after empty seeds, want no cards", col.Name, titles(col.Cards))
		}
	}
	probe, err := empty.Create("first ever")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if probe.ID != 1 {
		t.Errorf("first id after empty seeds = %d, want 1 — a no-op consumes nothing", probe.ID)
	}

	populated := openStore(t)
	seedBoard(t, populated, []columnFixture{
		{Todo, []string{"t0", "t1"}},
		{Done, []string{"d0"}},
	})
	before := mustList(t, populated)
	if err := populated.Seed(Todo, nil); err != nil {
		t.Errorf("Seed(todo, nil) on a populated board: %v", err)
	}
	if after := mustList(t, populated); !boardEqual(before, after) {
		t.Errorf("empty seed changed the board:\n before: %s\n after:  %s",
			flatten(before), flatten(after))
	}
}

// The seeded state is ordinary stored state: a board built entirely through
// Seed survives Close and reopen at the same path cell-for-cell, reusing the
// board/13 machinery — the seed commits through the same schema and writes
// the same rows every other operation reads back.
func TestSeededStateSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "board.db")
	store := openStoreAt(t, path)

	if err := store.Seed(Todo, []string{"m1", "m2", "m3"}); err != nil {
		t.Fatalf("Seed(todo): %v", err)
	}
	if err := store.Seed(InProgress, []string{"m4"}); err != nil {
		t.Fatalf("Seed(in_progress): %v", err)
	}
	if err := store.Seed(Done, []string{"m5", "m6"}); err != nil {
		t.Fatalf("Seed(done): %v", err)
	}
	before := mustList(t, store)
	assertPlacements(t, before, []placement{
		{"m1", Todo, 0}, {"m2", Todo, 1}, {"m3", Todo, 2},
		{"m4", InProgress, 0},
		{"m5", Done, 0}, {"m6", Done, 1},
	})

	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	after := mustList(t, openStoreAt(t, path))
	if !boardEqual(before, after) {
		t.Errorf("reopened seeded board differs from the closed one:\n before: %s\n after:  %s",
			flatten(before), flatten(after))
	}
}
