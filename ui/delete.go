package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// The delete surface (card ui/07, KW4): every card in every column — Done
// included, no frozen-done (J6: done is just a column, and a done card is as
// deletable as any other) — carries a live Delete control whose click issues
// DELETE {apiBase}/cards/{id} over real HTTP (never in-process), and the
// answer is the resulting state as a swap fragment — the edit endpoint's
// pattern (edit.go) on the removal surface: the board re-renders from server
// truth, so the card is simply gone and its column's remaining cards pack up
// with no gap because the fresh read lists the column contiguously (board/11).
// No confirmation dialog: the card text states activation, not confirmation,
// and neither the workplan scenario nor the journeys add one.

// handleDelete — DELETE /ui/cards/{id}. It performs the removal by issuing
// DELETE {apiBase}/cards/{id} over real HTTP, then answers the resulting
// state as a swap fragment targeting #board-area: on the contract's 204 the
// fresh board (rendered at 200 — htmx treats a 204 as "nothing to swap", and
// the card's contract is the board re-rendered without it, which is exactly
// the contract's own observable truth: the next GET /board), and on a stale
// card (404) the shared stale-failure surface (stale.go) — byte-for-byte the
// same surface the edit and move verbs serve, one mechanism per failure
// class across verbs rather than a second one.
func (p *page) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid card id", http.StatusBadRequest)
		return
	}
	status, reason := p.deleteCard(r, id)

	switch status {
	case http.StatusNoContent:
		// The re-read is the drops-one-card guarantee: the removed card is
		// absent because the server no longer lists it, the column closes
		// its gap because the fresh arrays are contiguous, and every other
		// card renders from the same truth untouched.
		var b bytes.Buffer
		if err := p.writeBoardAreaFragment(&b); err != nil {
			http.Error(w, "render failed", http.StatusInternalServerError)
			return
		}
		writeHTML(w, http.StatusOK, b)
	case http.StatusNotFound:
		// Already gone behind the page's back (deleted in another session):
		// the shared stale-failure surface — the same banner over the same
		// truth every verb serves (stale.go). Nothing is left looking
		// deleted by a click that did nothing, and reloading reads the same
		// truth, so the statement never outlives the reload.
		p.writeStaleFailure(w, reason)
	default:
		http.Error(w, reason, status)
	}
}

// deleteCard performs the module's delete write: DELETE {apiBase}/cards/{id}
// over real HTTP, no body — the contract's delete carries no payload. It
// returns the contract's status and, on a refusal, its stated reason; the
// message's single owner stays upstream (api/board), this module only carries
// it. The fallbacks cover only a body that states nothing: a refusal from the
// real contract always carries its own wording.
func (p *page) deleteCard(r *http.Request, id int64) (status int, reason string) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodDelete,
		fmt.Sprintf("%s/cards/%d", p.apiBase, id), nil)
	if err != nil {
		return http.StatusInternalServerError, "could not delete card"
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return http.StatusBadGateway, "could not reach the board contract"
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		_, _ = io.Copy(io.Discard, resp.Body) // drain for connection reuse
		return http.StatusNoContent, ""
	}
	buf, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	var parsed map[string]string
	if json.Unmarshal(buf, &parsed) == nil && parsed["error"] != "" {
		return resp.StatusCode, parsed["error"]
	}
	if resp.StatusCode == http.StatusNotFound {
		return resp.StatusCode, "no such card"
	}
	return resp.StatusCode, "could not delete card"
}
