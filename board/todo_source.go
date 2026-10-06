// todo_source.go is the migration-source reader — slated for deletion after
// KW6, alongside the composition root's one-time import it serves. It is not
// part of the board contract: it lives outside Store, creates no schema, and
// writes nothing.
//
// It exists in this module, and not in the composition root that runs the
// migration, because the repo's architecture gate (server/08) makes board
// the sole home of the SQLite driver — the only data-file owner. The reader
// is the mechanical door through that pin; the migration policy itself
// (guard, mapping, order, failure handling) stays in cmd/todo, where startup
// policy lives.

package board

import (
	"database/sql"
	"fmt"
	"path/filepath"

	_ "modernc.org/sqlite" // driver already imported by this module (store.go)
)

// TodoRow is one row of the superseded todo data file — the reader's narrow
// shape, the old table's three columns verbatim (todos(id, title, done),
// git 48a4ca5^:todos/store.go). It is not a Card and carries no board
// meaning: no column, no position, no validation. The caller decides what a
// row becomes.
type TodoRow struct {
	ID    int64
	Title string
	Done  bool
}

// ReadTodoSource opens the superseded todo data file at path strictly
// read-only and returns its rows in creation order — ascending id, the order
// the old store's List answered and therefore the order the rows were
// created. Opening with mode=ro makes the "never modified" guarantee
// structural rather than conventional: an absent file fails instead of being
// created, and any write attempt is refused by SQLite itself. A file that
// exists but is not the old store (no todos table) fails loudly here — the
// migration never guesses at a shape.
//
// The one-shot reader takes the path from the composition root and keeps
// nothing open: no handle outlives the call, no schema is touched, the file
// is left byte-for-byte as found. Slated for deletion once the one-time
// migration retires after KW6.
func ReadTodoSource(path string) ([]TodoRow, error) {
	// Absolute temp/home paths are valid SQLite URIs as-is on unix; the
	// reader leans on that minimal shape because it is throwaway code.
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("open todo source %q read-only: %w", path, err)
	}
	defer db.Close()
	// Same single-writer triviality as every other opener here.
	db.SetMaxOpenConns(1)

	rows, err := db.Query(`SELECT id, title, done FROM todos ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("read todo source %q: %w", path, err)
	}
	defer rows.Close()

	var out []TodoRow
	for rows.Next() {
		var r TodoRow
		if err := rows.Scan(&r.ID, &r.Title, &r.Done); err != nil {
			return nil, fmt.Errorf("read todo source %q: %w", path, err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read todo source %q: %w", path, err)
	}
	return out, nil
}
