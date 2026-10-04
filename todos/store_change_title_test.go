package todos

import (
	"errors"
	"path/filepath"
	"reflect"
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

// Card todos/06 — Change title on a done todo is refused.
// Given the store contains a done todo
// When  Change is called for its identifier with a title
// Then  the result is a done-frozen outcome
//
//	And the todo's text and done state are unchanged
func TestChangeTitleOnDoneTodoIsRefused(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	created, err := store.Create("Buy milk")
	if err != nil {
		t.Fatalf("seed Create: %v", err)
	}
	done := true
	if _, err := store.Change(created.ID, ChangeFields{Done: &done}); err != nil {
		t.Fatalf("seed Change(done=true): %v", err)
	}
	before := mustList(t, store)

	// A title supplied for a done todo is the frozen case — refused before
	// any column update; even carried alongside done, the refusal stands.
	title := "Buy oat milk"
	for name, fields := range map[string]ChangeFields{
		"title only":      {Title: &title},
		"title with done": {Title: &title, Done: &done},
	} {
		got, err := store.Change(created.ID, fields)
		if !errors.Is(err, ErrDoneFrozen) {
			t.Errorf("Change(%s) on a done todo: err = %v, got %+v; want the done-frozen outcome", name, err, got)
		}
	}

	// And nothing changed: text and done state are exactly as before.
	if after := mustList(t, store); !reflect.DeepEqual(before, after) {
		t.Errorf("state changed by a frozen change:\nbefore: %+v\nafter:  %+v", before, after)
	}
}

// The freeze is about the TITLE direction only: a done-only change (the
// reopen) stays allowed on a done todo — exercised by todos/07 below.

// Card todos/07 — Reopen a done todo by done-only change.
// Given the store contains a done todo
// When  Change is called for its identifier with done false only
// Then  the result is the todo, not-done, text unchanged
//
//	(The card's edit story: the reopen UNLOCKS editing — the title
//	direction then succeeds on the same todo.)
func TestReopenDoneTodoUnlocksTitleEditing(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	created, err := store.Create("Pay electricity bill")
	if err != nil {
		t.Fatalf("seed Create: %v", err)
	}
	done := true
	if _, err := store.Change(created.ID, ChangeFields{Done: &done}); err != nil {
		t.Fatalf("seed Change(done=true): %v", err)
	}

	// Editing is frozen while done — the rule from todos/06.
	frozen := "Pay the water bill"
	if _, err := store.Change(created.ID, ChangeFields{Title: &frozen}); !errors.Is(err, ErrDoneFrozen) {
		t.Fatalf("Change(title) on done todo: err = %v, want ErrDoneFrozen", err)
	}

	// Reopen: done false only — text unchanged, todo not-done.
	undone := false
	got, err := store.Change(created.ID, ChangeFields{Done: &undone})
	if err != nil {
		t.Fatalf("Change(done=false): %v", err)
	}
	if got.ID != created.ID || got.Title != "Pay electricity bill" || got.Done {
		t.Errorf("reopen = %+v, want {ID:%d Title:Pay electricity bill Done:false}", got, created.ID)
	}

	// The unlock: the title direction now succeeds on the reopened todo.
	title := "Pay electricity bill in person"
	got, err = store.Change(created.ID, ChangeFields{Title: &title})
	if err != nil {
		t.Fatalf("Change(title) after reopen: %v, want the freeze lifted", err)
	}
	if got.Title != title || got.Done {
		t.Errorf("Change(title) after reopen = %+v, want {Title:%q Done:false}", got, title)
	}
}
