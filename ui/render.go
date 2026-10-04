package ui

import (
	"bytes"
	"html/template"
	"net/http"
)

// Page surfaces rendered by this module: the list, the stated empty state,
// and the stated load-failure state. Template-per-surface; procedural.
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
{{.}}
</main>
</body>
</html>
`))

var listTmpl = template.Must(template.New("list").Parse(`<ul id="todo-list">
{{- range .}}
<li id="todo-{{.ID}}">{{.Title}}</li>
{{- end}}
</ul>`))

var emptyTmpl = template.Must(template.New("empty").Parse(
	`<p id="empty-state">No todos yet.</p>`))

var failedTmpl = template.Must(template.New("failed").Parse(
	`<div id="load-error">Could not load todos.</div>`))

func renderPage(w http.ResponseWriter, surface template.HTML) {
	_ = pageTmpl.Execute(w, surface)
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
