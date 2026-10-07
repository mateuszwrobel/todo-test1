package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"todo/board"
)

// columnTitles maps the board's fixed column set to the contract's display
// titles. Display titles belong to the contract layer, not the board module
// (whose enum is storage), so this mapping has exactly one owner: here.
var columnTitles = map[board.Column]string{
	board.Todo:       "To Do",
	board.InProgress: "In Progress",
	board.Done:       "Done",
}

// boardResponse is the GET /board contract body: the columns array in the
// fixed order todo, in_progress, done (the order board.List guarantees), each
// entry carrying its display title and its cards in position order.
type boardResponse struct {
	Columns []boardColumnResponse `json:"columns"`
}

type boardColumnResponse struct {
	Title string       `json:"title"`
	Cards []board.Card `json:"cards"`
}

// handleBoard maps GET /board to the board store's List: one call, translated
// into the contract shape, answered 200. The Card fields encode from the
// board module's own JSON tags and MarshalJSON — the contract's five fields
// (id, title, column, position, assignee; api/13 put assignee on every card
// payload as name or null). A List failure
// is the stated-failure path of every store read here: 500 (mirroring the
// todo list handler's store-failure convention).
//
// The ?assignee= parameter (card api/14, workplan amendment 2026-10-07)
// narrows that answer through the store's filtered read, and the translation
// stays as thin as the unfiltered leg:
//
//   - parameter ABSENT → store.List, this handler's shipped call — the no-
//     param payload is byte-unchanged from before the increment, which is
//     what the card's last clause demands ("the answer is the full board,
//     unchanged from today");
//   - parameter PRESENT → store.ListFiltered with the value as the filter
//     keyword: an exact roster name or the literal "unassigned" answers the
//     same board shape with only the matching cards at their stored
//     positions, columns preserved;
//   - a value outside the roster is board's ErrUnknownAssignee (validity is
//     the users module's rule screened by board, never restated here) and
//     maps to the contract's stated 422 {"error":"unknown user"} — the same
//     message the PATCH legs state, errorJSON being the one body site;
//   - any other filtered-read failure keeps this module's store-failure
//     500, worded as List's.
//
// A present-but-empty value (?assignee=) is not an absent one: the empty
// string is neither a roster name nor the sentinel, so it is an unknown
// value and answers 422. With the parameter repeated, the first value is the
// query's answer (net/http's convention, not a contract statement).
func handleBoard(store BoardStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var list []board.ColumnCards
		var err error
		if values, present := r.URL.Query()["assignee"]; present {
			list, err = store.ListFiltered(values[0])
			if errors.Is(err, board.ErrUnknownAssignee) {
				errorJSON(w, http.StatusUnprocessableEntity, "unknown user")
				return
			}
		} else {
			list, err = store.List()
		}
		if err != nil {
			http.Error(w, "failed to list board", http.StatusInternalServerError)
			return
		}
		out := boardResponse{Columns: make([]boardColumnResponse, 0, len(list))}
		for _, col := range list {
			cards := col.Cards
			if cards == nil {
				cards = []board.Card{} // the contract's empty list is [], never null
			}
			out.Columns = append(out.Columns, boardColumnResponse{
				Title: columnTitles[col.Name],
				Cards: cards,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(out); err != nil {
			return // status already sent; nothing left to state
		}
	}
}
