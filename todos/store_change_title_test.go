package todos

import (
	"path/filepath"
	"testing"
)

// Card todos/04 — Change title of a not-done todo keeps done state.
// Given the store contains a not-done todo
// When  Change is called for its identifier with a new title only
// Then  the result is the todo with the new title and still not-done
func TestChangeTitleOfNotDoneKeepsDoneState(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	created, err := store.Create("Walk the dog")
	if err != nil {
		t.Fatalf("seed Create: %v", err)
	}

	// Title supplied only — the done column must not be touched (stays false).
	title := "Walk the dog in the park"
	got, err := store.Change(created.ID, ChangeFields{Title: &title})
	if err != nil {
		t.Fatalf("Change(title): %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("Change(title) id = %d, want the stable id %d", got.ID, created.ID)
	}
	if got.Title != title {
		t.Errorf("Change(title) title = %q, want %q", got.Title, title)
	}
	if got.Done {
		t.Errorf("Change(title) done = true, want the todo still not-done")
	}

	// The durable state matches: new text, still not-done, same identifier.
	for _, todo := range mustList(t, store) {
		if todo.ID == created.ID {
			if todo.Title != title || todo.Done {
				t.Errorf("List entry after title change = %+v, want {Title:%q Done:false}", todo, title)
			}
			return
		}
	}
	t.Fatalf("List lost the changed todo (id %d)", created.ID)
}
