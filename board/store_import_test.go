package board

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// Migration import — order preserved across three columns in ONE call.
// Given an empty board
// When  Import places one ordered list per column, all three columns at once
// Then  each column holds exactly its own texts top-to-bottom in the given
//
//	order, below nothing (fresh board), with fresh identifiers
//
// The lists are ones no creation sequence produced — the given order is the
// only order the column can show — and one text arrives padded to pin that
// Import runs the same trimming rule as Create and Seed. Identifiers come
// from the autoincrement: strictly ascending along each column's given order
// and above every id ever issued before the import. No column holds another
// column's card, and the whole board is exactly the fixture: one Import,
// one transaction, every column in order or not at all.
func TestImportOrderedAcrossThreeColumns(t *testing.T) {
	store := openStore(t)
	before := maxID(t, store)

	batches := []SeedBatch{
		{Todo, []string{"zeta", "alpha", "middle"}},
		{InProgress, []string{"w4", "w1", "w3", "w2"}},
		{Done, []string{"  shipped release \t", "archived"}},
	}
	if err := store.Import(batches); err != nil {
		t.Fatalf("Import: %v", err)
	}

	board := mustList(t, store)
	assertPlacements(t, board, []placement{
		{"zeta", Todo, 0}, {"alpha", Todo, 1}, {"middle", Todo, 2},
		{"w4", InProgress, 0}, {"w1", InProgress, 1}, {"w3", InProgress, 2}, {"w2", InProgress, 3},
		{"shipped release", Done, 0}, {"archived", Done, 1},
	})
	for _, col := range board {
		for pos, c := range col.Cards {
			if c.ID <= before {
				t.Errorf("card %q carries id %d, want fresh — above every id issued before the import (%d)",
					c.Title, c.ID, before)
			}
			if pos > 0 && col.Cards[pos-1].ID >= c.ID {
				t.Errorf("card %q carries id %d, want above %q's %d — ids follow the given order",
					c.Title, c.ID, col.Cards[pos-1].Title, col.Cards[pos-1].ID)
			}
		}
	}
	assertContiguous(t, board)
}

// One bad anything refuses the whole import: the contract is that every
// column ends holding exactly its list, so a partial import is not an
// outcome. Every validation runs before Begin — every batch's column
// against the fixed enum, then every entry through validateText — so any
// single fault anywhere answers the typed store error naming the batch (and
// entry) and leaves zero writes behind: board cell-for-cell unchanged and no
// identifier consumed, exactly as the rejected Seed behaves. The enum guard
// runs across all batches before any text is read, so a bad column in any
// batch wins over text faults wherever they sit.
func TestImportRejectionsRefuseWholeImport(t *testing.T) {
	good := []string{"fine one", "another", "third"}
	tests := []struct {
		name    string
		batches []SeedBatch
		want    error
		wantMsg string // the refusal names the failing batch and entry
	}{
		{
			"one blank entry among many — batch 1 entry 1 refuses every batch",
			[]SeedBatch{
				{Todo, good},
				{InProgress, []string{"ok", "", "also ok"}},
				{Done, []string{"was fine"}},
			},
			ErrTextRequired, "batch 1 entry 1",
		},
		{
			"one whitespace-only entry last in the last batch",
			[]SeedBatch{
				{Todo, good},
				{Done, []string{"ok", "  \t "}},
			},
			ErrTextRequired, "batch 1 entry 1",
		},
		{
			"one over-long entry in the middle of the middle batch",
			[]SeedBatch{
				{Done, []string{"fine"}},
				{Todo, []string{"ok", strings.Repeat("x", MaxTextLen+1), "ok too"}},
				{InProgress, good},
			},
			ErrTextTooLong, "batch 1 entry 1",
		},
		{
			"invalid column in a later batch refuses the earlier valid ones",
			[]SeedBatch{
				{Todo, good},
				{InProgress, []string{"also fine"}},
				{Column("backlog"), []string{"never seen"}},
			},
			ErrInvalidColumn, "batch 2",
		},
		{
			"invalid column in the first batch refuses everything after it",
			[]SeedBatch{
				{Column("in-progress"), []string{"never seen"}},
				{Done, good},
			},
			ErrInvalidColumn, "batch 0",
		},
		{
			"bad column and bad text together — the enum guard answers first",
			[]SeedBatch{
				{Todo, []string{"ok", ""}},
				{Column("nope"), []string{"ok"}},
			},
			ErrInvalidColumn, "batch 1",
		},
		{
			"empty texts but an invalid column — the guard is structural",
			[]SeedBatch{
				{Todo, good},
				{Column("Todo"), nil},
			},
			ErrInvalidColumn, "batch 1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := openStore(t)
			seedBoard(t, store, []columnFixture{{Todo, []string{"resident"}}})
			before := mustList(t, store)
			lastCreated := maxID(t, store)

			err := store.Import(tt.batches)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Import err = %v, want %v", err, tt.want)
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("Import err = %q, want it to name %q", err, tt.wantMsg)
			}
			after := mustList(t, store)
			if !boardEqual(before, after) {
				t.Errorf("rejected import changed the board:\n before: %s\n after:  %s",
					flatten(before), flatten(after))
			}
			probe, err := store.Create("id probe")
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			if probe.ID != lastCreated+1 {
				t.Errorf("identifier after rejected import = %d, want %d — a rejected import must not consume ids",
					probe.ID, lastCreated+1)
			}
		})
	}
}

// A valid-but-invalid-column-free nothing is a nil no-op: an empty batches
// list, an empty slice, and batches that name valid columns but hold no
// texts — on an empty board and on a populated one. Nothing is read, nothing
// is written, no identifier is consumed.
func TestImportEmptyIsNoOp(t *testing.T) {
	empty := openStore(t)
	for i, batches := range [][]SeedBatch{
		nil,
		{},
		{{Todo, nil}, {InProgress, []string{}}, {Done, nil}},
	} {
		if err := empty.Import(batches); err != nil {
			t.Errorf("Import %d on an empty board: %v", i, err)
		}
	}
	board := mustList(t, empty)
	for _, col := range board {
		if len(col.Cards) != 0 {
			t.Errorf("column %q holds %s after empty imports, want no cards", col.Name, titles(col.Cards))
		}
	}
	probe, err := empty.Create("first ever")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if probe.ID != 1 {
		t.Errorf("first id after empty imports = %d, want 1 — a no-op consumes nothing", probe.ID)
	}

	populated := openStore(t)
	seedBoard(t, populated, []columnFixture{
		{Todo, []string{"t0", "t1"}},
		{Done, []string{"d0"}},
	})
	before := mustList(t, populated)
	beforeMax := maxID(t, populated)
	if err := populated.Import([]SeedBatch{{Todo, nil}, {Done, []string{}}}); err != nil {
		t.Errorf("Import of empty batches on a populated board: %v", err)
	}
	if after := mustList(t, populated); !boardEqual(before, after) {
		t.Errorf("empty import changed the board:\n before: %s\n after:  %s",
			flatten(before), flatten(after))
	}
	probe, err = populated.Create("id probe")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if probe.ID != beforeMax+1 {
		t.Errorf("identifier after empty import = %d, want %d — a no-op consumes no ids",
			probe.ID, beforeMax+1)
	}
}

// Imported lists extend existing columns exactly as Seed does: residents
// keep their cells, imported texts land below them in the lists' order,
// columns the batches never name stay cell-for-cell untouched, every
// imported identifier is fresh above the pre-import maximum, and positions
// stay contiguous 0..n-1 per column.
func TestImportAppendsBelowExisting(t *testing.T) {
	store := openStore(t)
	seedBoard(t, store, []columnFixture{
		{Todo, []string{"t0", "t1"}},
		{Done, []string{"d0"}},
	})
	before := mustList(t, store)
	beforeMax := maxID(t, store)

	batches := []SeedBatch{
		{Todo, []string{"i0", "i1"}},
		{Done, []string{"d1"}},
		{InProgress, []string{"p0"}},
	}
	if err := store.Import(batches); err != nil {
		t.Fatalf("Import: %v", err)
	}

	after := mustList(t, store)
	assertPlacements(t, after, []placement{
		{"t0", Todo, 0}, {"t1", Todo, 1}, {"i0", Todo, 2}, {"i1", Todo, 3},
		{"p0", InProgress, 0},
		{"d0", Done, 0}, {"d1", Done, 1},
	})
	// The tail of each batched column is exactly what the import appended.
	for _, batch := range batches {
		col := after[columnIndex(t, after, batch.Column)]
		for _, c := range col.Cards[len(col.Cards)-len(batch.Texts):] {
			if c.ID <= beforeMax {
				t.Errorf("imported card %q carries id %d, want fresh — above the pre-import maximum %d",
					c.Title, c.ID, beforeMax)
			}
		}
	}
	doneBefore := before[columnIndex(t, before, Done)]
	doneAfter := after[columnIndex(t, after, Done)]
	if len(doneAfter.Cards) != 2 || doneAfter.Cards[0] != doneBefore.Cards[0] {
		t.Errorf("resident done card = %+v, want %+v at its cell — residents keep their places",
			doneAfter.Cards[0], doneBefore.Cards[0])
	}
	assertContiguous(t, after)
}

// The imported state is ordinary stored state: a board built entirely
// through Import survives Close and reopen at the same path cell-for-cell,
// reusing the board/13 machinery — the import commits through the same
// schema every other operation reads back.
func TestImportedStateSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "board.db")
	store := openStoreAt(t, path)

	batches := []SeedBatch{
		{Todo, []string{"m1", "m2", "m3"}},
		{InProgress, []string{"m4"}},
		{Done, []string{"m5", "m6"}},
	}
	if err := store.Import(batches); err != nil {
		t.Fatalf("Import: %v", err)
	}
	before := mustList(t, store)
	assertPlacements(t, before, []placement{
		{"m1", Todo, 0}, {"m2", Todo, 1}, {"m3", Todo, 2},
		{"m4", InProgress, 0},
		{"m5", Done, 0}, {"m6", Done, 1},
	})

	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	after := mustList(t, openStoreAt(t, path))
	if !boardEqual(before, after) {
		t.Errorf("reopened imported board differs from the closed one:\n before: %s\n after:  %s",
			flatten(before), flatten(after))
	}
}
