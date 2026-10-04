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
<div id="todos-area">{{.State}}</div>
{{.CreateArea}}
</main>
<script>
// htmx swaps 2xx responses by default; a create refusal arrives as a 4xx
// whose body is an already-rendered create-area fragment. Route it through
// the same htmx swap engine — no browser-side rendering happens here.
document.body.addEventListener('htmx:responseError', function (event) {
  var elt = event.detail && event.detail.elt;
  if (elt && elt.closest && elt.closest('#create-form')) {
    htmx.swap(document.getElementById('create-area'),
              event.detail.xhr.responseText,
              { swapStyle: 'outerHTML' });
  }
});
</script>
</body>
</html>
`))

var listTmpl = template.Must(template.New("list").Parse(`<ul id="todo-list">
{{- range .}}
<li id="todo-{{.ID}}" data-state="{{if .Done}}done{{else}}not-done{{end}}">
<label class="done-toggle"><input type="checkbox" {{if .Done}}checked{{end}} hx-patch="/ui/todos/{{.ID}}" hx-vals='{"done": {{if .Done}}false{{else}}true{{end}}}' hx-target="#todo-list" hx-swap="outerHTML">
<span>{{if .Done}}done{{else}}not done{{end}}</span></label>
<span class="title">{{.Title}}</span>
{{if not .Done}}<button type="button" class="edit">Edit</button>{{end}}
<button type="button" class="delete" hx-delete="/ui/todos/{{.ID}}" hx-target="#todo-list" hx-swap="outerHTML">Delete</button>
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
	if err := listTmpl.Execute(&b, todos); err != nil {
		http.Error(w, "render failed", http.StatusInternalServerError)
		return
	}
	renderPage(w, template.HTML(b.String()))
}

func renderState(w http.ResponseWriter, which *template.Template) {
	var b bytes.Buffer
	if err := which.Execute(&b, nil); err != nil {
		http.Error(w, "render failed", http.StatusInternalServerError)
		return
	}
	renderPage(w, template.HTML(b.String()))
}
