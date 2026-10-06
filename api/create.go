package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"todo/board"
)

// handleCreate implements POST /cards: shape-only request validation (is the
// body well-formed JSON carrying a string title?), one call to the board
// store's Create, 201 with the created Card JSON on success — the board
// module's own JSON tags are the contract's four fields (id, title, column,
// position), so the created card encodes straight through. The board trims
// the title before storing, so the 201 states the created card as stored:
// trimmed.
//
// Outcome → transport mapping lives at one site below, keyed on the board's
// typed outcomes via errors.Is (api workplan decision): blank → 422 "title
// is required" (api/03); the over-limit outcome gains its stated message
// with api/04, and until it lands answers the store-failure 500. No title
// rule is restated here. Malformed JSON is a transport error: 400
// `{"error": "invalid request"}`
// (the module's standing convention, kept from the retired todo create
// handler; distinct from the 422 rule refusals).
//
// The error shape is the contract's one shape, `{ "error": message }`. It is
// answered through a local responder rather than a package-level helper so
// this handler stays self-contained in its own file.
func handleCreate(store BoardStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		respondError := func(status int, message string) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
		}

		// An empty body is io.EOF — it carries no title, so it belongs to
		// the required path below, not the transport error. Anything else
		// the decoder rejects (broken syntax, truncation) is malformed
		// JSON: 400 invalid request.
		var body any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			respondError(http.StatusBadRequest, "invalid request")
			return
		}
		title := ""
		if obj, ok := body.(map[string]any); ok {
			switch t := obj["title"].(type) {
			case string:
				title = t
			case nil: // title absent or JSON null — no title to hand over
			default: // present but not a string — not the contract's body shape
				respondError(http.StatusBadRequest, "invalid request")
				return
			}
		}
		// Any other body — no body at all, or well-formed JSON that is not
		// an object — carries no title either. The contract states the same
		// refusal for an absent title as for an empty one, so the empty
		// string goes to the store and board's required outcome states it
		// once; the rule's one owner stays board.
		created, err := store.Create(title)
		if err != nil {
			// Outcome → transport mapping is this module's decision (api
			// workplan): one site, keyed on the board's typed outcomes.
			switch {
			case errors.Is(err, board.ErrTextRequired):
				respondError(http.StatusUnprocessableEntity, "title is required")
			default:
				http.Error(w, "failed to create card", http.StatusInternalServerError)
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
