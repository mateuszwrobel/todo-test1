package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// The edit surface (card ui/06, KW3): every card outside the Done column
// carries an inline edit band, and its Save posts here as PATCH
// /ui/cards/{id}. Since the done freeze of 2026-10-07 (parent scenario 15
// — the frozen-text rule the 2026-10-06 pivot had deleted, restored) a card
// sitting in Done renders no edit affordance at all; this endpoint stays
// the seam the contract's refusal shows through — a forced title PATCH on a
// Done card answers 422 with the contract's stated reason rendered at the
// card, the same attachEditError path as the blank/over-limit refusals.
// Moving out of Done is the only unlock (editing elsewhere touches neither
// position nor identity). The title change is performed by calling the api
// contract over HTTP (never in-process), and the answer is the resulting
// state as a swap fragment — the create endpoint's pattern (create.go) on
// the card surface: mutations re-render from server truth, not local
// guesses, so the edited card shows the new title at the same position in
// the same column wherever the server keeps it. The column field is
// deliberately absent: moving is the drag card's operation (KW5).

// handleEdit — PATCH /ui/cards/{id}. It performs the change by issuing
// PATCH {apiBase}/cards/{id} over real HTTP with the contract's body —
// {"title": ...} plus, since KW9 (card ui/14), the band's "assignee" field
// (name, or null for the select's Unassigned) — then answers the resulting
// state as a swap fragment targeting #board-area: on success the fresh
// board (200) — the re-read is the in-place guarantee, the card lands where
// the server holds it with the new title and assignee; on a contract
// refusal (422 — blank, over-limit, an edit on a card sitting in Done, or
// an assignee the roster does not know) the board re-rendered from server
// truth with the stated reason at the editing card, the card's original
// fields intact and the contract's status mirrored; on a stale card (404)
// the shared stale-failure surface (stale.go) — identical for every verb.
func (p *page) handleEdit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid card id", http.StatusBadRequest)
		return
	}
	title := r.FormValue("title")
	status, reason := p.patchCard(r, id, title)

	switch status {
	case http.StatusOK:
		var b bytes.Buffer
		if err := p.writeBoardAreaFragment(&b); err != nil {
			http.Error(w, "render failed", http.StatusInternalServerError)
			return
		}
		writeHTML(w, http.StatusOK, b)
	case http.StatusUnprocessableEntity:
		// The refusal is stated AT the editing card over the server's
		// unchanged truth: the card keeps its original text (this module
		// never re-words the contract's message), and the board around it
		// is whatever a fresh read says.
		var b bytes.Buffer
		if board, err := p.loadBoard(); err == nil {
			columns := columnsOf(board)
			attachEditError(columns, id, reason)
			err = writeFragment(&b, boardTmpl, p.viewFor(columns, ""))
		} else {
			err = writeFragment(&b, failedTmpl, nil)
		}
		if err != nil {
			http.Error(w, "render failed", http.StatusInternalServerError)
			return
		}
		writeHTML(w, http.StatusUnprocessableEntity, b)
	case http.StatusNotFound:
		// The card vanished behind the page's back (deleted in another
		// session): the shared stale-failure surface — the same banner over
		// the same truth every verb serves (stale.go).
		p.writeStaleFailure(w, reason)
	default:
		http.Error(w, reason, status)
	}
}

// patchCard performs the module's edit write: PATCH {apiBase}/cards/{id}
// over real HTTP with the contract's body — never the column field, the
// edit has no say over placement. It returns the contract's
// status and, on a refusal, its stated reason; the message's single owner
// stays upstream (api/board), this module only carries it. The fallbacks
// cover only a body that states nothing: a refusal from the real contract
// always carries its own wording.
//
// The assignee field carries the band select's three states through to the
// contract's three spellings (api/change.go): the submitted form has no
// assignee field (a pre-KW9 band, or a forced title-only seam request) →
// the field stays ABSENT from the body, no direction, and the request
// bytes match the pre-KW9 edit exactly; an empty value (the select's
// Unassigned) → JSON null, the contract's clear; a name → JSON string, the
// contract's assign. Selections ride the one PATCH the save already makes
// — htmx serializes the whole band form — so a save is one change request
// whether or not the assignee moved. Roster validity is never screened
// here: a name off the cast comes back as the contract's own 422 and
// renders at the card through the refusal arm below.
func (p *page) patchCard(r *http.Request, id int64, title string) (status int, reason string) {
	payload := map[string]any{"title": title}
	if r.Form.Has("assignee") {
		if name := r.FormValue("assignee"); name != "" {
			payload["assignee"] = name
		} else {
			payload["assignee"] = nil
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return http.StatusInternalServerError, "could not change card"
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPatch,
		fmt.Sprintf("%s/cards/%d", p.apiBase, id), bytes.NewReader(body))
	if err != nil {
		return http.StatusInternalServerError, "could not change card"
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return http.StatusBadGateway, "could not reach the board contract"
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body) // drain for connection reuse
		return http.StatusOK, ""
	}
	buf, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	var parsed map[string]string
	if json.Unmarshal(buf, &parsed) == nil && parsed["error"] != "" {
		return resp.StatusCode, parsed["error"]
	}
	if resp.StatusCode == http.StatusNotFound {
		return resp.StatusCode, "no such card"
	}
	return resp.StatusCode, "could not change card"
}

// writeHTML answers a fragment endpoint with an already-rendered body at
// the mirrored contract status.
func writeHTML(w http.ResponseWriter, status int, body bytes.Buffer) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body.Bytes())
}
