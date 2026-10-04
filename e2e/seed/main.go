// Command seed is an e2e test-support tool: it seeds a todo data file with
// given titles and marks some of them done. It is not served application
// code — the app itself gains no seeding behavior from this file.
//
// Marking done goes straight to the data file with the SQLite driver because
// the store's Change operation is a later-wave feature; W1 needs a mixed
// done/not-done seed without that contract existing yet.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"todo/todos"

	_ "modernc.org/sqlite"
)

func main() {
	dbPath := flag.String("db", "todos.db", "data file path")
	titles := flag.String("titles", "", "comma-separated todo titles, in creation order")
	doneIdx := flag.String("done", "", "comma-separated indexes (into --titles) to mark done")
	flag.Parse()

	store, err := todos.Open(*dbPath)
	if err != nil {
		fail(err)
	}
	var ids []int64
	for _, title := range splitList(*titles) {
		created, err := store.Create(title)
		if err != nil {
			fail(err)
		}
		ids = append(ids, created.ID)
	}
	store.Close()

	if marks := splitList(*doneIdx); len(marks) > 0 {
		db, err := sql.Open("sqlite", *dbPath)
		if err != nil {
			fail(err)
		}
		defer db.Close()
		for _, idx := range marks {
			n, err := strconv.Atoi(idx)
			if err != nil || n < 0 || n >= len(ids) {
				fail(fmt.Errorf("done index %q out of range", idx))
			}
			if _, err := db.Exec(`UPDATE todos SET done = 1 WHERE id = ?`, ids[n]); err != nil {
				fail(err)
			}
		}
	}
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
