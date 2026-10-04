package todos

import (
	"errors"
	"path/filepath"
	"testing"
)

// Card todos/05 — Change done state keeps title.
// Given the store contains a todo with text "Buy milk"
// When  Change is called for its identifier with done only
// Then  the result is the todo with the text unchanged and the new done state
//
//	(and the same in the reopen direction, done false only)
func TestChangeDoneStateKeepsTitle(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	created, err := store.Create("Buy milk")
	if err != nil {
		t.Fatalf("seed Create: %v", err)
	}

	// Mark done: done supplied only — the title must be untouched.
	done := true
	got, err := store.Change(created.ID, ChangeFields{Done: &done})
	if err != nil {
		t.Fatalf("Change(done=true): %v", err)
	}
	if got.ID != created.ID || got.Title != "Buy milk" || !got.Done {
		t.Errorf("Change(done=true) = %+v, want {ID:%d Title:Buy milk Done:true}", got, created.ID)
	}

	// Reopen: the card's verbatim case — done false only — text unchanged.
	undone := false
	got, err = store.Change(created.ID, ChangeFields{Done: &undone})
	if err != nil {
		t.Fatalf("Change(done=false): %v", err)
	}
	if got.ID != created.ID || got.Title != "Buy milk" || got.Done {
		t.Errorf("Change(done=false) = %+v, want {ID:%d Title:Buy milk Done:false}", got, created.ID)
	}

	// The durable state matches the returned todo: text unchanged, done false.
	for _, todo := range mustList(t, store) {
		if todo.ID == created.ID {
			if todo.Title != "Buy milk" || todo.Done {
				t.Errorf("List entry after reopen = %+v, want {Title:Buy milk Done:false}", todo)
			}
			return
		}
	}
	t.Fatalf("List lost the changed todo (id %d)", created.ID)
}

// Card todos/08 — Change missing identifier.
// Given no todo exists with identifier X
// When  Change is called for X
// Then  the result is a typed not-found outcome and no state changes
func TestChangeMissingIdentifier(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	seed, err := store.Create("keep me")
	if err != nil {
		t.Fatalf("seed Create: %v", err)
	}
	before := mustList(t, store)

	// An identifier no todo has ever had (autoincrement: seed.ID + 1 is unassigned).
	missing := seed.ID + 1

	done := true
	_, err = store.Change(missing, ChangeFields{Done: &done})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Change(missing id) err = %v, want ErrNotFound", err)
	}

	// No state change: the list is exactly what it was.
	after := mustList(t, store)
	if len(after) != len(before) {
		t.Fatalf("list changed size: before %+v after %+v", before, after)
	}
	for i := range before {
		if after[i] != before[i] {
			t.Errorf("state changed: entry %d = %+v, want %+v", i, after[i], before[i])
		}
	}
}

// Card todos/09 — Change with no fields is invalid.
// Given the store is open (with a todo, to prove nothing moves)
// When  Change is called with neither title nor done supplied
// Then  the result is an invalid outcome and no state changes
func TestChangeWithNoFieldsIsInvalid(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	created, err := store.Create("untouched")
	if err != nil {
		t.Fatalf("seed Create: %v", err)
	}
	before := mustList(t, store)

	_, err = store.Change(created.ID, ChangeFields{})
	if !errors.Is(err, ErrNoFields) {
		t.Fatalf("Change(no fields) err = %v, want ErrNoFields", err)
	}

	after := mustList(t, store)
	if len(after) != 1 || after[0] != before[0] {
		t.Fatalf("state changed on invalid change: before %+v after %+v", before, after)
	}
}
