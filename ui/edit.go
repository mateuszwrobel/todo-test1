package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// The inline edit band (ui/09): each not-done row carries a hidden form —
// an input prefilled with the row's text plus Save and Cancel — revealed
// by the row's Edit control. Save PATCHes the shared fragment endpoint;
// Cancel is client-side only (it triggers no request, so the original text
// stays untouched by construction). This file owns the endpoint's title
// direction: the operation travels THROUGH the api contract over real
// HTTP — never in-process — and the contract owns every rule, including
// the frozen-done refusal and the title-required refusal.

// handleEditSave is the edit-band save path of PATCH /ui/todos/{id} (the
// same endpoint the done checkbox uses; the form's title key routes here).
// On success the answer is the fresh list fragment (200): the row shows
// the new text without a reload, the todo's done state and position are
// whatever the server says — the edit never rewrites them. On a contract
// refusal (422) the list is re-rendered from server truth with the
// contract's stated reason on the affected row; the rejected text survives
// in a still-not-done row's band for the correcting submit. A missing todo
// is answered with the stated missing-todo surface shared by all row
// operations (ui/14, stale.go).
func (p *page) handleEditSave(w http.ResponseWriter, r *http.Request, id int64) {
	title := r.Form.Get("title")
	status, reason, err := p.patchTitle(r, id, title)
	if err != nil {
		http.Error(w, "api unreachable", http.StatusBadGateway)
		return
	}
	switch status {
	case http.StatusOK:
		list, err := p.listTodos()
		if err != nil {
			http.Error(w, "could not read todos", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = listTmpl.Execute(w, rowsFor(list))
	case http.StatusUnprocessableEntity:
		list, err := p.listTodos()
		if err != nil {
			http.Error(w, "could not read todos", http.StatusBadGateway)
			return
		}
		rows := rowsFor(list)
		for i := range rows {
			if rows[i].ID == id {
				rows[i].EditError = reason
				rows[i].Typed = title
				// A not-done row keeps its band open around the rejected
				// text; a done row has no band at all — the refusal shows
				// beside the row that now truthfully reads done.
				rows[i].Editing = !rows[i].Done
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = listTmpl.Execute(w, rows)
	case http.StatusNotFound:
		// The todo vanished behind the page's back — the missing-todo
		// failure is stated for edit exactly as for toggle and delete
		// (ui/14): banner + truth re-render, no row left faking the edit.
		p.writeMissingTodo(w, reason)
	default:
		http.Error(w, "edit failed", http.StatusBadGateway)
	}
}

// patchTitle performs the title change on the api contract: PATCH
// {apiBase}/todos/{id} with {"title": string} — never done, the edit has
// no say over the done state. It returns the contract's status and, on a
// refusal, the contract's stated reason; the message's single owner stays
// the api.
func (p *page) patchTitle(r *http.Request, id int64, title string) (int, string, error) {
	body, err := json.Marshal(map[string]string{"title": title})
	if err != nil {
		return 0, "", err
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPatch,
		fmt.Sprintf("%s/todos/%d", p.apiBase, id), bytes.NewReader(body))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body) // drain for connection reuse
		return http.StatusOK, "", nil
	}
	return resp.StatusCode, statedEditReason(resp.Body), nil
}

// statedEditReason extracts the contract's error message
// (`{"error": message}`), falling back to a plainly stated failure if the
// body says nothing.
func statedEditReason(body io.Reader) string {
	buf, err := io.ReadAll(io.LimitReader(body, 4<<10))
	if err != nil {
		return "edit refused"
	}
	var parsed map[string]string
	if err := json.Unmarshal(buf, &parsed); err == nil && parsed["error"] != "" {
		return parsed["error"]
	}
	return "edit refused"
}
