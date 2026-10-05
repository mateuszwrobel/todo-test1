// Package ui renders the todo page and its HTML surfaces. It has zero
// in-process dependencies on the other modules: it reaches the api contract
// over HTTP at runtime, at the base URL the composition root injects.
package ui

import (
	"embed"
	"net/http"
)

//go:embed static
var staticFS embed.FS

// todo mirrors the api contract's Todo DTO. The ui module defines its own
// shape because it may not import the other modules (architecture spec).
type todo struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

type page struct {
	apiBase string
	client  *http.Client
}

// NewHandler builds the ui module's handlers. apiBase is the address of the
// api contract on the same listener (injected by the composition root); every
// read of todos goes to that address over HTTP.
func NewHandler(apiBase string) http.Handler {
	p := &page{
		apiBase: apiBase,
		client:  &http.Client{Timeout: defaultLoadTimeout},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", p.handleIndex)
	mux.HandleFunc("GET /static/htmx.min.js", p.handleHTMX)
	// tokens.css before style.css: style.css is pure var() consumption,
	// so the token sheet must be parsed first. The <link> order in the
	// page shells (render.go, stories.go) enforces that at load time;
	// the routes themselves are order-independent.
	mux.HandleFunc("GET /static/tokens.css", serveStaticCSS("static/tokens.css"))
	mux.HandleFunc("GET /static/style.css", serveStaticCSS("static/style.css"))
	mux.HandleFunc("GET /__components", p.handleStories)
	mux.HandleFunc("POST /ui/todos", p.handleCreate)
	mux.HandleFunc("PATCH /ui/todos/{id}", p.handleToggle)
	mux.HandleFunc("DELETE /ui/todos/{id}", p.handleDelete)
	return mux
}

func (p *page) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	todos, err := p.listTodos()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err != nil {
		renderState(w, failedTmpl)
		return
	}
	if len(todos) == 0 {
		renderState(w, emptyTmpl)
		return
	}
	renderList(w, todos)
}

func (p *page) handleHTMX(w http.ResponseWriter, r *http.Request) {
	asset, err := staticFS.ReadFile("static/htmx.min.js")
	if err != nil {
		http.Error(w, "static asset unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/javascript")
	_, _ = w.Write(asset)
}

// serveStaticCSS serves one stylesheet from the embedded asset tree that
// also carries htmx and the page markup. tokens.css and style.css share
// this path: the design system is two sheets, the token layer and its
// consumer, served identically.
func serveStaticCSS(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		asset, err := staticFS.ReadFile(name)
		if err != nil {
			http.Error(w, "static asset unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/css")
		_, _ = w.Write(asset)
	}
}
