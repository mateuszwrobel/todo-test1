package board

import (
	"database/sql"
	"errors"
	"fmt"

	"todo/users" // the roster the filter keyword is validated against
)

// UnassignedKeyword is the filter keyword's spelling of "cards with no
// assignee" — the sentinel the contract shares across surfaces: the workplan
// amendment states the filtered read's keyword set as "exact roster name or
// a sentinel for unassigned" (workplan_board_store.md, Database → Data Flow:
// `assignee = <name>` or `assignee IS NULL` keyword "unassigned"), the GET
// /board query says "unassigned" (api/14), and the PATCH move pair's
// "within" says the same word (api/15). No roster name can collide with it:
// the cast is fixed (Ada, Grace, Alan, Barbara, Linus — users contract), and
// validity screens every other keyword through users.IsMember. The keyword is
// distinct from Card.Assignee's in-memory sentinel: the record spells
// unassigned as the empty string (a stored NULL), the FILTER spells it
// "unassigned"; filterMatcher maps one onto the other at the boundary.
const UnassignedKeyword = "unassigned"

// ErrInvalidSlot reports a filtered-slot move aimed at a negative slot — the
// store's "slot invalid" outcome. Where Move is total over positions (its
// documented clamp, board/store.go), the FILTERED slot is a count among
// matching cards and a negative count is a request defect, not an end of the
// column: the guard answers before the transaction opens, so a rejected slot
// touches no storage. Callers map it with errors.Is.
var ErrInvalidSlot = errors.New("board: slot must be a non-negative index")

// filterCondition translates a filter keyword into the SQL condition its
// matching rows satisfy — the single spelling of the keyword's meaning for
// reads (ListFiltered) and for the filtered-slot move's matching-ids query
// alike. The clause is one of two module-internal literals; the keyword never
// reaches the SQL itself, only the bound name parameter when there is one.
// An unknown keyword (not a roster name, not the unassigned sentinel) is the
// named outcome ErrUnknownAssignee with no query involved: like Change's
// assignee direction, roster validity is the users contract's rule screened
// here before anything else is consulted (board/17's rank).
func filterCondition(keyword string) (clause string, arg any, err error) {
	switch {
	case keyword == UnassignedKeyword:
		return `assignee IS NULL`, nil, nil
	case users.IsMember(keyword):
		return `assignee = ?`, keyword, nil
	default:
		return "", nil, fmt.Errorf("filter %q is neither a roster name nor %q: %w",
			keyword, UnassignedKeyword, ErrUnknownAssignee)
	}
}

// ListFiltered is List narrowed by an assignee keyword: the same fixed
// three-column shape in the same fixed order, every column holding ONLY the
// cards matching the keyword (exact roster name, or UnassignedKeyword for
// cards with no assignee). Stored order and stored positions pass through
// unchanged — the filter removes cells, it rewrites nothing (workplan
// amendment: "stored positions and order pass through unchanged"). That
// means a filtered column's positions can show gaps: the cards arrive at
// their absolute positions, exactly the cells List would show with the
// non-matching ones cut out. An empty match is an empty column, the same nil
// slice List answers for a column holding nothing. An unknown keyword is the
// named outcome ErrUnknownAssignee before any query runs; the read itself is
// SELECT-only, so no keyword — known or not — ever changes the board.
func (s *Store) ListFiltered(keyword string) ([]ColumnCards, error) {
	clause, arg, err := filterCondition(keyword)
	if err != nil {
		return nil, fmt.Errorf("list filtered board: %w", err)
	}
	query := fmt.Sprintf(
		`SELECT id, title, "column", position, assignee FROM cards WHERE "column" = ? AND %s ORDER BY position ASC`,
		clause,
	)
	board := make([]ColumnCards, 0, len(columns))
	for _, col := range columns {
		args := []any{string(col)}
		if arg != nil {
			args = append(args, arg)
		}
		rows, err := s.db.Query(query, args...)
		if err != nil {
			return nil, fmt.Errorf("list filtered board: %w", err)
		}
		var cards []Card
		for rows.Next() {
			var c Card
			var storedAssignee sql.NullString
			if err := rows.Scan(&c.ID, &c.Title, &c.Column, &c.Position, &storedAssignee); err != nil {
				rows.Close()
				return nil, fmt.Errorf("list filtered board: %w", err)
			}
			c.Assignee = storedAssignee.String
			cards = append(cards, c)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("list filtered board: %w", err)
		}
		rows.Close()
		board = append(board, ColumnCards{Name: col, Cards: cards})
	}
	return board, nil
}

// MoveFiltered is Move's filtered sibling (card board/19): the card lands at
// a slot counted among the MATCHING cards only of the target column — the
// operation behind a drag made while a filter is on, where the visible list
// is a subset of the column. Matching means the same keyword semantics as
// ListFiltered: exact roster name, or UnassignedKeyword for unassigned.
//
// Slot resolution (workplan amendment, Database → Data Flow: "the
// filtered-slot move resolves its slot against the matching rows of the
// target column and normalizes contiguity whole-column"):
//
//   - The moved card is REMOVED from the target first, exactly like Move —
//     cross-column departure closes the source gap first; a same-column card
//     is simply absent from the sequence the slot is counted against.
//   - Of the remaining sequence, the card must end up with exactly `slot`
//     matching cards before it. The EARLIEST such absolute position is the
//     landing spot: immediately after the matching card that occupies slot
//     slot-1 (so slot 0 sits at index 0, the column's front), with the slot
//     clamped to the remaining matching count — a slot at or past it behaves
//     like the last matching slot: right after the last remaining match,
//     Move's end-of-range totality expressed against the matching cards
//     rather than the whole column. When nothing matches — the amendment's
//     stated edge ("slot 0 into a column with no matching cards places the
//     card at that column's front") — every slot resolves to index 0, the
//     front, because index 0 is the earliest position with zero matching
//     cards before it and no larger slot is satisfiable at all.
//   - Non-matching cards keep their relative order cell-for-cell: the splice
//     shifts a tail, it never reorders — and every column OUTSIDE the target
//     is untouched except the departed-from column's gap close, which is
//     Move's own renormalization.
//   - Positions are renormalized contiguous 0..n-1 WHOLE-COLUMN, so the
//     returned card carries the ABSOLUTE position the slot resolved to.
//
// Validation is board-style and all of it answers before any write, in
// Change's rank: keyword validity through the users contract FIRST (an
// unknown filter is a request defect — ErrUnknownAssignee outranks the
// column enum, the slot guard and the not-found lookup, exactly as board/17
// ranks the assignee direction), then the target column enum
// (ErrInvalidColumn), then the slot's non-negativity (ErrInvalidSlot), then
// inside the transaction the existence read (ErrCardNotFound). Like Move,
// this is placement and NOTHING ELSE: no title, no assignee direction, so
// the done freeze never fires — a Done card filtered-moves out exactly as it
// plain-moves out, and an assigned card arrives still assigned. A slot past
// the matching count clamps as described; a negative slot is refused.
func (s *Store) MoveFiltered(id int64, column Column, slot int, keyword string) (Card, error) {
	clause, arg, err := filterCondition(keyword)
	if err != nil {
		return Card{}, fmt.Errorf("filtered move card %d: %w", id, err)
	}
	if !validColumn(column) {
		return Card{}, fmt.Errorf("filtered move card %d: column %q is not todo, in_progress, or done: %w",
			id, string(column), ErrInvalidColumn)
	}
	if slot < 0 {
		return Card{}, fmt.Errorf("filtered move card %d: slot %d is negative: %w", id, slot, ErrInvalidSlot)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return Card{}, fmt.Errorf("filtered move card %d: %w", id, err)
	}
	defer tx.Rollback() // no-op after Commit

	var card Card
	var storedAssignee sql.NullString
	err = tx.QueryRow(
		`SELECT id, title, "column", position, assignee FROM cards WHERE id = ?`, id,
	).Scan(&card.ID, &card.Title, &card.Column, &card.Position, &storedAssignee)
	if errors.Is(err, sql.ErrNoRows) {
		return Card{}, fmt.Errorf("filtered move card %d: %w", id, ErrCardNotFound)
	}
	if err != nil {
		return Card{}, fmt.Errorf("filtered move card %d: %w", id, err)
	}
	card.Assignee = storedAssignee.String

	// Departure first for a cross-column move (Move's pattern): the flip lets
	// the source renormalization see the card as gone and close its gap.
	if column != card.Column {
		if _, err := tx.Exec(`UPDATE cards SET "column" = ? WHERE id = ?`, string(column), id); err != nil {
			return Card{}, fmt.Errorf("filtered move card %d: %w", id, err)
		}
		if err := renormalize(tx, card.Column); err != nil {
			return Card{}, fmt.Errorf("filtered move card %d: %w", id, err)
		}
	}

	// The target's remaining cards top-to-bottom, the moved card excluded,
	// and the ids among them the keyword matches — the slot is counted
	// against that matching list, in column order.
	restQuery := `SELECT id FROM cards WHERE "column" = ? AND id <> ? ORDER BY position ASC, id ASC`
	restArgs := []any{string(column), id}
	rows, err := tx.Query(restQuery, restArgs...)
	if err != nil {
		return Card{}, fmt.Errorf("filtered move card %d: %w", id, err)
	}
	var rest []int64
	restIndex := map[int64]int{}
	for rows.Next() {
		var other int64
		if err := rows.Scan(&other); err != nil {
			rows.Close()
			return Card{}, fmt.Errorf("filtered move card %d: %w", id, err)
		}
		restIndex[other] = len(rest)
		rest = append(rest, other)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Card{}, fmt.Errorf("filtered move card %d: %w", id, err)
	}
	rows.Close()

	matchingQuery := fmt.Sprintf(
		`SELECT id FROM cards WHERE "column" = ? AND %s AND id <> ? ORDER BY position ASC, id ASC`,
		clause,
	)
	matchingArgs := []any{string(column)}
	if arg != nil {
		matchingArgs = append(matchingArgs, arg)
	}
	matchingArgs = append(matchingArgs, id)
	rows, err = tx.Query(matchingQuery, matchingArgs...)
	if err != nil {
		return Card{}, fmt.Errorf("filtered move card %d: %w", id, err)
	}
	var matching []int64
	for rows.Next() {
		var other int64
		if err := rows.Scan(&other); err != nil {
			rows.Close()
			return Card{}, fmt.Errorf("filtered move card %d: %w", id, err)
		}
		matching = append(matching, other)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Card{}, fmt.Errorf("filtered move card %d: %w", id, err)
	}
	rows.Close()

	// Resolve the slot to the earliest absolute index with `slot` matching
	// cards before it (see the doc above): clamp the slot to the matching
	// count, then land immediately after the matching card that precedes
	// the slot — index 0 when the clamped slot is 0, which covers both
	// "slot 0" and "column with no matching cards at all", so the
	// amendment's front rule falls out of the one formula instead of a
	// special case.
	k := slot
	if k > len(matching) {
		k = len(matching)
	}
	insertion := 0
	if k > 0 {
		insertion = restIndex[matching[k-1]] + 1
	}

	ordered := make([]int64, 0, len(rest)+1)
	ordered = append(ordered, rest[:insertion]...)
	ordered = append(ordered, id)
	ordered = append(ordered, rest[insertion:]...)

	// Whole-column normalization: every remaining card takes its place in
	// the merged sequence (a write only where the value actually moves),
	// and the moved card's absolute position is its splice index — the
	// number the contract's payload states back.
	for pos, placed := range ordered {
		if placed == id {
			continue
		}
		if _, err := tx.Exec(`UPDATE cards SET position = ? WHERE id = ?`, pos, placed); err != nil {
			return Card{}, fmt.Errorf("filtered move card %d: %w", id, err)
		}
	}
	if _, err := tx.Exec(
		`UPDATE cards SET "column" = ?, position = ? WHERE id = ?`, string(column), insertion, id,
	); err != nil {
		return Card{}, fmt.Errorf("filtered move card %d: %w", id, err)
	}
	card.Column, card.Position = column, insertion

	if err := tx.Commit(); err != nil {
		return Card{}, fmt.Errorf("filtered move card %d: %w", id, err)
	}
	return card, nil
}
