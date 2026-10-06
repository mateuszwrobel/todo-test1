package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"todo/board"
)

// handleCardChange implements PATCH /cards/{id}: parse the body — a JSON
// object carrying "title" and/or "column" — hand presence-pointers to the
// board store's Change (a nil pointer means "leave untouched", exactly the
// contract's "at least one field" distinction), and map outcomes to transport
// at one site keyed on the board's typed errors with errors.Is (the api
// workplan's mapping-table decision). Success is 200 with the updated Card
// encoded straight from board's JSON tags — the contract's four fields
// (id, title, column, position).
//
// The title direction reaches board's text rule — the same single rule
// source create uses — so its refusals are worded identically across verbs:
// ErrTextRequired → 422 "title is required", ErrTextTooLong → 422 stating
// the limit with the number formatted from board.MaxTextLen, so PATCH and
// POST cannot word the same error class differently. Remaining outcome
// mappings arrive with their cards.
//
// Request validation is shape-only (the api workplan's decision): malformed
// JSON, a known field present with a non-string type, and a non-numeric id
// are transport errors — 400 `{"error": "invalid request"}` — the module's
// standing convention, kept from create. Semantic rules (blank text, the
// character limit, the column enum) stay owned by the board module and are
// reached through its typed outcomes, never restated here.
//
// Body-shape decisions mirror create: an empty body (io.EOF), a well-formed
// JSON body that is not an object, an absent key, and a key whose value is
// JSON null all mean "field absent"; the pointers stay nil. A column value
// is handed over as-is — the enum guard is board's, so a bad name arrives
// as board's invalid-column outcome rather than a second copy here.
func handleCardChange(store BoardStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			errorJSON(w, http.StatusBadRequest, "invalid request")
			return
		}

		// An empty body is io.EOF — it carries no fields, so it belongs to
		// the shape path below, not the transport error. Anything else the
		// decoder rejects (broken syntax, truncation) is malformed JSON:
		// 400 invalid request (create's standing split).
		var body any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			errorJSON(w, http.StatusBadRequest, "invalid request")
			return
		}

		var title *string
		var column *board.Column
		if obj, ok := body.(map[string]any); ok {
			if v, present := obj["title"]; present && v != nil {
				text, ok := v.(string)
				if !ok { // present but not a string — not the contract's body shape
					errorJSON(w, http.StatusBadRequest, "invalid request")
					return
				}
				title = &text
			}
			if v, present := obj["column"]; present && v != nil {
				name, ok := v.(string)
				if !ok { // same transport class as a non-string title
					errorJSON(w, http.StatusBadRequest, "invalid request")
					return
				}
				col := board.Column(name)
				column = &col
			}
		}

		updated, err := store.Change(id, title, column)
		if err != nil {
			// Outcome → transport mapping — one site, keyed on the board's
			// typed outcomes (api workplan decision).
			switch {
			case errors.Is(err, board.ErrCardNotFound):
				// Card api/07 — the contract's stated not-found body. The
				// board's existence check is its transaction's first read,
				// so the rejected patch left the board exactly as it was.
				errorJSON(w, http.StatusNotFound, "no such card")
			case errors.Is(err, board.ErrTextRequired):
				errorJSON(w, http.StatusUnprocessableEntity, "title is required")
			case errors.Is(err, board.ErrTextTooLong):
				// The limit number has one owner — the board constant — and
				// this is create's exact wording, so the class reads the
				// same from either verb.
				errorJSON(w, http.StatusUnprocessableEntity,
					fmt.Sprintf("title exceeds the %d character limit", board.MaxTextLen))
			default:
				http.Error(w, "failed to change card", http.StatusInternalServerError)
			}
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
