package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// handleToggle is the htmx target of each row's done checkbox:
// PATCH /ui/todos/{id} carrying the new done value (hx-vals sends it for
// both directions of the checkbox). It performs the operation THROUGH the
// api contract — a JSON PATCH over real HTTP — then renders the current
// list state as the swap fragment, so the row shows the server's truth
// without a full page reload.
func (p *page) handleToggle(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid todo id", http.StatusBadRequest)
		return
	}
	// The endpoint serves both row operations (ui workplan endpoint table):
	// a form carrying a title is the inline edit's save — that direction
	// owns its own file (edit.go).
	if err := r.ParseForm(); err == nil && r.Form.Has("title") {
		p.handleEditSave(w, r, id)
		return
	}
	done := r.FormValue("done")
	if done != "true" && done != "false" {
		// Defensive path mirroring the contract's at-least-one-field rule.
		http.Error(w, "done=true or done=false is required", http.StatusUnprocessableEntity)
		return
	}
	status, reason, err := p.patchDone(id, done == "true")
	if err != nil {
		http.Error(w, "api unreachable", http.StatusBadGateway)
		return
	}
	if status != http.StatusOK {
		if status == http.StatusNotFound {
			// The page is stale about this todo — state the missing-todo
			// failure (banner + truth re-render, ui/14). A silent failure
			// would leave the clicked checkbox flipped as if it worked.
			p.writeMissingTodo(w, reason)
			return
		}
		// The contract's failure is stated with the contract's own status:
		// htmx does not swap error responses, so no row is left looking like
		// the operation succeeded.
		http.Error(w, fmt.Sprintf("toggle refused (%d)", status), status)
		return
	}
	list, err := p.listTodos()
	if err != nil {
		http.Error(w, "could not read todos", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = listTmpl.Execute(w, rowsFor(list))
}

// patchDone performs the done-state change on the api contract:
// PATCH {apiBase}/todos/{id} with {"done": bool}. It returns the contract's
// status and, on a failure, the contract's stated reason; ui never
// re-implements the rule, the contract owns it.
func (p *page) patchDone(id int64, done bool) (int, string, error) {
	body, err := json.Marshal(map[string]bool{"done": done})
	if err != nil {
		return 0, "", err
	}
	req, err := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/todos/%d", p.apiBase, id), bytes.NewReader(body))
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
	return resp.StatusCode, statedMissingReason(resp.Body), nil
}
