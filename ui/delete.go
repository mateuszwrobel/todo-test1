package ui

import (
	"net/http"
)

// handleDelete is the row-delete fragment endpoint named in the ui workplan
// decisions (DELETE /ui/todos/{id}): it performs the operation against the
// api HTTP contract (never in-process), then renders the resulting list
// state as the fragment htmx swaps into the page — the deleted row is gone,
// every other row keeps its text, done state, and order, and the removal of
// the last row lands the stated empty state. A missing todo arrives as the
// contract's 404 and is answered with the stated missing-todo surface
// (ui/14): the banner plus the current truth, not a silent relay.
func (p *page) handleDelete(w http.ResponseWriter, r *http.Request) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodDelete,
		p.apiBase+"/todos/"+r.PathValue("id"), nil)
	if err != nil {
		http.Error(w, "failed to build delete request", http.StatusInternalServerError)
		return
	}
	resp, err := p.client.Do(req)
	if err != nil {
		http.Error(w, "delete failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		p.renderTodosFragment(w)
	case http.StatusNotFound:
		// The todo was already gone before this delete reached the
		// contract — state the missing-todo failure (ui/14) rather than
		// relaying a transport error that leaves the page silent.
		p.writeMissingTodo(w, statedMissingReason(resp.Body))
	default:
		http.Error(w, "delete failed", http.StatusBadGateway)
	}
}

// renderTodosFragment renders the resulting list state as a swap fragment:
// the same list/empty/failure state selection as the full page, without the
// page shell, so htmx can swap it into the open page.
func (p *page) renderTodosFragment(w http.ResponseWriter) {
	todos, err := p.listTodos()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	switch {
	case err != nil:
		_ = failedTmpl.Execute(w, nil)
	case len(todos) == 0:
		_ = emptyTmpl.Execute(w, nil)
	default:
		_ = listTmpl.Execute(w, rowsFor(todos))
	}
}
