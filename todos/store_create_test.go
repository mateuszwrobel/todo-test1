package todos

import (
	"path/filepath"
	"testing"
)

// Card todos/01 — Create assigns fresh identifier.
// Given the store contains no todo with the text "Buy milk"
// When  Create is called with that text
// Then  the result is a todo with that text, done false, and an identifier
//
//	never used before
//
//	And the next List includes it
func TestCreateAssignsFreshIdentifier(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	// A previously used identifier: create and delete-equivalent history is
	// out of W1 scope, so seed one todo to establish a used id.
	seed, err := store.Create("seed")
	if err != nil {
		t.Fatalf("seed Create: %v", err)
	}

	// Given: no todo with that text exists.
	for _, todo := range mustList(t, store) {
		if todo.Title == "Buy milk" {
			t.Fatal("Given violated: store already contains the text")
		}
	}

	created, err := store.Create("Buy milk")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Title != "Buy milk" {
		t.Errorf("created.Title = %q, want %q", created.Title, "Buy milk")
	}
	if created.Done {
		t.Error("created.Done = true, want false")
	}
	if created.ID == 0 {
		t.Error("created.ID = 0, want a fresh non-zero identifier")
	}
	if created.ID == seed.ID {
		t.Errorf("created.ID %d reuses previously used identifier", created.ID)
	}

	list, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var found bool
	for _, todo := range list {
		if todo.ID == created.ID {
			found = true
			if todo.Title != "Buy milk" || todo.Done {
				t.Errorf("List entry %+v, want {Title:Buy milk, Done:false}", todo)
			}
		}
	}
	if !found {
		t.Fatalf("List does not include the created todo (id %d): %+v", created.ID, list)
	}
}

func mustList(t *testing.T, store *Store) []Todo {
	t.Helper()
	list, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	return list
}
