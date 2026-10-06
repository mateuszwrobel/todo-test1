package api

import (
	"encoding/json"
	"net/http"
)

// handleCreate implements POST /cards: parse `{ "title": string }`, call the
// board store's Create, answer 201 with the created Card JSON — the board
// module's own JSON tags are the contract's four fields (id, title, column,
// position), so the created card encodes straight through. The board trims
// the title before storing, so the 201 states the created card as stored:
// trimmed. Malformed JSON is a transport error: 400
// `{"error": "invalid request"}` (the module's standing convention, kept
// from the retired todo create handler; distinct from the 422 rule refusals).
// This card's stage maps every store failure to 500; the invalid-text
// outcomes gain their stated 422 mapping as their cards land (api/03,
// api/04).
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
			// workplan): the board owns the rule, this site states the
			// mapping. The 422 arms land with api/03 and api/04; until each
			// lands, its outcome answers the store-failure status.
			http.Error(w, "failed to create card", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(created); err != nil {
			return // status already sent; nothing left to state
		}
	}
}
