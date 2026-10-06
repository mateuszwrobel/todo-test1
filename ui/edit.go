package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strconv"
)

// The edit surface (card ui/06, KW3): every card in every column — Done
// included, no frozen-done (J6: done is just a column, and editing touches
// neither position nor identity) — carries an inline edit band, and its Save
// posts here as PATCH /ui/cards/{id}. The title change is performed by
// calling the api contract over HTTP (never in-process), and the answer is
// the resulting state as a swap fragment — the create endpoint's pattern
// (create.go) on the card surface: mutations re-render from server truth,
// not local guesses, so the edited card shows the new title at the same
// position in the same column wherever the server keeps it. The column
// field is deliberately absent: moving is the drag card's operation (KW5).

// missingCardTmpl states the stale-edit failure above the board: the
// contract's own reason ("no such card") in the design system's banner
// class, riding with the truth re-render so the stale card is simply gone
// and nothing is left looking edited. The full stale surface shared by
// every card operation lands with card ui/11 (KW5); this is the edit's
// stated 404 arm.
var missingCardTmpl = template.Must(template.New("missing-card").Parse(
	`<div id="missing-card" class="banner" role="alert">{{.}}</div>`))

// handleEdit — PATCH /ui/cards/{id}. It performs the title change by
// issuing PATCH {apiBase}/cards/{id} over real HTTP with the contract's
// {"title": ...} body, then answers the resulting state as a swap fragment
// targeting #board-area: on success the fresh board (200) — the re-read is
// the in-place guarantee, the card lands where the server holds it with the
// new title; on a contract refusal (422) the board re-rendered from server
// truth with the stated reason at the editing card, the card's original
// title intact and the contract's status mirrored; on a stale card (404)
// the missing-card banner over the truth without it.
func (p *page) handleEdit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid card id", http.StatusBadRequest)
		return
	}
	title := r.FormValue("title")
	status, reason := p.patchCardTitle(r, id, title)

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
			err = writeFragment(&b, boardTmpl, columns)
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
		// session): the failure is stated, the truth re-renders without
		// it, and nothing is left looking edited. Reloading reads the
		// same truth, so the statement never outlives the reload.
		var b bytes.Buffer
		if err := writeFragment(&b, missingCardTmpl, reason); err != nil {
			http.Error(w, "render failed", http.StatusInternalServerError)
			return
		}
		if err := p.writeBoardAreaFragment(&b); err != nil {
			http.Error(w, "render failed", http.StatusInternalServerError)
			return
		}
		writeHTML(w, http.StatusNotFound, b)
	default:
		http.Error(w, reason, status)
	}
}

// patchCardTitle performs the module's edit write: PATCH {apiBase}/cards/{id}
// over real HTTP with the contract's {"title": ...} body — never the column
// field, the edit has no say over placement. It returns the contract's
// status and, on a refusal, its stated reason; the message's single owner
// stays upstream (api/board), this module only carries it. The fallbacks
// cover only a body that states nothing: a refusal from the real contract
// always carries its own wording.
func (p *page) patchCardTitle(r *http.Request, id int64, title string) (status int, reason string) {
	body, err := json.Marshal(map[string]string{"title": title})
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
