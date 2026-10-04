package ui

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io"
	"net/http"
)

// The create surface: an input and an Add button posting through htmx so the
// list swap lands in place without a page reload. Template-per-surface, in
// the create feature's own file. The stated-refusal path lands with its own
// card; until then a non-created answer is a plainly failed create.

var createAreaTmpl = template.Must(template.New("create-area").Parse(
	`<div id="create-area"{{if .OOB}} hx-swap-oob="outerHTML"{{end}}>
<form id="create-form" hx-post="/ui/todos" hx-target="#todos-area" hx-swap="innerHTML">
<input type="text" name="title" value="{{.Value}}" placeholder="What needs doing?">
<button type="submit">Add</button>
</form>
</div>`))

type createAreaData struct {
	Value string // text in the input
	OOB   bool   // answer as an out-of-band swap (success reset)
}

func createAreaHTML(data createAreaData) template.HTML {
	var b bytes.Buffer
	if err := createAreaTmpl.Execute(&b, data); err != nil {
		return "" // static template; nothing here can fail
	}
	return template.HTML(b.String())
}

// handleCreate — POST /ui/todos. It performs the create by calling the api
// contract over HTTP (never in-process), then answers the resulting state as
// a swap fragment: on success the fresh list state plus a reset create area,
// so the new row appears last without a page reload and the control is
// ready for the next todo.
func (p *page) handleCreate(w http.ResponseWriter, r *http.Request) {
	title := r.FormValue("title")
	if p.createTodo(title) != http.StatusCreated {
		http.Error(w, "failed to create todo", http.StatusInternalServerError)
		return
	}
	var b bytes.Buffer
	if err := p.writeTodosAreaFragment(&b); err != nil {
		http.Error(w, "render failed", http.StatusInternalServerError)
		return
	}
	if err := writeFragment(&b, createAreaTmpl, createAreaData{OOB: true}); err != nil {
		http.Error(w, "render failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(b.Bytes())
}

// createTodo performs the module's create write: POST {apiBase}/todos over
// real HTTP. It returns the contract's status.
func (p *page) createTodo(title string) int {
	body, err := json.Marshal(map[string]string{"title": title})
	if err != nil {
		return http.StatusInternalServerError
	}
	resp, err := p.client.Post(p.apiBase+"/todos", "application/json", bytes.NewReader(body))
	if err != nil {
		return http.StatusBadGateway
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

// writeTodosAreaFragment writes the current list state as the swap content
// for #todos-area: rows, the stated empty state, or the stated load-failure
// surface — never a blank or stale stand-in.
func (p *page) writeTodosAreaFragment(w io.Writer) error {
	items, err := p.listTodos()
	if err != nil {
		return writeFragment(w, failedTmpl, nil)
	}
	if len(items) == 0 {
		return writeFragment(w, emptyTmpl, nil)
	}
	return writeFragment(w, listTmpl, items)
}

func writeFragment(w io.Writer, tmpl *template.Template, data any) error {
	var b bytes.Buffer
	if err := tmpl.Execute(&b, data); err != nil {
		return err
	}
	_, err := w.Write(b.Bytes())
	return err
}
