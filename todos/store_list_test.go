package todos

import (
	"path/filepath"
	"testing"
)

// Card todos/03 — List is creation order.
// Given the store contains several todos
// When  List is called
// Then  todos are returned in ascending identifier order, oldest first
func TestListIsCreationOrder(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	titles := []string{"first", "second", "third", "fourth"}
	created := make([]Todo, 0, len(titles))
	for _, title := range titles {
		todo, err := store.Create(title)
		if err != nil {
			t.Fatalf("Create(%q): %v", title, err)
		}
		created = append(created, todo)
	}

	list := mustList(t, store)
	if len(list) != len(created) {
		t.Fatalf("List returned %d todos, want %d", len(list), len(created))
	}
	for i := range list {
		if list[i].ID != created[i].ID {
			t.Fatalf("List order = %v, want creation order %v",
				idSlice(list), idSlice(created))
		}
	}
	for i := 1; i < len(list); i++ {
		if list[i].ID <= list[i-1].ID {
			t.Fatalf("identifiers not ascending at index %d: %v", i, idSlice(list))
		}
	}
}

func idSlice(todos []Todo) []int64 {
	ids := make([]int64, len(todos))
	for i, t := range todos {
		ids[i] = t.ID
	}
	return ids
}
