package board

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// Card board/08 — Title change keeps place and identity.
// Given a board holding a card in in_progress at position 1 with identifier N
// When  the card's text is changed
// Then  the card shows the new text in in_progress at position 1 with identifier N
//
// The scenario pins the in_progress-at-1 witness; the table checks the general
// contract — any column, any position. The former done row ("done column
// stays fully editable") flipped to frozen legs under the 2026-10-07
// contract-level done freeze (card board/08 amendment: title edits refused
// on cards in Done) — see TestChangeTitleOnDoneCardIsFrozen; column
// membership is still the only done state, and it alone freezes the text.
// Place and identity are pinned through List, cell by cell: only the changed
// card's title may differ, so neighbors provably stay where they were.
func TestChangeTitleKeepsPlaceAndIdentity(t *testing.T) {
	tests := []struct {
		name    string
		columns []columnFixture
		pick    Column // column of the card whose title changes
		pickPos int    // its position within that column
		newText string
	}{
		{
			"scenario: in_progress at position 1",
			[]columnFixture{
				{Todo, []string{"t0", "t1"}},
				{InProgress, []string{"p0", "p1", "p2"}},
				{Done, []string{"d0"}},
			},
			InProgress, 1, "renamed in place",
		},
		{
			"todo column, top card",
			[]columnFixture{
				{Todo, []string{"t0", "t1"}},
			},
			Todo, 0, "renamed top",
		},
		// (The done row of this table — "done column stays fully editable" —
		// flipped to the frozen legs of TestChangeTitleOnDoneCardIsFrozen
		// under the 2026-10-07 done freeze; card board/08 amendment.)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := openStore(t)
			seedBoard(t, store, tt.columns)

			before := mustList(t, store)
			target := before[columnIndex(t, before, tt.pick)].Cards[tt.pickPos]

			changed, err := store.Change(target.ID, ptr(tt.newText), nil)
			if err != nil {
				t.Fatalf("Change(%d, %q, nil): %v", target.ID, tt.newText, err)
			}
			if changed.ID != target.ID {
				t.Errorf("returned identifier = %d, want %d (identity kept)", changed.ID, target.ID)
			}
			if changed.Title != tt.newText {
				t.Errorf("returned Title = %q, want %q", changed.Title, tt.newText)
			}
			if changed.Column != tt.pick {
				t.Errorf("returned Column = %q, want %q (place kept)", changed.Column, tt.pick)
			}
			if changed.Position != tt.pickPos {
				t.Errorf("returned Position = %d, want %d (place kept)", changed.Position, tt.pickPos)
			}

			after := mustList(t, store)
			if len(after) != len(before) {
				t.Fatalf("column count changed: %d before, %d after", len(before), len(after))
			}
			for i := range before {
				if after[i].Name != before[i].Name {
					t.Fatalf("column %d changed: %q -> %q", i, before[i].Name, after[i].Name)
				}
				if len(after[i].Cards) != len(before[i].Cards) {
					t.Fatalf("column %q holds %d cards before, %d after",
						before[i].Name, len(before[i].Cards), len(after[i].Cards))
				}
				for j := range before[i].Cards {
					want := before[i].Cards[j]
					if want.ID == target.ID {
						want.Title = tt.newText
					}
					if after[i].Cards[j] != want {
						t.Errorf("cell %q position %d = %+v, want %+v — a title change moves only the one text",
							before[i].Name, j, after[i].Cards[j], want)
					}
				}
			}
		})
	}
}

// Card board/08 — Amendment 2026-10-07 (contract-level done freeze, user
// decision; card text appended: title edits refused on cards in Done; edit
// affordance absent on Done cards — the affordance half belongs to api/05 and
// ui/06). The frozen legs this test now pins flipped from the old
// done-editable row of TestChangeTitleKeepsPlaceAndIdentity. The contract,
// quoted from the parent workplan scenario:
//
// Given a card in the "Done" column
// When  the user submits a text change for it
// Then  the change is refused with a stated error and the board is unchanged
//
//	And the card becomes editable after it is moved out of "Done"
//
// The freeze reads the card's CURRENT column (the amendment's ordering): a
// combined title+column change from Done refuses too — the way to edit is
// move-out first, edit after. Column-only moves are not edits and never meet
// the guard, and writing FRESH text into Done (Seed, Create's target, Import)
// is not a title edit either.
func TestChangeTitleOnDoneCardIsFrozen(t *testing.T) {
	t.Run("title direction on a done card refuses with the board cell-for-cell unchanged", func(t *testing.T) {
		store, target := frozenFixture(t)
		before := mustList(t, store)

		got, err := store.Change(target.ID, ptr("frozen?"), nil)
		if !errors.Is(err, ErrDoneFrozen) {
			t.Fatalf("Change(%d, title, nil) on a done card: err = %v, want ErrDoneFrozen", target.ID, err)
		}
		if got != (Card{}) {
			t.Errorf("Change returned %+v on frozen refusal, want the zero Card", got)
		}
		after := mustList(t, store)
		if !boardEqual(before, after) {
			t.Errorf("frozen refusal changed the board:\n before: %s\n after:  %s",
				flatten(before), flatten(after))
		}
	})

	t.Run("title+column together from done refuses — the freeze reads the CURRENT column", func(t *testing.T) {
		store, target := frozenFixture(t)
		before := mustList(t, store)

		got, err := store.Change(target.ID, ptr("escaped with a title?"), ptr(Todo))
		if !errors.Is(err, ErrDoneFrozen) {
			t.Fatalf("Change(%d, title, Todo) on a done card: err = %v, want ErrDoneFrozen", target.ID, err)
		}
		if got != (Card{}) {
			t.Errorf("Change returned %+v on frozen refusal, want the zero Card", got)
		}
		after := mustList(t, store)
		if !boardEqual(before, after) {
			t.Errorf("frozen refusal of the combined change moved the board:\n before: %s\n after:  %s",
				flatten(before), flatten(after))
		}
	})

	t.Run("column-only move out of done is fine and unlocks the title direction", func(t *testing.T) {
		store, target := frozenFixture(t)

		moved, err := store.Change(target.ID, nil, ptr(Todo))
		if err != nil {
			t.Fatalf("column-only Change out of Done: %v — a move is not an edit", err)
		}
		if moved.ID != target.ID || moved.Title != target.Title {
			t.Errorf("moved card = %+v, want identity and text of %+v kept", moved, target)
		}
		if moved.Column != Todo || moved.Position != 1 { // bottom of the one-card todo column
			t.Errorf("moved card at %q position %d, want todo bottom (position 1)", moved.Column, moved.Position)
		}

		renamed, err := store.Change(moved.ID, ptr("editable after unlock"), nil)
		if err != nil {
			t.Fatalf("Change(title) after the move-out: %v — moving out of Done must unlock editing", err)
		}
		if renamed.Title != "editable after unlock" || renamed.Column != Todo ||
			renamed.Position != 1 || renamed.ID != target.ID {
			t.Errorf("card after unlock = %+v, want the renamed card at todo position 1 with identity kept", renamed)
		}
	})

	t.Run("writing fresh text into done is not an edit — seed stays allowed", func(t *testing.T) {
		store := openStore(t)
		// Create lands in todo and Import feeds its done batches through
		// the same fresh-text path as Seed; the seed arm is Done's writing
		// witness here — none of the three is a title edit, so the freeze
		// must not reach them.
		if err := store.Seed(Done, []string{"s0", "s1"}); err != nil {
			t.Fatalf("Seed(Done, ...): %v — seeding into Done is not a title edit", err)
		}
		list := mustList(t, store)
		done := list[columnIndex(t, list, Done)].Cards
		if len(done) != 2 || done[0].Title != "s0" || done[0].Position != 0 ||
			done[1].Title != "s1" || done[1].Position != 1 {
			t.Errorf("Done column after seed = %s, want the given order at positions 0, 1", titles(done))
		}
	})
}

// frozenFixture boards {todo: t0} + {done: d0, d1} and hands back the store
// with the done card at position 1 — the frozen legs' one card.
func frozenFixture(t *testing.T) (*Store, Card) {
	t.Helper()
	store := openStore(t)
	seedBoard(t, store, []columnFixture{
		{Todo, []string{"t0"}},
		{Done, []string{"d0", "d1"}},
	})
	list := mustList(t, store)
	return store, list[columnIndex(t, list, Done)].Cards[1]
}

// The title direction stores the trimmed text — the same normalization Create
// applies, because it is the same rule function.
func TestChangeTitleStoresTrimmedText(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"leading and trailing spaces", "  renamed  ", "renamed"},
		{"tabs and newlines", "\t\nrenamed\n\t", "renamed"},
		{"inner whitespace preserved", "renamed\ttext here", "renamed\ttext here"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := openStore(t)
			created, err := store.Create("original")
			if err != nil {
				t.Fatalf("Create: %v", err)
			}

			changed, err := store.Change(created.ID, ptr(tt.text), nil)
			if err != nil {
				t.Fatalf("Change(%d, %q, nil): %v", created.ID, tt.text, err)
			}
			if changed.Title != tt.want {
				t.Errorf("returned Title = %q, want %q", changed.Title, tt.want)
			}
			board := mustList(t, store)
			if got := board[0].Cards[0].Title; got != tt.want {
				t.Errorf("listed title = %q, want %q", got, tt.want)
			}
		})
	}
}

// The title direction runs the SAME rule function Create screens through, so
// blank text comes back as ErrTextRequired and over-long text as
// ErrTextTooLong — decided before any write, so the board and every position
// stay exactly as they were and no statement runs against the card.
func TestChangeTitleRejectionsUseTheSameTextRules(t *testing.T) {
	tests := []struct {
		name string
		text string
		want error
	}{
		{"empty", "", ErrTextRequired},
		{"whitespace only", " \t\n ", ErrTextRequired},
		{"over the limit", strings.Repeat("x", MaxTextLen+1), ErrTextTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := openStore(t)
			seedBoard(t, store, []columnFixture{
				{Todo, []string{"t0", "t1"}},
				{InProgress, []string{"p0"}},
			})
			before := mustList(t, store)
			id := before[columnIndex(t, before, Todo)].Cards[1].ID

			got, err := store.Change(id, ptr(tt.text), nil)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Change(%d, %q, nil) err = %v, want %v", id, tt.text, err, tt.want)
			}
			if got != (Card{}) {
				t.Errorf("Change returned %+v on rejection, want the zero Card", got)
			}
			after := mustList(t, store)
			if !boardEqual(before, after) {
				t.Errorf("rejected change moved the board:\n before: %s\n after:  %s",
					flatten(before), flatten(after))
			}
		})
	}
}

// Card board/09 — Change of unknown card is reported.
// Given no card exists with identifier Z
// When  any change targets identifier Z
// Then  nothing changes and the store reports no such card
//
// "Any change" is exercised in every direction combination — title, column,
// and both together, with the column value valid so the not-found outcome is
// what the call reports. The existence read precedes every write, so the
// board is pinned cell-for-cell identical afterwards, and the zero Card comes
// back alongside the typed outcome.
func TestChangeUnknownCardIsReported(t *testing.T) {
	tests := []struct {
		name   string
		title  *string
		column *Column
	}{
		{"title direction", ptr("new text"), nil},
		{"column direction", nil, ptr(InProgress)},
		{"column direction, empty target", nil, ptr(Done)},
		{"both directions", ptr("new text"), ptr(Todo)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := openStore(t)
			seedBoard(t, store, []columnFixture{
				{Todo, []string{"t0", "t1"}},
				{InProgress, []string{"p0"}},
				{Done, nil},
			})
			before := mustList(t, store)
			z := unknownID(t, store)

			got, err := store.Change(z, tt.title, tt.column)
			if !errors.Is(err, ErrCardNotFound) {
				t.Fatalf("Change(%d, %+v, %+v) err = %v, want ErrCardNotFound", z, tt.title, tt.column, err)
			}
			if got != (Card{}) {
				t.Errorf("Change returned %+v on no-such-card, want the zero Card", got)
			}
			after := mustList(t, store)
			if !boardEqual(before, after) {
				t.Errorf("change of unknown card moved the board:\n before: %s\n after:  %s",
					flatten(before), flatten(after))
			}
		})
	}
}

// A rejected change reports through a read, not a write, so no identifier is
// burned either way — the next accepted create carries exactly the next id
// after the last one ever handed out.
func TestRejectedChangeDoesNotConsumeIdentifier(t *testing.T) {
	store := openStore(t)
	seedBoard(t, store, []columnFixture{{Todo, []string{"t0", "t1"}}})
	lastCreated, err := store.Create("t2")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	z := unknownID(t, store)

	if _, err := store.Change(z, ptr("burned?"), nil); !errors.Is(err, ErrCardNotFound) {
		t.Fatalf("title-direction change of unknown card err = %v, want ErrCardNotFound", err)
	}
	if _, err := store.Change(z, nil, ptr(Done)); !errors.Is(err, ErrCardNotFound) {
		t.Fatalf("column-direction change of unknown card err = %v, want ErrCardNotFound", err)
	}

	next, err := store.Create("next card")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if next.ID != lastCreated.ID+1 {
		t.Errorf("identifier after rejected changes = %d, want %d — a rejected change must not consume ids",
			next.ID, lastCreated.ID+1)
	}
}

// unknownID answers an identifier no card holds: one past the highest ever
// inserted (autoincrement never reuses, so it stays absent for the test's
// lifetime).
func unknownID(t *testing.T, store *Store) int64 {
	t.Helper()
	var max int64
	if err := store.db.QueryRow(`SELECT coalesce(max(id), 0) FROM cards`).Scan(&max); err != nil {
		t.Fatalf("max id: %v", err)
	}
	return max + 1
}

// Card board/10 — Invalid column value is rejected.
// Given an open board
// When  a change sets a card's column to a value other than todo, in_progress, done
// Then  nothing changes and the store reports the column as invalid
//
// Column is a string type, so the guard decides for any candidate value:
// empty, wrong case, padded, hyphenated, arbitrary junk — every one refused
// as ErrInvalidColumn before any write, the board pinned cell-for-cell
// unchanged.
func TestChangeRejectsInvalidColumn(t *testing.T) {
	tests := []struct {
		name string
		col  Column
	}{
		{"empty string", Column("")},
		{"capitalized todo", Column("Todo")},
		{"wrong case done", Column("Done")},
		{"trailing space", Column("done ")},
		{"leading space", Column(" todo")},
		{"hyphen instead of underscore", Column("in-progress")},
		{"arbitrary junk", Column("backlog")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := openStore(t)
			seedBoard(t, store, []columnFixture{
				{Todo, []string{"t0", "t1"}},
				{InProgress, []string{"p0"}},
				{Done, []string{"d0"}},
			})
			before := mustList(t, store)
			id := before[columnIndex(t, before, Todo)].Cards[0].ID

			got, err := store.Change(id, nil, ptr(tt.col))
			if !errors.Is(err, ErrInvalidColumn) {
				t.Fatalf("Change(%d, nil, %q) err = %v, want ErrInvalidColumn", id, string(tt.col), err)
			}
			if got != (Card{}) {
				t.Errorf("Change returned %+v on invalid column, want the zero Card", got)
			}
			after := mustList(t, store)
			if !boardEqual(before, after) {
				t.Errorf("rejected column change moved the board:\n before: %s\n after:  %s",
					flatten(before), flatten(after))
			}
		})
	}
}

// A valid column change lands the card at the BOTTOM of the target column
// (position = the target's size before the move) and closes the gap it left
// in the source column — both columns contiguous 0..n-1, source order
// preserved — with identifier and text untouched. Minimal bottom-append
// semantics only: neighbor ordering (index insert, same-column reorder) is
// the dedicated KW5 operation (board/06, board/07), and a requested column
// equal to the card's current one is not a move, so it places nothing.
func TestChangeColumnMovesToBottomAndClosesGap(t *testing.T) {
	tests := []struct {
		name    string
		columns []columnFixture
		pick    Column
		pickPos int
		to      Column
	}{
		{
			"scenario: todo middle to in_progress bottom",
			[]columnFixture{
				{Todo, []string{"A", "B", "C"}},
				{InProgress, []string{"X", "Y"}},
			},
			Todo, 1, InProgress,
		},
		{
			"in_progress top to done bottom",
			[]columnFixture{
				{Todo, []string{"t0"}},
				{InProgress, []string{"p0", "p1"}},
				{Done, []string{"d0"}},
			},
			InProgress, 0, Done,
		},
		{
			"move into an empty column",
			[]columnFixture{
				{Todo, []string{"t0", "t1"}},
			},
			Todo, 0, Done,
		},
		{
			"done card back to todo bottom — the move-out is the freeze unlock (2026-10-07)",
			[]columnFixture{
				{Todo, []string{"t0"}},
				{Done, []string{"d0", "d1"}},
			},
			Done, 1, Todo,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := openStore(t)
			seedBoard(t, store, tt.columns)

			before := mustList(t, store)
			target := before[columnIndex(t, before, tt.pick)].Cards[tt.pickPos]
			wantPosition := len(before[columnIndex(t, before, tt.to)].Cards)

			changed, err := store.Change(target.ID, nil, ptr(tt.to))
			if err != nil {
				t.Fatalf("Change(%d, nil, %q): %v", target.ID, tt.to, err)
			}
			if changed.ID != target.ID || changed.Title != target.Title {
				t.Errorf("moved card = %+v, want identity and text of %+v kept", changed, target)
			}
			if changed.Column != tt.to {
				t.Errorf("moved Column = %q, want %q", changed.Column, tt.to)
			}
			if changed.Position != wantPosition {
				t.Errorf("moved Position = %d, want %d (bottom of %q)", changed.Position, wantPosition, tt.to)
			}

			after := mustList(t, store)
			// Target: the moved card is last, at the bottom position.
			moved := after[columnIndex(t, after, tt.to)].Cards
			if len(moved) == 0 || moved[len(moved)-1].ID != target.ID || moved[len(moved)-1].Position != wantPosition {
				t.Errorf("%q bottom = %s, want card %d at position %d",
					tt.to, titles(moved), target.ID, wantPosition)
			}
			// Source: the card is gone and the gap is closed in order.
			source := after[columnIndex(t, after, tt.pick)].Cards
			if tt.pick != tt.to {
				for _, c := range source {
					if c.ID == target.ID {
						t.Errorf("card %d still listed in source column %q", target.ID, tt.pick)
					}
				}
				var wantIDs []int64
				for _, c := range before[columnIndex(t, before, tt.pick)].Cards {
					if c.ID != target.ID {
						wantIDs = append(wantIDs, c.ID)
					}
				}
				if len(source) != len(wantIDs) {
					t.Fatalf("%q holds %s, want the source minus the moved card", tt.pick, titles(source))
				}
				for i, wantID := range wantIDs {
					if source[i].ID != wantID {
						t.Errorf("%q order after the gap close = %s, want %v kept", tt.pick, titles(source), wantIDs)
					}
				}
			}
			// The invariant holds on every column, not just the two touched.
			for _, col := range after {
				for pos, c := range col.Cards {
					if c.Position != pos {
						t.Errorf("column %q position gap: card at index %d reports position %d — %s",
							col.Name, pos, c.Position, titles(col.Cards))
					}
				}
			}
		})
	}
}

// A requested column equal to the card's current column is not a move:
// Change carries no position direction, so it places nothing and the card
// keeps its position. This pin survives board/07 with its assertion unchanged
// but its rationale flipped: the old claim that within-column reordering
// doesn't exist *anywhere* in the store is retired — Move is the reorder
// operation (board/07, pinned in store_move_test.go as remove-then-insert at
// an index), and Change remains the column-only direction where no index is
// ever given.
func TestChangeColumnToSameColumnPlacesNothing(t *testing.T) {
	store := openStore(t)
	seedBoard(t, store, []columnFixture{{Todo, []string{"t0", "t1", "t2"}}})
	before := mustList(t, store)
	target := before[0].Cards[1]

	changed, err := store.Change(target.ID, nil, ptr(Todo))
	if err != nil {
		t.Fatalf("Change(%d, nil, Todo): %v", target.ID, err)
	}
	if changed != target {
		t.Errorf("same-column change returned %+v, want the card unchanged %+v", changed, target)
	}
	after := mustList(t, store)
	if !boardEqual(before, after) {
		t.Errorf("same-column change reordered the board:\n before: %s\n after:  %s",
			flatten(before), flatten(after))
	}
}

func ptr[T any](v T) *T { return &v }

// boardEqual reports cell-for-cell equality of two listings: same columns in
// the same order, the same cards at the same positions.
func boardEqual(before, after []ColumnCards) bool {
	if len(before) != len(after) {
		return false
	}
	for i := range before {
		if before[i].Name != after[i].Name || len(before[i].Cards) != len(after[i].Cards) {
			return false
		}
		for j := range before[i].Cards {
			if before[i].Cards[j] != after[i].Cards[j] {
				return false
			}
		}
	}
	return true
}

// flatten renders a listing as "title"@column:position for failure messages.
func flatten(board []ColumnCards) string {
	parts := make([]string, 0, 8)
	for _, col := range board {
		for _, c := range col.Cards {
			parts = append(parts, fmt.Sprintf("%q@%s:%d", c.Title, col.Name, c.Position))
		}
	}
	return strings.Join(parts, " ")
}
