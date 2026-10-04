package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"todo/todos"
)

// changeRequest mirrors the PATCH contract body: both fields optional, at
// least one required. Pointers distinguish "absent" from present-with-zero
// value, which is what the at-least-one-field rule needs.
type changeRequest struct {
	Title *string `json:"title"`
	Done  *bool   `json:"done"`
}

// handleChange maps PATCH /todos/{id}: parse, call the store's Change,
// translate the typed outcome into transport (mapping fixed by the api
// workplan). W3 wires the done direction; remaining outcome mappings arrive
// with their cards.
func handleChange(store TodoStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			errorJSON(w, http.StatusBadRequest, "invalid request")
			return
		}
		// An absent (empty) body is simply "no fields supplied" — the store's
		// invalid-no-fields outcome states the rule; a present body must
		// parse, or it is a transport error (400, per the workplan).
		body, err := io.ReadAll(r.Body)
		if err != nil {
			errorJSON(w, http.StatusBadRequest, "invalid request")
			return
		}
		var req changeRequest
		if strings.TrimSpace(string(body)) != "" {
			if err := json.Unmarshal(body, &req); err != nil {
				errorJSON(w, http.StatusBadRequest, "invalid request")
				return
			}
		}
		fields := todos.ChangeFields{Title: req.Title, Done: req.Done}
		updated, err := store.Change(id, fields)
		if err != nil {
			// Outcome mapping fixed by the api workplan: not-found → 404,
			// invalid-no-fields → 422 stating the rule, invalid-text → 422
			// with the store's rule stated (title required / limit number
			// owned by todos.MaxTitleLength).
			if errors.Is(err, todos.ErrNotFound) {
				errorJSON(w, http.StatusNotFound, "no such todo")
				return
			}
			if errors.Is(err, todos.ErrDoneFrozen) {
				// The store's frozen-text rule stated on the contract
				// (api/07): a done todo's title is not editable.
				errorJSON(w, http.StatusUnprocessableEntity, "cannot edit a done todo")
				return
			}
			if errors.Is(err, todos.ErrTitleRequired) {
				errorJSON(w, http.StatusUnprocessableEntity, "title is required")
				return
			}
			if errors.Is(err, todos.ErrTitleTooLong) {
				// The limit number has one owner — the todos constant — so
				// this message can never drift from the rule.
				errorJSON(w, http.StatusUnprocessableEntity,
					fmt.Sprintf("title exceeds the %d-character limit", todos.MaxTitleLength))
				return
			}
			if errors.Is(err, todos.ErrNoFields) {
				errorJSON(w, http.StatusUnprocessableEntity, "at least one field is required")
				return
			}
			http.Error(w, "failed to change todo", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(updated); err != nil {
			return // status already sent; nothing left to state
		}
	}
}

// errorJSON writes the contract's one error shape: { "error": message }.
func errorJSON(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
