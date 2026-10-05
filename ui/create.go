package ui

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io"
	"net/http"
)

// The create surface: an input and an Add button posting through htmx so the
// list swap lands in place without a page reload, plus the stated error the
// create area shows when the contract refuses. Template-per-surface, in the
// create feature's own file.

var createAreaTmpl = template.Must(template.New("create-area").Parse(
	`<div id="create-area"{{if .OOB}} hx-swap-oob="outerHTML"{{end}}>
<form id="create-form" hx-post="/ui/todos" hx-target="#todos-area" hx-swap="innerHTML" hx-disabled-elt="#create-form button[type=submit]">
<input class="input" type="text" name="title" value="{{.Value}}" placeholder="What needs doing?">
<button type="submit" class="btn btn--primary">Add</button>
{{/* Hint wording mirrors the api contract's title limit (owned upstream by
    todos.MaxTitleLength = 500; ui may not import todos, so this static
    line matches the contract instead of deriving from the constant). */}}
<p class="hint">Up to 500 characters</p>{{if .Error}}
<p id="create-error" class="error-text">{{.Error}}</p>{{end}}
</form>
</div>`))

type createAreaData struct {
	Value string // text in the input (typed text survives a rejection)
	Error string // stated refusal, empty when nothing was rejected
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
// a swap fragment: on success the fresh list state plus a reset create area
// (201); on a contract refusal the contract's stated reason in the create
// area with the typed text preserved (the contract's status is mirrored so
// the page's error path stays true to the contract).
func (p *page) handleCreate(w http.ResponseWriter, r *http.Request) {
	title := r.FormValue("title")
	status, reason := p.createTodo(title)

	if status == http.StatusCreated {
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
		return
	}

	var b bytes.Buffer
	if err := writeFragment(&b, createAreaTmpl, createAreaData{Value: title, Error: reason}); err != nil {
		http.Error(w, "render failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(b.Bytes())
}

// createTodo performs the module's create write: POST {apiBase}/todos over
// real HTTP. It returns the contract's status and, on a refusal, the
// contract's stated reason — the message's single owner stays the api.
func (p *page) createTodo(title string) (status int, reason string) {
	body, err := json.Marshal(map[string]string{"title": title})
	if err != nil {
		return http.StatusInternalServerError, "could not create todo"
	}
	resp, err := p.client.Post(p.apiBase+"/todos", "application/json", bytes.NewReader(body))
	if err != nil {
		return http.StatusBadGateway, "could not reach the todo contract"
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusCreated {
		return http.StatusCreated, ""
	}
	return resp.StatusCode, statedReason(resp.Body)
}

// statedReason extracts the contract's error message (`{"error": message}`),
// falling back to a plainly stated failure if the body says nothing.
func statedReason(body io.Reader) string {
	buf, err := io.ReadAll(io.LimitReader(body, 4<<10))
	if err != nil {
		return "could not create todo"
	}
	var parsed map[string]string
	if err := json.Unmarshal(buf, &parsed); err == nil && parsed["error"] != "" {
		return parsed["error"]
	}
	return "could not create todo"
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
	return writeFragment(w, listTmpl, rowsFor(items))
}

func writeFragment(w io.Writer, tmpl *template.Template, data any) error {
	var b bytes.Buffer
	if err := tmpl.Execute(&b, data); err != nil {
		return err
	}
	_, err := w.Write(b.Bytes())
	return err
}
