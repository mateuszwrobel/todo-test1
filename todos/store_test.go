package todos

import (
	"os"
	"path/filepath"
	"testing"
)

// Card todos/13 — Fresh file opens as empty valid store.
// Given no data file exists at the given path
// When  the store is opened at that path
// Then  the store is empty and all operations work
func TestOpenFreshFileIsEmptyValidStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fresh.db")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("data file must not exist before Open, stat err = %v", err)
	}

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open on absent path failed: %v", err)
	}
	defer store.Close()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Open did not create the data file: %v", err)
	}

	todos, err := store.List()
	if err != nil {
		t.Fatalf("List on fresh store failed: %v", err)
	}
	if len(todos) != 0 {
		t.Fatalf("fresh store must be empty, got %d todos", len(todos))
	}

	// "all operations work": Create and List succeed against the fresh store.
	created, err := store.Create("first")
	if err != nil {
		t.Fatalf("Create on fresh store failed: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("Create on fresh store returned zero id")
	}
	todos, err = store.List()
	if err != nil {
		t.Fatalf("List after Create failed: %v", err)
	}
	if len(todos) != 1 || todos[0].Title != "first" {
		t.Fatalf("List after Create = %+v, want one todo {Title:first}", todos)
	}
}
