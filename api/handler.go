// Package api exposes the todo application's JSON wire contract and
// translates HTTP requests into todos-module operations. It owns no listener
// and never renders HTML.
package api

import (
	"encoding/json"
	"net/http"

	"todo/todos"
)

// TodoStore is the minimal port this module declares for itself (consumer-
// defined port): the concrete store is injected by the composition root, so
// no module depends on another's concrete type.
type TodoStore interface {
	List() ([]todos.Todo, error)
	Change(id int64, fields todos.ChangeFields) (todos.Todo, error)
	Delete(id int64) error
}

// NewHandler builds the handler for the /todos subtree. The composition root
// mounts it on its listener.
func NewHandler(store TodoStore) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /todos", handleList(store))
	mux.HandleFunc("PATCH /todos/{id}", handleChange(store))
	mux.HandleFunc("DELETE /todos/{id}", handleDelete(store))
	return mux
}

// handleList maps GET /todos to the store's List operation: 200 with the JSON
// array of todos ordered by identifier ascending (List's order).
func handleList(store TodoStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := store.List()
		if err != nil {
			http.Error(w, "failed to read todos", http.StatusInternalServerError)
			return
		}
		if list == nil {
			list = []todos.Todo{} // the contract's empty list is [], never null
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(list); err != nil {
			return // status already sent; nothing left to state
		}
	}
}
