package api

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// handleDelete maps DELETE /todos/{id} to the store's Delete operation:
// success is 204 with no body (contract's fixed delete mapping). A
// non-numeric id is unparseable input → 400 "invalid request" (contract
// blanket rule); the not-found outcome maps with card api/11.
func handleDelete(store TodoStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
			return
		}
		if err := store.Delete(id); err != nil {
			http.Error(w, "failed to delete todo", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
