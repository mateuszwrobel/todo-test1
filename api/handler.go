// Package api exposes the application's JSON wire contract and translates
// HTTP requests into board and todo module operations. It owns no listener
// and never renders HTML. The board endpoints are the kanban contract
// (workplan_api_board.md); the remaining /todos mutation endpoints serve the
// superseded todo surface until their retirement cards land.
package api

import (
	"net/http"

	"todo/board"
	"todo/todos"
)

// TodoStore is the minimal port this module declares for itself (consumer-
// defined port): the concrete store is injected by the composition root, so
// no module depends on another's concrete type.
type TodoStore interface {
	List() ([]todos.Todo, error)
	Create(title string) (todos.Todo, error)
	Change(id int64, fields todos.ChangeFields) (todos.Todo, error)
	Delete(id int64) error
}

// BoardStore is this module's port onto the board contract: the read the
// GET /board endpoint translates. Consumer-defined like TodoStore; the
// composition root injects the concrete store.
type BoardStore interface {
	List() ([]board.ColumnCards, error)
}

// NewHandler builds the handler for the JSON contract: GET /board over the
// board store, POST/PATCH/DELETE /todos over the todo store (GET /todos is
// retired — the board read replaces the todo list surface). The composition
// root mounts it on its listener.
func NewHandler(todoStore TodoStore, boardStore BoardStore) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /board", handleBoard(boardStore))
	mux.HandleFunc("POST /todos", handleCreate(todoStore))
	mux.HandleFunc("PATCH /todos/{id}", handleChange(todoStore))
	mux.HandleFunc("DELETE /todos/{id}", handleDelete(todoStore))
	return mux
}
