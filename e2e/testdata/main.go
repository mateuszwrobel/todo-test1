// Command seed is an e2e test-support tool: it seeds a kanban board data
// file with given titles and places some of them in the In Progress and
// Done columns. It is not served application code — the app itself gains
// no seeding behavior from this file.
//
// Cards are appended via the board store's Create (the seed primitive,
// board/02), which puts every card at the bottom of the todo column. The
// column placement then goes straight to the data file with the SQLite
// driver because the store's move operation is a later-wave feature (KW5);
// the e2e lane needs cards in all three columns before that contract
// exists — the same precedent as the superseded todo seed's done-marking.
// Positions stay contiguous 0..n-1 per column, assigned in creation order
// within each destination column.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"todo/board"

	_ "modernc.org/sqlite"
)

func main() {
	dbPath := flag.String("db", "kanban.db", "board data file path")
	titles := flag.String("titles", "", "comma-separated card titles, in creation order")
	inProgressIdx := flag.String("in-progress", "", "comma-separated indexes (into --titles) to place in In Progress")
	doneIdx := flag.String("done", "", "comma-separated indexes (into --titles) to place in Done")
	flag.Parse()

	inProgress := indexSet("in-progress", *inProgressIdx, len(splitList(*titles)))
	done := indexSet("done", *doneIdx, len(splitList(*titles)))
	for i := range inProgress {
		if inProgress[i] && done[i] {
			fail(fmt.Errorf("index %d is in both --in-progress and --done", i))
		}
	}

	store, err := board.Open(*dbPath)
	if err != nil {
		fail(err)
	}
	type placement struct {
		id   int64
		dest board.Column
	}
	var plan []placement
	for i, title := range splitList(*titles) {
		created, err := store.Create(title)
		if err != nil {
			fail(err)
		}
		dest := board.Todo
		if inProgress[i] {
			dest = board.InProgress
		}
		if done[i] {
			dest = board.Done
		}
		plan = append(plan, placement{id: created.ID, dest: dest})
	}
	store.Close()

	// Apply placements per destination column, in creation order, so each
	// column's positions come out contiguous 0..n-1 top-to-bottom.
	db, err := sql.Open("sqlite", *dbPath)
	if err != nil {
		fail(err)
	}
	defer db.Close()
	for _, col := range []board.Column{board.Todo, board.InProgress, board.Done} {
		position := 0
		for _, card := range plan {
			if card.dest != col {
				continue
			}
			if _, err := db.Exec(`UPDATE cards SET "column" = ?, position = ? WHERE id = ?`,
				string(col), position, card.id); err != nil {
				fail(err)
			}
			position++
		}
	}
}

// indexSet parses a comma-separated index list into a presence set over
// [0, n); every entry must be a valid in-range index.
func indexSet(flagName, value string, n int) []bool {
	set := make([]bool, n)
	for _, raw := range splitList(value) {
		idx, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil || idx < 0 || idx >= n {
			fail(fmt.Errorf("%s index %q out of range", flagName, raw))
		}
		set[idx] = true
	}
	return set
}

func splitList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return strings.Split(s, ",")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "seed:", err)
	os.Exit(1)
}
