// Package api exposes the application's JSON wire contract and translates
// HTTP requests into board and todo module operations. It owns no listener
// and never renders HTML. The board endpoints are the kanban contract
// (workplan_api_board.md); the surviving /todos deletion serves the
// superseded todo surface until its retirement card lands.
package api

import (
	"net/http"

	"todo/board"
	"todo/todos"
)

// TodoStore is the minimal port this module declares for itself (consumer-
// defined port): the concrete store is injected by the composition root, so
// no module depends on another's concrete type. Create left the port when
// POST /todos retired (api/02) and Change when PATCH /todos retired (api/05)
// — the board endpoints replaced those surfaces; the survivor is the delete.
type TodoStore interface {
	List() ([]todos.Todo, error)
	Delete(id int64) error
}

// BoardStore is this module's port onto the board contract: the read the
// GET /board endpoint translates, the create POST /cards translates, the
// change PATCH /cards/{id} translates, and the delete DELETE /cards/{id}
// translates — the board operations the endpoints exist to reach, nothing
// more. Consumer-defined like TodoStore; the composition root injects the
// concrete store.
type BoardStore interface {
	List() ([]board.ColumnCards, error)
	Create(text string) (board.Card, error)
	Change(id int64, title *string, column *board.Column) (board.Card, error)
	Delete(id int64) error
}

// NewHandler builds the handler for the JSON contract: GET /board,
// POST /cards, PATCH /cards/{id}, and DELETE /cards/{id} over the board
// store, DELETE /todos over the todo store (GET /todos retired at api/01,
// POST /todos at api/02, PATCH /todos at api/05 — the board endpoints
// replace the todo surface). The composition root mounts it on its listener.
func NewHandler(todoStore TodoStore, boardStore BoardStore) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /board", handleBoard(boardStore))
	mux.HandleFunc("POST /cards", handleCreate(boardStore))
	mux.HandleFunc("PATCH /cards/{id}", handleCardChange(boardStore))
	mux.HandleFunc("DELETE /cards/{id}", handleCardDelete(boardStore))
	mux.HandleFunc("DELETE /todos/{id}", handleDelete(todoStore))
	return mux
}
