package todos

import (
	"reflect"
	"testing"
)

// Card todos/12 — State survives reopen.
// Given todos exist with mixed done states
// When  the store is closed and a new store instance is opened on the same file
// Then  List returns the same todos with the same texts and done states
func TestStateSurvivesReopen(t *testing.T) {
	path := t.TempDir() + "/reopen.db"

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	// Mixed-state seed: not-done, done, not-done, done via real operations.
	titles := []string{"write report", "water plants", "buy milk", "call dentist"}
	var seeded []Todo
	for i, title := range titles {
		created, err := store.Create(title)
		if err != nil {
			t.Fatalf("Create %q: %v", title, err)
		}
		if i%2 == 1 {
			done := true
			created, err = store.Change(created.ID, ChangeFields{Done: &done})
			if err != nil {
				t.Fatalf("Change %d done: %v", created.ID, err)
			}
		}
		seeded = append(seeded, created)
	}
	before, err := store.List()
	if err != nil {
		t.Fatalf("List before close: %v", err)
	}
	if !reflect.DeepEqual(before, seeded) {
		t.Fatalf("seed List mismatch:\n got %+v\nwant %+v", before, seeded)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen Open: %v", err)
	}
	defer reopened.Close()

	after, err := reopened.List()
	if err != nil {
		t.Fatalf("List after reopen: %v", err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("state did not survive reopen:\nbefore %+v\n after %+v", before, after)
	}

	// The reopened store is fully live: operations keep working on the state.
	done := false
	if _, err := reopened.Change(after[1].ID, ChangeFields{Done: &done}); err != nil {
		t.Fatalf("Change on reopened store: %v", err)
	}
}
