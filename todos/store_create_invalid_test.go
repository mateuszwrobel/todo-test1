package todos

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Card todos/02 — Create rejects invalid text.
// Given the store is open
// When  Create is called with empty, whitespace-only, or longer-than-500-character
//
//	text
//
// Then  the result is an invalid-text outcome
//
//	And no state changes
func TestCreateRejectsInvalidText(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	// Seed one valid todo so "no state changes" has something to measure against.
	if _, err := store.Create("seed"); err != nil {
		t.Fatalf("seed Create: %v", err)
	}
	before := mustList(t, store)

	invalid := []struct {
		name string
		text string
	}{
		{"empty", ""},
		{"whitespace only", " \t\n "},
		{"over the limit", strings.Repeat("x", MaxTitleLength+1)},
	}
	for _, tc := range invalid {
		created, err := store.Create(tc.text)
		switch {
		case errors.Is(err, ErrTitleRequired), errors.Is(err, ErrTitleTooLong):
			// An invalid-text outcome — what the contract states for this input.
		case err == nil:
			t.Errorf("Create(%s): accepted invalid text, got %+v", tc.name, created)
		default:
			t.Errorf("Create(%s): err = %v, want an invalid-text outcome", tc.name, err)
		}
	}

	// And no state changes: the list is exactly what it was.
	if after := mustList(t, store); !reflect.DeepEqual(before, after) {
		t.Errorf("state changed by rejected creates:\nbefore: %+v\nafter:  %+v", before, after)
	}
}

// The limit is "over 500": exactly MaxTitleLength characters is valid, and
// validation (and stored form) use the trimmed title — the single canonical
// text the rules speak about.
func TestCreateAcceptsLimitBoundaryAndTrims(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	atLimit, err := store.Create(strings.Repeat("y", MaxTitleLength))
	if err != nil {
		t.Fatalf("Create(exactly the limit): %v", err)
	}
	if r := runeCount(atLimit.Title); r != MaxTitleLength {
		t.Errorf("stored title has %d characters, want exactly the limit", r)
	}

	padded, err := store.Create("   Buy milk\t")
	if err != nil {
		t.Fatalf("Create(padded text): %v", err)
	}
	if padded.Title != "Buy milk" {
		t.Errorf("padded.Title = %q, want the trimmed title %q", padded.Title, "Buy milk")
	}
}

func runeCount(s string) int {
	return len([]rune(s))
}
