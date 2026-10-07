package board

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"todo/users"
)

// Card board/15 — Assigning and clearing the assignee keeps the card.
// Given a card on the board
// When  the card is changed with an assignee set to a roster name
// Then  the returned card carries that assignee
//
//	And its column, position, and identifier are unchanged
//
// When  the card is later changed with the assignee cleared
// Then  the returned card carries no assignee
//
//	And its column, position, and identifier are unchanged
//
// The assignee direction is an edit and NOTHING ELSE: place and identity
// survive it exactly as a rename's do, so every cell beside the target is
// pinned unchanged (withAssignee builds the expected listing — the target
// cell's assignee is the one field allowed to differ) and the target's column,
// position and identifier are pinned equal on the returned Card and through
// List. The unassigned state is the empty string (Card.Assignee's documented
// representation), which is what ClearAssignee writes and what a fresh card
// already carries.
func TestAssigneeSetAndClearKeepTheCard(t *testing.T) {
	store := openStore(t)
	seedBoard(t, store, []columnFixture{
		{Todo, []string{"t0", "t1"}},
		{InProgress, []string{"p0"}},
		{Done, []string{"d0"}},
	})

	before := mustList(t, store)
	target := before[columnIndex(t, before, InProgress)].Cards[0]

	t.Run("set to a roster name keeps place and identity", func(t *testing.T) {
		changed, err := store.Change(target.ID, nil, nil, AssignTo("Grace"))
		if err != nil {
			t.Fatalf("Change(%d, nil, nil, AssignTo(\"Grace\")): %v", target.ID, err)
		}
		if changed.Assignee != "Grace" {
			t.Errorf("returned Assignee = %q, want \"Grace\"", changed.Assignee)
		}
		assertKeepsPlace(t, changed, target)

		want := withAssignee(before, target.ID, "Grace")
		if after := mustList(t, store); !boardEqual(after, want) {
			t.Errorf("board after the set:\n got:  %s\n want: %s", flatten(after), flatten(want))
		}
	})

	t.Run("a later set replaces the assignee — a card carries at most one", func(t *testing.T) {
		changed, err := store.Change(target.ID, nil, nil, AssignTo("Linus"))
		if err != nil {
			t.Fatalf("Change(%d, nil, nil, AssignTo(\"Linus\")): %v", target.ID, err)
		}
		if changed.Assignee != "Linus" {
			t.Errorf("returned Assignee = %q, want \"Linus\" (the set direction replaces)", changed.Assignee)
		}
		assertKeepsPlace(t, changed, target)
	})

	t.Run("clearing keeps place and identity", func(t *testing.T) {
		changed, err := store.Change(target.ID, nil, nil, ClearAssignee())
		if err != nil {
			t.Fatalf("Change(%d, nil, nil, ClearAssignee()): %v", target.ID, err)
		}
		if changed.Assignee != "" {
			t.Errorf("returned Assignee = %q, want \"\" (unassigned)", changed.Assignee)
		}
		assertKeepsPlace(t, changed, target)

		if after := mustList(t, store); !boardEqual(after, before) {
			t.Errorf("board after the clear should equal the opening board:\n got:  %s\n want: %s",
				flatten(after), flatten(before))
		}
	})

	t.Run("an assignee direction beside a column move lands both", func(t *testing.T) {
		moved, err := store.Change(target.ID, nil, ptr(Done), AssignTo("Ada"))
		if err != nil {
			t.Fatalf("Change(%d, nil, Done, AssignTo(\"Ada\")): %v", target.ID, err)
		}
		if moved.Assignee != "Ada" {
			t.Errorf("returned Assignee = %q, want \"Ada\"", moved.Assignee)
		}
		if moved.ID != target.ID || moved.Title != target.Title {
			t.Errorf("moved card = %+v, want identity and text of %+v kept", moved, target)
		}
		if moved.Column != Done || moved.Position != 1 {
			t.Errorf("moved card at %q position %d, want done bottom (position 1)", moved.Column, moved.Position)
		}
	})

	t.Run("Move is unaffected — placement alone never touches the assignee", func(t *testing.T) {
		back, err := store.Move(target.ID, InProgress, 0)
		if err != nil {
			t.Fatalf("Move(%d, InProgress, 0): %v", target.ID, err)
		}
		if back.Assignee != "Ada" {
			t.Errorf("card after Move Assignee = %q, want \"Ada\" — a move places, it does not assign", back.Assignee)
		}
	})

	t.Run("every roster name is assignable — validity is the users contract", func(t *testing.T) {
		for _, name := range users.Names() {
			changed, err := store.Change(target.ID, nil, nil, AssignTo(name))
			if err != nil {
				t.Errorf("AssignTo(%q): %v — every roster name is a legal assignee", name, err)
			}
			if changed.Assignee != name {
				t.Errorf("returned Assignee = %q, want %q", changed.Assignee, name)
			}
		}
	})
}

// assertKeepsPlace pins the assignee direction's "nothing else moved" half:
// identifier, column and position equal to the card as it stood.
func assertKeepsPlace(t *testing.T, got, was Card) {
	t.Helper()
	if got.ID != was.ID || got.Column != was.Column || got.Position != was.Position {
		t.Errorf("card after the assignee change = %+v, want %+v's place and identity kept", got, was)
	}
}

// withAssignee copies a listing with one card's assignee replaced — the
// expected board for an assignee edit: the target cell's assignee is the only
// field the change may move.
func withAssignee(board []ColumnCards, id int64, assignee string) []ColumnCards {
	out := make([]ColumnCards, len(board))
	for i, col := range board {
		cards := make([]Card, len(col.Cards))
		for j, c := range col.Cards {
			if c.ID == id {
				c.Assignee = assignee
			}
			cards[j] = c
		}
		out[i] = ColumnCards{Name: col.Name, Cards: cards}
	}
	return out
}

// Card board/16 — Assignment on a done card is frozen.
// Given a card in the Done column
// When  a change carrying an assignee targets it
// Then  the freeze error answers and nothing is written
//
//	And the same card moved out of Done accepts the assignee change afterwards
//
// The KW8 freeze extended to the assignee direction (user decision
// 2026-10-07): a Done card changes nothing but its column and its existence.
// Setting AND clearing are both edits — the freeze does not grade them — and
// the check reads the card's CURRENT column, so carrying the card out of Done
// in the same call does not sneak the edit through. The unlock is the
// column-only move out, exactly as for the title direction, and it unlocks
// both. ErrDoneFrozen is the same typed outcome the title leg answers with.
func TestAssigneeOnDoneCardIsFrozen(t *testing.T) {
	t.Run("set direction on a done card refuses with the board unchanged", func(t *testing.T) {
		store, target := frozenFixture(t)
		before := mustList(t, store)

		got, err := store.Change(target.ID, nil, nil, AssignTo("Grace"))
		if !errors.Is(err, ErrDoneFrozen) {
			t.Fatalf("Change on a done card: err = %v, want ErrDoneFrozen", err)
		}
		if got != (Card{}) {
			t.Errorf("Change returned %+v on frozen refusal, want the zero Card", got)
		}
		if after := mustList(t, store); !boardEqual(before, after) {
			t.Errorf("frozen refusal changed the board:\n before: %s\n after:  %s",
				flatten(before), flatten(after))
		}
	})

	t.Run("clear direction on a done card is an edit too — frozen", func(t *testing.T) {
		store := openStore(t)
		created, err := store.Create("headed out")
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if _, err := store.Change(created.ID, nil, nil, AssignTo("Grace")); err != nil {
			t.Fatalf("assign before the move: %v", err)
		}
		// Into Done is a column direction, not an edit — allowed (board/08's
		// ruling), and the assignee rides along.
		if _, err := store.Change(created.ID, nil, ptr(Done), nil); err != nil {
			t.Fatalf("column move into Done: %v", err)
		}
		before := mustList(t, store)

		got, err := store.Change(created.ID, nil, nil, ClearAssignee())
		if !errors.Is(err, ErrDoneFrozen) {
			t.Fatalf("clear direction on a done card: err = %v, want ErrDoneFrozen", err)
		}
		if got != (Card{}) {
			t.Errorf("Change returned %+v on frozen refusal, want the zero Card", got)
		}
		after := mustList(t, store)
		if !boardEqual(before, after) {
			t.Errorf("frozen clear changed the board:\n before: %s\n after:  %s",
				flatten(before), flatten(after))
		}
		// The refused clear wrote nothing: the card sits in Done still
		// assigned — "nothing is written" pinned on the assignee itself.
		if still := after[columnIndex(t, after, Done)].Cards[0]; still.Assignee != "Grace" {
			t.Errorf("done card after the refused clear Assignee = %q, want \"Grace\"", still.Assignee)
		}
	})

	t.Run("title plus assignee together on a done card is frozen", func(t *testing.T) {
		store, target := frozenFixture(t)
		before := mustList(t, store)

		got, err := store.Change(target.ID, ptr("renamed?"), nil, AssignTo("Ada"))
		if !errors.Is(err, ErrDoneFrozen) {
			t.Fatalf("title+assignee on a done card: err = %v, want ErrDoneFrozen", err)
		}
		if got != (Card{}) {
			t.Errorf("Change returned %+v on frozen refusal, want the zero Card", got)
		}
		if after := mustList(t, store); !boardEqual(before, after) {
			t.Errorf("frozen refusal changed the board:\n before: %s\n after:  %s",
				flatten(before), flatten(after))
		}
	})

	t.Run("assignee plus column out of Done refuses — the freeze reads the CURRENT column", func(t *testing.T) {
		store, target := frozenFixture(t)
		before := mustList(t, store)

		if _, err := store.Change(target.ID, nil, ptr(Todo), AssignTo("Ada")); !errors.Is(err, ErrDoneFrozen) {
			t.Fatalf("assignee+column escape from Done: err = %v, want ErrDoneFrozen", err)
		}
		if after := mustList(t, store); !boardEqual(before, after) {
			t.Errorf("refused escape moved the board:\n before: %s\n after:  %s",
				flatten(before), flatten(after))
		}
	})

	t.Run("moved out of Done, the card accepts the assignee change afterwards", func(t *testing.T) {
		store, target := frozenFixture(t)

		moved, err := store.Change(target.ID, nil, ptr(Todo), nil)
		if err != nil {
			t.Fatalf("column-only Change out of Done: %v — a move is not an edit", err)
		}
		if moved.Assignee != "" {
			t.Fatalf("moved card carries assignee %q, want unassigned", moved.Assignee)
		}

		assigned, err := store.Change(moved.ID, nil, nil, AssignTo("Alan"))
		if err != nil {
			t.Fatalf("assign after the move-out: %v — moving out of Done must unlock the assignee direction", err)
		}
		if assigned.Assignee != "Alan" {
			t.Errorf("card after unlock Assignee = %q, want \"Alan\"", assigned.Assignee)
		}
		if assigned.ID != target.ID || assigned.Column != Todo || assigned.Position != 1 {
			t.Errorf("unlocked card = %+v, want identity kept at todo position 1", assigned)
		}
	})
}

// Card board/17 — Unknown assignee is rejected.
// Given any card
// When  a change carries an assignee outside the roster
// Then  the unknown-assignee error answers before any write — roster validity
//
//	    outranks the text rules, the not-found lookup, and the freeze
//	And the board is unchanged cell-for-cell
//
// The rank is what this test pins: the roster check is Change's FIRST rule, so
// an unknown name answers ErrUnknownAssignee beside blank text, an over-long
// name field beside a valid one, a missing card, an invalid column, and a card
// sitting in Done — the same outcome, every time, with nothing written. The
// name set comes from the users contract, so every case variant, padded and
// empty spelling is outside it; the closing leg proves the roster's five names
// are exactly where the boundary sits.
func TestUnknownAssigneeIsRejected(t *testing.T) {
	unknownNames := []string{"grace", "GRACE", "Ada ", " ada", "Zoidberg", "Ada,Grace", "", "\u00e9clair", "todo"}

	for _, name := range unknownNames {
		t.Run("unknown name "+name, func(t *testing.T) {
			store, target := frozenFixture(t) // {todo: t0} + {done: d0, d1}
			before := mustList(t, store)

			legs := []struct {
				label    string
				id       int64
				title    *string
				column   *Column
				assignee *AssigneeDirection
			}{
				{"beside blank text — outranks the text rules", target.ID, ptr(""), nil, AssignTo(name)},
				{"beside over-long text — outranks the text rules", target.ID, ptr(longTitle()), nil, AssignTo(name)},
				{"beside an invalid column — a request defect wins", target.ID, nil, ptr(Column("backlog")), AssignTo(name)},
				{"on a done card — outranks the freeze", target.ID, nil, nil, AssignTo(name)},
				{"on a missing card — outranks not-found", unknownID(t, store), nil, nil, AssignTo(name)},
			}
			for _, leg := range legs {
				got, err := store.Change(leg.id, leg.title, leg.column, leg.assignee)
				if !errors.Is(err, ErrUnknownAssignee) {
					t.Fatalf("%s: Change err = %v, want ErrUnknownAssignee", leg.label, err)
				}
				if got != (Card{}) {
					t.Errorf("%s: returned %+v, want the zero Card", leg.label, got)
				}
				if after := mustList(t, store); !boardEqual(before, after) {
					t.Errorf("%s: refused change moved the board:\n before: %s\n after:  %s",
						leg.label, flatten(before), flatten(after))
				}
			}
		})
	}

	t.Run("the roster boundary is exact — every member name is accepted", func(t *testing.T) {
		store := openStore(t)
		created, err := store.Create("boundary")
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		for _, name := range users.Names() {
			if _, err := store.Change(created.ID, nil, nil, AssignTo(name)); err != nil {
				t.Errorf("AssignTo(%q): %v — the five roster names are the whole accepted set", name, err)
			}
		}
	})
}

// longTitle is one character over the store's text limit — the text-rule
// refusal the unknown assignee is pinned to outrank.
func longTitle() string {
	long := make([]rune, MaxTextLen+1)
	for i := range long {
		long[i] = 'x'
	}
	return string(long)
}

// Card board/18 — Assignment survives store reopen.
// Given a board with assigned and unassigned cards
// When  the store is closed and reopened
// Then  every card carries exactly the assignee it carried before
//
// Two legs. The first builds the mix through the public operations — created
// and left unassigned, assigned then moved (the assignee rides a move),
// assigned then moved INTO Done (the column direction is no edit), seeded
// (seeding writes unassigned rows) — takes the listing before Close and
// re-takes it after Open at the same path, cell-for-cell: Card is comparable
// and Assignee is part of the comparison, so one equality per cell pins title,
// column, position, identifier and assignee together. The second leg proves
// the schema upgrade beside the survival: a board file written before the
// assignee column existed opens with the column added, every existing row
// reading as unassigned, and such a board assigns and persists like any other.
func TestAssigneeSurvivesStoreReopen(t *testing.T) {
	t.Run("assigned and unassigned cards come back with their assignees", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "board.db")
		store := openStoreAt(t, path)

		mixed := []string{"mix a", "mix b", "mix c", "mix d"}
		for _, text := range mixed {
			if _, err := store.Create(text); err != nil {
				t.Fatalf("Create(%q): %v", text, err)
			}
		}
		if err := store.Seed(Done, []string{"seeded done"}); err != nil {
			t.Fatalf("Seed(Done, ...): %v", err)
		}
		list := mustList(t, store)
		byTitle := map[string]Card{}
		for _, col := range list {
			for _, c := range col.Cards {
				byTitle[c.Title] = c
			}
		}
		// "mix b" assigned and left in todo; "mix c" assigned then moved
		// across columns; "mix d" assigned then moved into Done; "mix a" and
		// the seeded card stay unassigned.
		if _, err := store.Change(byTitle["mix b"].ID, nil, nil, AssignTo("Ada")); err != nil {
			t.Fatalf("assign mix b: %v", err)
		}
		if _, err := store.Change(byTitle["mix c"].ID, nil, nil, AssignTo("Barbara")); err != nil {
			t.Fatalf("assign mix c: %v", err)
		}
		if _, err := store.Move(byTitle["mix c"].ID, InProgress, 0); err != nil {
			t.Fatalf("move mix c: %v", err)
		}
		if _, err := store.Change(byTitle["mix d"].ID, nil, nil, AssignTo("Linus")); err != nil {
			t.Fatalf("assign mix d: %v", err)
		}
		if _, err := store.Change(byTitle["mix d"].ID, nil, ptr(Done), nil); err != nil {
			t.Fatalf("move mix d into Done: %v", err)
		}

		before := mustList(t, store)
		if err := store.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		reopened := openStoreAt(t, path)
		after := mustList(t, reopened)
		if !boardEqual(before, after) {
			t.Errorf("reopened board differs cell-for-cell:\n before: %s\n after:  %s",
				flatten(before), flatten(after))
		}
		// The mix is provably non-trivial: at least one assigned and one
		// unassigned card survive side by side.
		var assigned, unassigned int
		for _, col := range after {
			for _, c := range col.Cards {
				if c.Assignee == "" {
					unassigned++
				} else {
					assigned++
				}
			}
		}
		if assigned == 0 || unassigned == 0 {
			t.Fatalf("board after reopen: %d assigned, %d unassigned — want both", assigned, unassigned)
		}
	})

	t.Run("a board file written without the assignee column opens as unassigned", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "legacy.db")
		writePreAssignmentBoard(t, path)

		store := openStoreAt(t, path)
		list := mustList(t, store)
		assertPlacements(t, list, []placement{
			{"old a", Todo, 0},
			{"old b", Todo, 1},
			{"old done", Done, 0},
		})
		for _, col := range list {
			for _, c := range col.Cards {
				if c.Assignee != "" {
					t.Errorf("legacy card %q carries assignee %q, want \"\" — every existing row reads as unassigned",
						c.Title, c.Assignee)
				}
			}
		}

		// And the upgraded file behaves like any other: assign, close, reopen,
		// the assignee is stored.
		target := list[columnIndex(t, list, Todo)].Cards[0]
		if _, err := store.Change(target.ID, nil, nil, AssignTo("Grace")); err != nil {
			t.Fatalf("assign on a legacy board: %v", err)
		}
		if err := store.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		reopened := openStoreAt(t, path)
		after := mustList(t, reopened)
		got := after[columnIndex(t, after, Todo)].Cards[0]
		if got.Assignee != "Grace" {
			t.Errorf("legacy card after assign and reopen Assignee = %q, want \"Grace\"", got.Assignee)
		}
		if other := after[columnIndex(t, after, Todo)].Cards[1]; other.Assignee != "" {
			t.Errorf("sibling legacy card Assignee = %q, want \"\"", other.Assignee)
		}
	})
}

// writePreAssignmentBoard hand-creates a board file in the schema BEFORE the
// assignee column existed — the exact pre-KW9 cards table, no ALTER, written
// straight through SQL — so the reopen leg proves Open's in-place upgrade
// rather than trusting the current schema string.
func writePreAssignmentBoard(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open legacy file %q: %v", path, err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE cards (
	id integer primary key autoincrement,
	title text not null check (trim(title) <> '' and length(title) <= 500),
	"column" text not null check ("column" in ('todo','in_progress','done')),
	position integer not null check (position >= 0)
)`); err != nil {
		t.Fatalf("create pre-assignment cards table: %v", err)
	}
	seed := []struct {
		title    string
		column   Column
		position int
	}{
		{"old a", Todo, 0}, {"old b", Todo, 1}, {"old done", Done, 0},
	}
	for _, row := range seed {
		if _, err := db.Exec(`INSERT INTO cards (title, "column", position) VALUES (?, ?, ?)`,
			row.title, string(row.column), row.position); err != nil {
			t.Fatalf("seed legacy row %+v: %v", row, err)
		}
	}
}
