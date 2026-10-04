package todos

import "fmt"

// Delete removes the todo with the given identifier. The write is committed
// before the result returns. Identifiers are never reused after deletion:
// the schema's autoincrement rowids only move forward, so a later Create
// cannot hand back a deleted todo's identifier.
func (s *Store) Delete(id int64) error {
	if _, err := s.db.Exec(`DELETE FROM todos WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete todo %d: %w", id, err)
	}
	return nil
}
