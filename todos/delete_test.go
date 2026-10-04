package todos

import (
	"errors"
	"path/filepath"
	"testing"
)

// Card todos/10 — Delete removes and identifier is never reused.
// Given the store contains a todo
// When  Delete is called for its identifier and then Create is called
// Then  the deleted todo never appears in List again
//
//	And the new todo's identifier differs from the deleted one
//
// The non-reuse assertion reads the autoincrement guarantee directly: the
// freshly created identifier is strictly greater than the deleted one, so the
// schema's never-recycle property holds rather than a coincidence of ids.
func TestDeleteRemovesAndIdentifierNeverReused(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	doomed, err := store.Create("doomed")
	if err != nil {
		t.Fatalf("seed Create: %v", err)
	}
	if err := store.Delete(doomed.ID); err != nil {
		t.Fatalf("Delete(%d): %v", doomed.ID, err)
	}

	fresh, err := store.Create("successor")
	if err != nil {
		t.Fatalf("Create after Delete: %v", err)
	}
	if fresh.ID == doomed.ID {
		t.Errorf("new id %d reuses the deleted identifier", fresh.ID)
	}
	if fresh.ID <= doomed.ID {
		t.Errorf("new id %d is not strictly greater than deleted id %d: autoincrement must never recycle rowids", fresh.ID, doomed.ID)
	}

	// The deleted todo never appears in List again — not now, not after
	// another Create.
	for _, todo := range mustList(t, store) {
		if todo.ID == doomed.ID {
			t.Fatalf("deleted todo (id %d) appears in List: %+v", doomed.ID, todo)
		}
	}
}

// Card todos/11 — Delete missing identifier.
// Given no todo exists with identifier X
// When  Delete is called for X
// Then  the result is a not-found outcome
//
//	And no state changes
func TestDeleteMissingIdentifierIsNotFound(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	keep, err := store.Create("untouched")
	if err != nil {
		t.Fatalf("seed Create: %v", err)
	}

	const missing int64 = 987654321
	if err := store.Delete(missing); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete(%d) error = %v, want not-found outcome (ErrNotFound)", missing, err)
	}

	// No state change: the store still holds exactly the pre-existing todo.
	list := mustList(t, store)
	if len(list) != 1 || list[0] != keep {
		t.Fatalf("List after failed Delete = %+v, want exactly [%+v]", list, keep)
	}
}
