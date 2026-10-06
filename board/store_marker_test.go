package board

import (
	"errors"
	"path/filepath"
	"testing"
)

// The import marker is the migration guard's durable observation: a fresh
// board answers Imported false, a recorded board answers true — and the
// answer depends on the marker row alone, never on what the board holds.
// The card-count reading ("an empty board was never created") is exactly
// what the delete-all-after-import repro breaks; these pins keep the marker
// independent of card state.
func TestFreshStoreHasNoImportMarker(t *testing.T) {
	store := openStore(t)

	done, err := store.Imported()
	if err != nil {
		t.Fatalf("Imported on a fresh store: %v", err)
	}
	if done {
		t.Fatal("a fresh store answers Imported — a fresh board has never run the import decision")
	}
}

// Import commits its marker in the same transaction as the cards: a landed
// import is recorded forever (and the record survives Close and reopen at
// the same path), while the failure side of a mid-import stop records
// nothing — a refused import executed no statement, so no marker rides a
// rollback that wrote nothing, and an empty-batches no-op "reads nothing,
// writes nothing" including the marker. These are the two halves the
// migration guard needs: landed = never again; interrupted = retry.
func TestImportSetsMarkerAtomically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "board.db")

	store := openStoreAt(t, path)
	batches := []SeedBatch{
		{Todo, []string{"keep me", "and me"}},
		{Done, []string{"was done"}},
	}
	if err := store.Import(batches); err != nil {
		t.Fatalf("Import: %v", err)
	}
	done, err := store.Imported()
	if err != nil {
		t.Fatalf("Imported after a landed import: %v", err)
	}
	if !done {
		t.Fatal("a landed import left no marker — the guard could re-import on the next start")
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reopened := openStoreAt(t, path)
	done, err = reopened.Imported()
	if err != nil {
		t.Fatalf("Imported after reopen: %v", err)
	}
	if !done {
		t.Fatal("the marker did not survive Close and reopen — the record would reset with the process")
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("Close reopened: %v", err)
	}

	// The failure side, on fresh boards: refused imports and no-op imports
	// leave the marker exactly as a fresh store has it — absent — so a
	// guard reading it may retry (card server/05's retry arm).
	for _, tt := range []struct {
		name    string
		batches []SeedBatch
	}{
		{"refused — one blank entry refuses the whole import and its marker",
			[]SeedBatch{{Todo, []string{"fine", "   "}}}},
		{"refused — one invalid column refuses every batch",
			[]SeedBatch{{Todo, []string{"fine"}}, {Column("backlog"), []string{"no"}}}},
		{"no-op — empty batches write nothing, marker included",
			[]SeedBatch{{Todo, nil}, {Done, []string{}}}},
		{"no-op — nothing at all to place", nil},
	} {
		store := openStore(t)
		if err := store.Import(tt.batches); err != nil && !errors.Is(err, ErrTextRequired) && !errors.Is(err, ErrInvalidColumn) {
			t.Fatalf("%s: Import: %v", tt.name, err)
		}
		done, err := store.Imported()
		if err != nil {
			t.Fatalf("%s: Imported: %v", tt.name, err)
		}
		if done {
			t.Errorf("%s left an import marker — an import that never landed must record nothing", tt.name)
		}
	}
}

// MarkImported records a decision that placed no cards — the first start
// that ran the decision against a source that is absent or holds no rows.
// The write is idempotent, changes no card state, and survives reopen like
// any other recorded decision.
func TestMarkImportedRecordsDecisionWithoutCards(t *testing.T) {
	path := filepath.Join(t.TempDir(), "board.db")
	store := openStoreAt(t, path)

	if err := store.MarkImported(); err != nil {
		t.Fatalf("MarkImported: %v", err)
	}
	if err := store.MarkImported(); err != nil {
		t.Fatalf("repeat MarkImported must be idempotent, got: %v", err)
	}
	done, err := store.Imported()
	if err != nil {
		t.Fatalf("Imported after MarkImported: %v", err)
	}
	if !done {
		t.Fatal("MarkImported left no marker — the decision would not survive the next start")
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reopened := openStoreAt(t, path)
	done, err = reopened.Imported()
	if err != nil {
		t.Fatalf("Imported after reopen: %v", err)
	}
	if !done {
		t.Fatal("the recorded decision did not survive Close and reopen")
	}
	board := mustList(t, reopened)
	for _, col := range board {
		if len(col.Cards) != 0 {
			t.Errorf("MarkImported touched card state — column %q holds %s", col.Name, titles(col.Cards))
		}
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("Close reopened: %v", err)
	}
}

// Verdict on the marker gate: Import does NOT refuse when the marker is on
// record. The guard is startup policy and lives in the composition root
// (the workplan places the whole import decision there, principle 8);
// Import is the store's ordered-insert primitive, and refusing on marker
// state would hand it a second reason to change — migration semantics —
// while protecting no window that exists: the marker commits atomically
// with the cards, and the single-process start sequence reads the guard
// before ever reaching Import. The marker write is idempotent, so a repeat
// import stays consistent: the batches land below the residents exactly as
// the contract states and the record stays set.
func TestImportWithMarkerOnRecordStillPlaces(t *testing.T) {
	store := openStore(t)
	seedBoard(t, store, []columnFixture{{Todo, []string{"resident"}}})
	if err := store.MarkImported(); err != nil {
		t.Fatalf("MarkImported: %v", err)
	}

	if err := store.Import([]SeedBatch{{Todo, []string{"appended"}}}); err != nil {
		t.Fatalf("Import with the marker on record must not refuse: %v", err)
	}
	board := mustList(t, store)
	assertPlacements(t, board, []placement{
		{"resident", Todo, 0}, {"appended", Todo, 1},
	})
	done, err := store.Imported()
	if err != nil {
		t.Fatalf("Imported: %v", err)
	}
	if !done {
		t.Fatal("a repeat import cleared the recorded decision — the marker is permanent state")
	}
}

// The marker is the whole guard truth the delete-all repro needs: a board
// that imported and then lost every card — every Delete ran, the column
// renormalized to empty — STILL answers Imported. Card emptiness is not
// the observation, and no amount of deleting rewinds a recorded decision.
func TestImportedSurvivesDeletingEveryCard(t *testing.T) {
	store := openStore(t)
	if err := store.Import([]SeedBatch{
		{Todo, []string{"r1", "r2"}},
		{Done, []string{"d1"}},
	}); err != nil {
		t.Fatalf("Import: %v", err)
	}
	for _, col := range mustList(t, store) {
		for _, card := range col.Cards {
			if err := store.Delete(card.ID); err != nil {
				t.Fatalf("Delete %d: %v", card.ID, err)
			}
		}
	}
	board := mustList(t, store)
	for _, col := range board {
		if len(col.Cards) != 0 {
			t.Fatalf("delete-all left cards in %q: %s", col.Name, titles(col.Cards))
		}
	}
	done, err := store.Imported()
	if err != nil {
		t.Fatalf("Imported on an emptied imported board: %v", err)
	}
	if !done {
		t.Fatal("an emptied board answers not-imported — the resurrection defect is back")
	}
}
