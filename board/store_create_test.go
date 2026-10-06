package board

import (
	"errors"
	"path/filepath"
	"testing"
)

// Card board/02 — Create appends to bottom of first column.
// Given a board holding cards in the todo column
// When  a card is created
// Then  the todo column lists the new card last at position count-before-creation
//
//	And the card carries an identifier assigned by the store
//	And the card's column is todo
//
// The card's scenario pins two prior cards and position 2; the table checks the
// general contract — bottom append at any column size, arbitrary texts.
func TestCreateAppendsToBottomOfTodo(t *testing.T) {
	tests := []struct {
		name     string
		existing []string
		text     string
	}{
		{"empty todo column", nil, "first card"},
		{"scenario: after two cards", []string{"walk dog", "pay bill"}, "Buy milk"},
		{"longer column", []string{"a1", "b2", "c3", "d4", "e5"}, "fresh bottom"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := openStore(t)

			for _, text := range tt.existing {
				if _, err := store.Create(text); err != nil {
					t.Fatalf("seed Create(%q): %v", text, err)
				}
			}

			created, err := store.Create(tt.text)
			if err != nil {
				t.Fatalf("Create(%q): %v", tt.text, err)
			}
			if created.Title != tt.text {
				t.Errorf("created.Title = %q, want %q", created.Title, tt.text)
			}
			if created.Column != Todo {
				t.Errorf("created.Column = %q, want %q", created.Column, Todo)
			}
			if want := len(tt.existing); created.Position != want {
				t.Errorf("created.Position = %d, want %d (bottom of todo)", created.Position, want)
			}
			if created.ID == 0 {
				t.Error("created.ID = 0, want a store-assigned identifier")
			}

			// The listing agrees with the returned card: last in todo, at that
			// position, under that identifier; the other columns untouched.
			board := mustList(t, store)
			todo := board[0]
			if todo.Name != Todo {
				t.Fatalf("List column 0 = %q, want %q", todo.Name, Todo)
			}
			if want := len(tt.existing) + 1; len(todo.Cards) != want {
				t.Fatalf("todo holds %d cards, want %d", len(todo.Cards), want)
			}
			last := todo.Cards[len(todo.Cards)-1]
			if last.ID != created.ID || last.Title != tt.text ||
				last.Column != Todo || last.Position != len(tt.existing) {
				t.Errorf("todo bottom = %+v, want the created card %+v", last, created)
			}
			for _, col := range board[1:] {
				if len(col.Cards) != 0 {
					t.Errorf("Create wrote into column %q: %+v", col.Name, col.Cards)
				}
			}
		})
	}
}

// Identifiers are assigned by the store and never repeat: successive Creates
// hand out distinct non-zero ids regardless of text.
func TestCreateAssignsDistinctIdentifiers(t *testing.T) {
	store := openStore(t)

	texts := []string{"one", "two", "three"}
	seen := map[int64]string{}
	for _, text := range texts {
		created, err := store.Create(text)
		if err != nil {
			t.Fatalf("Create(%q): %v", text, err)
		}
		if created.ID == 0 {
			t.Fatalf("Create(%q) returned zero identifier", text)
		}
		if prior, dup := seen[created.ID]; dup {
			t.Fatalf("Create(%q) reused identifier %d from Create(%q)", text, created.ID, prior)
		}
		seen[created.ID] = text
	}
}

// Card board/03 — Create rejects blank text.
// Given an open board
// When  a card with empty or whitespace-only text is created
// Then  no card is created and the store reports the text as invalid (required)
//
// Whitespace is the Unicode set (TrimSpace), so tabs and newlines count — the
// schema CHECK's trim() only strips spaces and is never the observable rule.
func TestCreateRejectsBlankText(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"empty", ""},
		{"spaces", "   "},
		{"tabs", "\t\t"},
		{"newlines", "\n\n"},
		{"mixed whitespace", " \t\n "},
		{"unicode whitespace (NBSP)", "\u00a0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := openStore(t)

			created, err := store.Create(tt.text)
			if !errors.Is(err, ErrTextRequired) {
				t.Fatalf("Create(%q) err = %v, want ErrTextRequired", tt.text, err)
			}
			if created != (Card{}) {
				t.Errorf("Create(%q) returned %+v on rejection, want the zero Card", tt.text, created)
			}
			board := mustList(t, store)
			for _, col := range board {
				if len(col.Cards) != 0 {
					t.Errorf("rejected Create(%q) wrote into column %q: %+v", tt.text, col.Name, col.Cards)
				}
			}
		})
	}
}

// Rejection leaves an existing board exactly as it was — the refusal happens
// before any mutation, so no card is added and no position shifts.
func TestCreateRejectionLeavesBoardUnchanged(t *testing.T) {
	store := openStore(t)

	for _, text := range []string{"keep 0", "keep 1"} {
		if _, err := store.Create(text); err != nil {
			t.Fatalf("seed Create(%q): %v", text, err)
		}
	}
	before := mustList(t, store)

	for _, text := range []string{"", "   ", "\t\n"} {
		if _, err := store.Create(text); !errors.Is(err, ErrTextRequired) {
			t.Fatalf("Create(%q) err = %v, want ErrTextRequired", text, err)
		}
	}

	after := mustList(t, store)
	if len(before) != len(after) {
		t.Fatalf("column count changed: %d before, %d after", len(before), len(after))
	}
	for i := range before {
		if before[i].Name != after[i].Name {
			t.Errorf("column %d name changed: %q -> %q", i, before[i].Name, after[i].Name)
		}
		if len(before[i].Cards) != len(after[i].Cards) {
			t.Fatalf("column %q holds %d cards before, %d after", before[i].Name, len(before[i].Cards), len(after[i].Cards))
		}
		for j := range before[i].Cards {
			if before[i].Cards[j] != after[i].Cards[j] {
				t.Errorf("column %q card %d changed: %+v -> %+v", before[i].Name, j, before[i].Cards[j], after[i].Cards[j])
			}
		}
	}
}

// A rejected create must not consume an identifier: autoincrement may burn
// ids on failed inserts, so the rejection has to precede the insert — pinned
// by requiring the next accepted create to carry exactly the next id.
func TestRejectedCreateDoesNotConsumeIdentifier(t *testing.T) {
	store := openStore(t)

	first, err := store.Create("valid first")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := store.Create("   \t"); !errors.Is(err, ErrTextRequired) {
		t.Fatalf("blank Create err = %v, want ErrTextRequired", err)
	}

	next, err := store.Create("valid second")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if next.ID != first.ID+1 {
		t.Errorf("identifier after rejection = %d, want %d — a rejected create must not consume ids", next.ID, first.ID+1)
	}
}

// The stored text is the trimmed text — surrounding whitespace (spaces, tabs,
// newlines) is stripped, never stored.
func TestCreateStoresTrimmedText(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"leading and trailing spaces", "  buy milk  ", "buy milk"},
		{"tabs and newlines", "\n\tbuy milk\t\n", "buy milk"},
		{"inner whitespace preserved", "buy\tmilk now", "buy\tmilk now"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := openStore(t)

			created, err := store.Create(tt.text)
			if err != nil {
				t.Fatalf("Create(%q): %v", tt.text, err)
			}
			if created.Title != tt.want {
				t.Errorf("created.Title = %q, want %q", created.Title, tt.want)
			}
			board := mustList(t, store)
			if got := board[0].Cards[0].Title; got != tt.want {
				t.Errorf("listed title = %q, want %q", got, tt.want)
			}
		})
	}
}

func openStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "board.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}
