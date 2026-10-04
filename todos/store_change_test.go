package todos

import (
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
