package api

import (
	"errors"
	"net/http"
	"strconv"

	"todo/todos"
)

// handleCardDelete maps DELETE /cards/{id} to the board store's Delete:
// success is 204 with no body — the contract's fixed delete mapping — and
// the deletion's observable truth is the next GET /board, which omits the
// card and lists its column contiguously (board/11 closed the gap in the
// same transaction that removed the row). A non-numeric id is unparseable
// input → 400 "invalid request", the module's standing transport convention
// kept from create and change. Outcome → transport mapping lives here, at
// one site like change.go's; the not-found arm joins it with api/11.
func handleCardDelete(store BoardStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			errorJSON(w, http.StatusBadRequest, "invalid request")
			return
		}
		if err := store.Delete(id); err != nil {
			http.Error(w, "failed to delete card", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

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
