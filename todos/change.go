package todos

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ErrNotFound is the module's typed not-found outcome: no todo carries the
// given identifier. Shared by every operation whose subject may be gone
// (Change, Delete). Outcomes stay free of HTTP semantics — that mapping is
// the api module's decision (workplan decision).
var ErrNotFound = errors.New("todos: no such todo")

// ErrNoFields is the typed invalid outcome for a Change that supplies
// neither title nor done — an empty change is rejected before any lookups.
var ErrNoFields = errors.New("todos: change supplies no fields")

// ErrDoneFrozen is the module's typed done-frozen outcome: a done todo's
// text is frozen, so a Change carrying a title for a done todo is refused
// before any write. Only a done-only change (reopen) unlocks editing again
// (store workplan decision — the refusal is server truth a stale page
// cannot outvote). Like every outcome here it carries no HTTP semantics.
var ErrDoneFrozen = errors.New("todos: done todo, title is frozen")

// ChangeFields carries the optional fields of a Change operation: at least
// one must be supplied. A nil field means "leave this column alone"; the
// update touches only the supplied columns, so a done-only change never
// rewrites the title and a title-only change never touches done.
type ChangeFields struct {
	Title *string
	Done  *bool
}

// Change applies the supplied fields to the todo with the given identifier
// and returns the updated todo. The write is committed before the result
// returns.
func (s *Store) Change(id int64, fields ChangeFields) (Todo, error) {
	var sets []string
	var args []any
	if fields.Title != nil {
		sets = append(sets, "title = ?")
		args = append(args, *fields.Title)
	}
	if fields.Done != nil {
		sets = append(sets, "done = ?")
		done := 0
		if *fields.Done {
			done = 1
		}
		args = append(args, done)
	}
	if len(sets) == 0 {
		return Todo{}, ErrNoFields
	}
	// Frozen-text rule (todos/06): a title for a done todo is refused BEFORE
	// any column update (store workplan data flow). The pre-read carries both
	// the done check and not-found for the title direction; the done-only
	// direction keeps the UPDATE's affected-row count as its not-found check.
	if fields.Title != nil {
		var doneFlag int
		err := s.db.QueryRow(`SELECT done FROM todos WHERE id = ?`, id).Scan(&doneFlag)
		if errors.Is(err, sql.ErrNoRows) {
			return Todo{}, ErrNotFound
		}
		if err != nil {
			return Todo{}, fmt.Errorf("change todo %d: %w", id, err)
		}
		if doneFlag != 0 {
			return Todo{}, ErrDoneFrozen
		}
	}
	query := fmt.Sprintf(`UPDATE todos SET %s WHERE id = ?`, strings.Join(sets, ", "))
	res, err := s.db.Exec(query, append(args, id)...)
	if err != nil {
		return Todo{}, fmt.Errorf("change todo %d: %w", id, err)
	}
	// The affected-row count distinguishes not-found: nothing matched, so
	// nothing was written either (workplan data flow).
	if n, err := res.RowsAffected(); err != nil {
		return Todo{}, fmt.Errorf("change todo %d: %w", id, err)
	} else if n == 0 {
		return Todo{}, ErrNotFound
	}
	return s.get(id)
}

// get reads one todo row by identifier.
func (s *Store) get(id int64) (Todo, error) {
	var t Todo
	err := s.db.QueryRow(`SELECT id, title, done FROM todos WHERE id = ?`, id).
		Scan(&t.ID, &t.Title, &t.Done)
	if err != nil {
		return Todo{}, fmt.Errorf("read todo %d: %w", id, err)
	}
	return t, nil
}
