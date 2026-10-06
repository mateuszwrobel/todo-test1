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
// contract — any column, any position — and the done row pins that done cards
// are fully editable (no frozen state exists anywhere; column membership is
// the only done state). Place and identity are pinned through List, cell by
// cell: only the changed card's title may differ, so neighbors provably stay
// where they were.
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
		{
			"done column stays fully editable",
			[]columnFixture{
				{Todo, []string{"t0"}},
				{Done, []string{"d0", "d1"}},
			},
			Done, 1, "done cards edit too",
		},
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
