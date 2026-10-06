package board

import (
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

func openStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "board.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}
