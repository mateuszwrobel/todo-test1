package ui

import (
	"encoding/json"
	"html/template"
	"io"
	"net/http"
)

// The missing-todo surface (ui/14, card "Missing todo states the failure for
// any operation"): ONE stated banner serving toggle, edit-save, and delete.
// The workplan fixes the choice — per-row error surfaces attach to the
// triggering area (the row), but a missing todo has no row to attach to: the
// todo is gone from the server, and a truth-synced re-render drops its row.
// So the missing case is stated as the banner-style notice the workplan's
// error-surface decision names for stale cases (mockup 06), above the list,
// identical for all three operations. The per-row edit-error mechanism stays
// what it was: row-attached contract refusals (422) on rows that exist.

// bannerTmpl is the missing-todo notice, rendered once per failed operation.
// Its text is the contract's own stated reason — the message's single owner
// stays the api ("no such todo" for a 404).
var bannerTmpl = template.Must(template.New("banner").Parse(
	`<div id="missing-todo-banner" class="missing-todo" role="alert">{{.}}</div>`))

// writeMissingTodo answers a fragment endpoint with the stated missing-todo
// failure: the contract's 404 status, the banner, and the current server
// list state under it. The truth re-render is what keeps the failure honest
// — the stale row disappears, a checkbox that visually flipped on click
// flips back, and no row is left looking like the failed operation
// succeeded. Reloading the page reads the same truth from the server, so
// the failure statement never outlives the reload.
func (p *page) writeMissingTodo(w http.ResponseWriter, reason string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	if reason == "" {
		reason = "no such todo"
	}
	_ = writeFragment(w, bannerTmpl, reason)
	_ = p.writeTodosAreaFragment(w)
}

// statedMissingReason extracts the contract's error message
// (`{"error": message}`), falling back to a plainly stated missing note if
// the body says nothing. The api owns the wording; the fallback only covers
// a body that never states one.
func statedMissingReason(body io.Reader) string {
	buf, err := io.ReadAll(io.LimitReader(body, 4<<10))
	if err != nil {
		return "no such todo"
	}
	var parsed map[string]string
	if err := json.Unmarshal(buf, &parsed); err == nil && parsed["error"] != "" {
		return parsed["error"]
	}
	return "no such todo"
}
