package ui

import (
	"bytes"
	"html/template"
	"net/http"
)

// Page surfaces rendered by this module: the shell (with the always-ready
// create control), the list (rows carry their done state and controls), the
// stated empty state, and the stated load-failure state. Template-per-
// surface; procedural.
var pageTmpl = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Todo</title>
<script src="/static/htmx.min.js" defer></script>
</head>
<body>
<main>
<h1>Todos</h1>
<style>
/* The row edit band stays hidden until the row's Edit control puts the
   row in edit mode — the list reads as text, not as inputs. */
.edit-form{display:none}
li.editing .edit-form{display:inline}
li.editing .title{display:none}
</style>
<div id="todos-area">{{.State}}</div>
{{.CreateArea}}
</main>
<script>
// htmx swaps 2xx responses by default; a stated failure arrives as a 4xx
// whose body is already-rendered HTML for the same surface the operation
// acts on. Route those bodies through the same htmx swap engine — no
// browser-side rendering happens here.
document.body.addEventListener('htmx:responseError', function (event) {
  var elt = event.detail && event.detail.elt;
  if (elt && elt.closest && elt.closest('#create-form')) {
    // A create refusal: the already-rendered create-area fragment.
    htmx.swap(document.getElementById('create-area'),
              event.detail.xhr.responseText,
              { swapStyle: 'outerHTML' });
    return;
  }
  // A row-operation failure: the stated surface plus the server-truth list
  // (an edit refusal carries the reason on the row, a missing todo carries
  // the banner above the truthful list). Plain (non-HTML) error bodies
  // never swap, so nothing on the page can look like it succeeded.
  var area = document.getElementById('todos-area');
  var ctype = event.detail.xhr.getResponseHeader('Content-Type') || '';
  if (area && ctype.indexOf('text/html') === 0) {
    htmx.swap(area, event.detail.xhr.responseText, { swapStyle: 'innerHTML' });
  }
});
</script>
</body>
</html>
`))

var listTmpl = template.Must(template.New("list").Parse(`<ul id="todo-list">
{{- range .}}
<li id="todo-{{.ID}}"{{if .Editing}} class="editing"{{end}} data-state="{{if .Done}}done{{else}}not-done{{end}}">
<label class="done-toggle"><input type="checkbox" {{if .Done}}checked{{end}} hx-patch="/ui/todos/{{.ID}}" hx-vals='{"done": {{if .Done}}false{{else}}true{{end}}}' hx-target="#todos-area" hx-swap="innerHTML" hx-disabled-elt="this">
<span>{{if .Done}}done{{else}}not done{{end}}</span></label>
<span class="title">{{.Title}}</span>
{{if not .Done}}<button type="button" class="edit" hx-on:click="this.closest('li').classList.toggle('editing')">Edit</button>
<form class="edit-form" hx-patch="/ui/todos/{{.ID}}" hx-target="#todos-area" hx-swap="innerHTML" hx-disabled-elt="#todo-{{.ID}} .save">
<input type="text" name="title" value="{{if .Editing}}{{.Typed}}{{else}}{{.Title}}{{end}}">
<button type="submit" class="save">Save</button>
<button type="button" class="cancel" hx-on:click="this.closest('li').classList.remove('editing')">Cancel</button>
</form>{{end}}{{if .EditError}}<p id="edit-error-{{.ID}}" class="edit-error">{{.EditError}}</p>{{end}}
<button type="button" class="delete" hx-delete="/ui/todos/{{.ID}}" hx-target="#todos-area" hx-swap="innerHTML" hx-disabled-elt="this">Delete</button>
</li>
{{- end}}
</ul>`))

var emptyTmpl = template.Must(template.New("empty").Parse(
	`<p id="empty-state">No todos yet.</p>`))

// The failure state offers a retry: a GET on the page, which re-issues the
// read — recovery stays inside GET semantics.
var failedTmpl = template.Must(template.New("failed").Parse(
	`<div id="load-error">Could not load todos. <a id="retry" href="/">Retry</a></div>`))

func renderPage(w http.ResponseWriter, state template.HTML) {
	_ = pageTmpl.Execute(w, struct {
		State      template.HTML
		CreateArea template.HTML
	}{state, createAreaHTML(createAreaData{})})
}

func renderList(w http.ResponseWriter, todos []todo) {
	var b bytes.Buffer
	if err := listTmpl.Execute(&b, rowsFor(todos)); err != nil {
		http.Error(w, "render failed", http.StatusInternalServerError)
		return
	}
	renderPage(w, template.HTML(b.String()))
}

// listRow is one rendered row: the todo itself plus, on the edit feature's
// refusal path only, the stated contract refusal and the rejected text kept
// in the row's band (ui/11, ui/12). The plain browse/toggle/delete renderings
// leave the three edit fields empty — rows render as before.
type listRow struct {
	todo
	EditError string // contract's stated refusal, shown on this row when set
	Typed     string // rejected edit text, prefilled back into the band
	Editing   bool   // band revealed on load — a refusal on a not-done row
}

// rowsFor maps a plain list state to rows carrying no edit surface.
func rowsFor(todos []todo) []listRow {
	rows := make([]listRow, 0, len(todos))
	for _, t := range todos {
		rows = append(rows, listRow{todo: t})
	}
	return rows
}

func renderState(w http.ResponseWriter, which *template.Template) {
	var b bytes.Buffer
	if err := which.Execute(&b, nil); err != nil {
		http.Error(w, "render failed", http.StatusInternalServerError)
		return
	}
	renderPage(w, template.HTML(b.String()))
}
