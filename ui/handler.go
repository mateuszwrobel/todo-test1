// Package ui renders the kanban board page and its HTML surfaces. It has
// zero in-process dependencies on the other modules: it reaches the api
// contract over HTTP at runtime, at the base URL the composition root
// injects. The page renders the board (GET /board); the superseded todo
// list-render stack retired here (workplan_ui_board.md decision: the ui
// module survives with its surface replaced, ADR-003).
package ui

import (
	"embed"
	"net/http"
)

//go:embed static
var staticFS embed.FS

// boardResponse mirrors the GET /board contract body. The ui module defines
// its own shapes because it may not import the other modules (architecture
// spec). Cards arrive inside their column arrays in position order — the
// render mirrors array order and never re-sorts (server order is the truth).
type boardResponse struct {
	Columns []boardColumnResponse `json:"columns"`
}

type boardColumnResponse struct {
	Title string              `json:"title"`
	Cards []boardCardResponse `json:"cards"`
}

type boardCardResponse struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Column   string `json:"column"`
	Position int    `json:"position"`
}

type page struct {
	apiBase string
	client  *http.Client
}

// NewHandler builds the ui module's handlers. apiBase is the address of the
// api contract on the same listener (injected by the composition root); every
// read of the board goes to that address over HTTP, and so does every write
// the page's fragment endpoints perform. The todo list fragment endpoints
// retired at KW1; the create endpoint (POST /ui/cards) re-extended the route
// table at KW2 and the edit endpoint (PATCH /ui/cards/{id}) at KW3; the
// delete endpoint (DELETE /ui/cards/{id}) re-extends it at KW4 and the drag
// endpoint at KW5 (see
// workplans/dependencies_kanban.md).
func NewHandler(apiBase string) http.Handler {
	p := &page{
		apiBase: apiBase,
		client:  &http.Client{Timeout: defaultLoadTimeout},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", p.handleIndex)
	// Create re-extends the fragment table at KW2 (card ui/04): the page's
	// create band posts here, this module performs POST /cards over HTTP,
	// and the answer is a swap fragment — the retired todo fragment
	// endpoints' pattern on the board's surface. Edit re-extends it at KW3
	// (card ui/06): a card's edit band patches here and this module
	// performs PATCH /cards/{id} over HTTP. Delete re-extends it at KW4
	// (card ui/07): a card's Delete control deletes here and this module
	// performs DELETE /cards/{id} over HTTP. Drag re-extends it at KW5.
	mux.HandleFunc("POST /ui/cards", p.handleCreate)
	mux.HandleFunc("PATCH /ui/cards/{id}", p.handleEdit)
	mux.HandleFunc("DELETE /ui/cards/{id}", p.handleDelete)
	mux.HandleFunc("GET /static/htmx.min.js", p.handleHTMX)
	// tokens.css before style.css: style.css is pure var() consumption,
	// so the token sheet must be parsed first. The <link> order in the
	// page shells (render.go, stories.go) enforces that at load time;
	// the routes themselves are order-independent.
	mux.HandleFunc("GET /static/tokens.css", serveStaticCSS("static/tokens.css"))
	mux.HandleFunc("GET /static/style.css", serveStaticCSS("static/style.css"))
	mux.HandleFunc("GET /__components", p.handleStories)
	return mux
}

func (p *page) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	board, err := p.loadBoard()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err != nil {
		// A failed read states the failure — never columns standing in
		// for a truth the server could not give (card ui/03 pins it).
		renderState(w, failedTmpl)
		return
	}
	renderBoard(w, columnsOf(board))
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
