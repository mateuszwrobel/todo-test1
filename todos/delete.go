package todos

import "fmt"

// Delete removes the todo with the given identifier. The write is committed
// before the result returns. Identifiers are never reused after deletion:
// the schema's autoincrement rowids only move forward, so a later Create
// cannot hand back a deleted todo's identifier. When no todo carries the
// identifier, the outcome is the module's shared ErrNotFound (declared in
// change.go alongside the other typed outcomes) and no state changes.
func (s *Store) Delete(id int64) error {
	res, err := s.db.Exec(`DELETE FROM todos WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete todo %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete todo %d: %w", id, err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
