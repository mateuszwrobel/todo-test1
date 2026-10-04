package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"todo/todos"
)

// handleCreate implements POST /todos: parse `{ "title": string }`, call the
// store's Create, answer 201 with the created todo JSON. Malformed JSON is a
// transport error: 400 `{"error": "invalid request"}` (distinct from the 422
// rule refusals). This card's stage maps every store failure to 500; the
// invalid-text outcomes gain their 422 mapping as their cards land.
//
// The error shape is the contract's one shape, `{ "error": message }`. It is
// answered through a local responder rather than a package-level helper so
// this handler stays self-contained in its own file.
func handleCreate(store TodoStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		respondError := func(status int, message string) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
		}

		var req struct {
			Title string `json:"title"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(http.StatusBadRequest, "invalid request")
			return
		}
		created, err := store.Create(req.Title)
		if err != nil {
			// Outcome → transport mapping is this module's decision (api
			// workplan): the store owns the rule, this switch states it.
			switch {
			case errors.Is(err, todos.ErrTitleRequired):
				respondError(http.StatusUnprocessableEntity, "title is required")
			default:
				http.Error(w, "failed to create todo", http.StatusInternalServerError)
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(created); err != nil {
			return // status already sent; nothing left to state
		}
	}
}
