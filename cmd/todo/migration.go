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
// The guard is the card's "the board has never been created", observed the
// only way that also satisfies server/05: the board holds no cards at all.
// A board with any card has been created — or already imported — so its todo
// source is evidence of a finished migration and is ignored; that is the
// once in "import once". Emptiness is also crash-safe: Import commits in one
// transaction, so no half import is ever observable to mistake for a created
// board — an import that never landed leaves the board empty, and the next
// start completes it (server/05). The workplan's "presence of the board
// schema" wording cannot distinguish "created but never seeded" from
// "created and served", which is exactly the half state its own risk section
// refuses to leave behind.
//
// Failures are loud, per the workplan's assumption and risk mitigation: a
// source that exists but cannot be read, or whose rows fail the board's own
// text rule inside Import, fails startup with a stated error before anything
// serves (main prints and exits non-zero). Import is all-or-nothing, so the
// failure leaves at most an empty board file — a clean board with no trace
// of a partial import — never a half board. The server never invents data:
// the empty board only serves after the user removes or repairs the file.
//
// An absent source is not a failure: it is the "start without todo data"
// path (card server/03) and the board simply starts. The source file itself
// is never written — the reader opens it mode=ro, enforced below the Go
// layer by SQLite.
func importTodos(store *board.Store, todoPath string) error {
	// Guard first: reading is only interesting while the board has never
	// been created.
	listing, err := store.List()
	if err != nil {
		return fmt.Errorf("import guard: %w", err)
	}
	for _, column := range listing {
		if len(column.Cards) > 0 {
			return nil // board created (or already migrated) — source ignored
		}
	}

	// The board is fresh: import if there is a todo source at the path the
	// composition root owns. Absent means nothing to import.
	if _, err := os.Stat(todoPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("todo source %s: %w", todoPath, err)
	}

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

	// One call, one transaction: the board gains every card in order or
	// nothing at all. A poisoned source row (blank or over-long title) is
	// refused here by the board's own text rule, before any statement —
	// the loud failure the workplan states, with the board left exactly as
	// it was.
	if err := store.Import([]board.SeedBatch{
		{Column: board.Todo, Texts: todoTexts},
		{Column: board.Done, Texts: doneTexts},
	}); err != nil {
		return fmt.Errorf("import todos from %s: %w", todoPath, err)
	}
	return nil
}
