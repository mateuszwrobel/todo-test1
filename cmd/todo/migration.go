// migration.go is the composition root's one-time todo import (KW6 server/02,
// server/05) — startup policy, slated for deletion after the migration
// retires post-KW6. It decides WHAT imports and WHEN: the reading door
// (board.ReadTodoSource) and the writing door (Store.Import, one transaction)
// own their mechanics; the policy below owns the guard, the mapping, and the
// failure decision. Nothing here is reachable after startup returns.

package main

import (
	"fmt"
	"os"

	"todo/board"
)

// importTodos runs the one-time migration against an already-open board:
// read the superseded todo data file (if the guard and the file's presence
// call for it), partition by done state in creation order, and place both
// lists through ONE Store.Import call so the whole board lands in a single
// transaction or not at all.
//
// The guard is the card's "the board has never been created", observed
// through the board's import marker (Store.Imported): the durable record of
// this board file's first-ever import decision. A marker on record means the
// decision is finished forever — the todo source is then past evidence and
// is ignored whatever the board holds. Emptiness cannot stand in for the
// marker: a user who imported at first start and later deleted every card
// would see the deleted todos resurrect with fresh identifiers on the next
// start, breaking "no card is re-imported" (server/02). And the workplan's
// "presence of the board schema" wording cannot work at all here: Open
// commits the schema before the import runs, so an interrupted import would
// be mistaken for a created board and could never complete (server/05). The
// marker answers both cards exactly: Import commits it in the SAME
// transaction as the cards, so an import that never landed recorded nothing
// and the next start completes it, while an import that landed is recorded
// forever.
//
// A first decision that imports nothing still decides: with no source (or a
// source holding no rows) the start records the marker alone, so a source
// appearing after the board was created and served is evidence of a finished
// migration, never migration material — that is the once in "import once",
// independent of how full the board happens to be.
//
// Failures are loud, per the workplan's assumption and risk mitigation: a
// source that exists but cannot be read, or whose rows fail the board's own
// text rule inside Import, fails startup with a stated error before anything
// serves (main prints and exits non-zero). Import is all-or-nothing and the
// marker rides its transaction, so the failure leaves at most an empty
// board file with no marker — a clean board, no trace of a partial import,
// and a guard that retries on the next start. The server never invents data:
// the empty board only serves after the user removes or repairs the file.
// The loud failure therefore never reaches the marker-recording line below.
//
// An absent source is not a failure: it is the "start without todo data"
// path (card server/03) and the board simply starts, marked as decided. The
// source file itself is never written — the reader opens it mode=ro,
// enforced below the Go layer by SQLite.
func importTodos(store *board.Store, todoPath string) error {
	// Guard first: the marker answers whether this board file has ever run
	// the import decision. On record — stop; the decision is finished,
	// whatever the board holds or ever held.
	done, err := store.Imported()
	if err != nil {
		return fmt.Errorf("import guard: %w", err)
	}
	if done {
		return nil
	}

	// The first-ever decision. A source at the path the composition root
	// owns is migration material; absent means nothing to import.
	if _, err := os.Stat(todoPath); err == nil {
		rows, err := board.ReadTodoSource(todoPath)
		if err != nil {
			return err
		}

		// Mapping (the workplan's data flow): not-done first, in creation
		// order, top-to-bottom into todo; done likewise into done. In-progress
		// is invented from nothing — the old store never had one.
		var todoTexts, doneTexts []string
		for _, row := range rows {
			if row.Done {
				doneTexts = append(doneTexts, row.Title)
			} else {
				todoTexts = append(todoTexts, row.Title)
			}
		}

		// One call, one transaction: the board gains every card in order
		// — and the marker — or nothing at all. A poisoned source row
		// (blank or over-long title) is refused here by the board's own
		// text rule, before any statement — the loud failure the workplan
		// states, with the board left exactly as it was and no marker
		// recorded.
		if err := store.Import([]board.SeedBatch{
			{Column: board.Todo, Texts: todoTexts},
			{Column: board.Done, Texts: doneTexts},
		}); err != nil {
			return fmt.Errorf("import todos from %s: %w", todoPath, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("todo source %s: %w", todoPath, err)
	}

	// The first decision is complete — record it so no later start ever
	// re-runs it. After a successful Import this is the marker the import
	// itself committed (the write is idempotent); on a decision that
	// imported nothing it is the decision's whole trace. A failed import
	// never reaches this line: startup is already dying loudly above, and
	// the unmarked board retries on the next start (server/05).
	return store.MarkImported()
}
