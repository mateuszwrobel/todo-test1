package api

import (
	"errors"
	"net/http"
	"strconv"

	"todo/todos"
)

// handleDelete maps DELETE /todos/{id} to the store's Delete operation:
// success is 204 with no body (contract's fixed delete mapping), the store's
// not-found outcome is 404 "no such todo" (contract outcome mapping lives
// here, in this module alone). A non-numeric id is unparseable input → 400
// "invalid request" (contract blanket rule).
func handleDelete(store TodoStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			errorJSON(w, http.StatusBadRequest, "invalid request")
			return
		}
		if err := store.Delete(id); err != nil {
			if errors.Is(err, todos.ErrNotFound) {
				errorJSON(w, http.StatusNotFound, "no such todo")
				return
			}
			http.Error(w, "failed to delete todo", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
