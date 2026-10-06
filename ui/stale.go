package ui

import (
	"bytes"
	"html/template"
	"net/http"
)

// The stale-operation failure surface (card ui/11): ONE named surface
// serving edit, drag-move, and delete identically. Wherever a card
// operation crosses the contract and comes back 404 — the page showed a
// card the server no longer holds — the answer here is the only form the
// failure takes: the contract's own reason ("no such card", owned upstream
// in api/board, carried verbatim, never re-worded) rendered in the design
// system's banner above the board re-read from server truth. The stale
// card is simply absent from that truth, so nothing is ever left looking
// edited, moved, or deleted — the failure needs no faking, and the page
// states plainly that the card does not exist.
//
// One surface in both senses of the word: the same fragment for every verb
// (edit.go, delete.go and move.go each call writeStaleFailure from their
// 404 arm — no second mechanism exists for this failure class), swapped
// into the same region by the client (the shell's htmx responseError arms
// and the drag fetch all replace #board-area with this body, render.go),
// at the same mirrored 404 status. A reload reads the same truth, so the
// statement never outlives the reload (J8: reload resolves staleness).
//
// The per-operation VALIDATION refusal (422) is a distinct failure class
// with a distinct surface — the stated reason at the offending card over
// the unchanged truth (attachEditError) — deliberately not merged here:
// this surface speaks only to operations on a card that no longer exists.

// missingCardTmpl states the stale failure in the design system's banner
// element — the named element (#missing-card) every verb's 404 rides.
var missingCardTmpl = template.Must(template.New("missing-card").Parse(
	`<div id="missing-card" class="banner" role="alert">{{.}}</div>`))

// writeStaleFailure is the single owner of the stale 404 answer, called by
// each verb's 404 arm: banner stating the contract's reason, then the
// truth re-rendered without the card, answered at the contract's mirrored
// status. The truth re-read failing surfaces through writeBoardAreaFragment
// as the stated load-failure panel — that class's own surface, not a
// second stale mechanism.
func (p *page) writeStaleFailure(w http.ResponseWriter, reason string) {
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
}
