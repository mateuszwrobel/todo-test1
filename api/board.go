package api

import (
	"encoding/json"
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
// board module's own JSON tags (id, title, column, position). A List failure
// is the stated-failure path of every store read here: 500 (mirroring the
// todo list handler's store-failure convention).
func handleBoard(store BoardStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := store.List()
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
