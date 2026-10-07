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
	// Assignee is the contract's name-or-null (KW9): JSON null decodes to
	// the empty string, which the render model reads as unassigned — the
	// same sentinel board uses, and the roster contract excludes an empty
	// name, so no real assignee can ever look like none.
	Assignee string `json:"assignee"`
}

// Roster is the ui module's consumer port for the simulation's cast (KW9):
// the fixed list of names the assignee select offers. ui never imports the
// users package — the architecture spec forbids the ui→users edge (names
// reach the page over the contract only, workplan_users_roster.md
// Boundaries) — so the module declares what it needs and the composition
// root wires the provider, exactly as it injects the api base URL the board
// reads flow through. The order the port answers in is contract: the select
// lists the cast in one fixed order (users/01 pins it at the source).
type Roster interface {
	Names() []string
}

// RosterFunc adapts a bare function (users.Names at the composition root)
// to the Roster port, so wiring stays one expression with no adapter type.
type RosterFunc func() []string

func (f RosterFunc) Names() []string { return f() }

type page struct {
	apiBase string
	client  *http.Client
	roster  Roster
}

// rosterNames answers the port defensively: no provider wired means no
// names, which the select renders as the Unassigned option alone — a page
// never invents roster names of its own.
func (p *page) rosterNames() []string {
	if p.roster == nil {
		return nil
	}
	return p.roster.Names()
}

// NewHandler builds the ui module's handlers. apiBase is the address of the
// api contract on the same listener (injected by the composition root); every
// read of the board goes to that address over HTTP, and so does every write
// the page's fragment endpoints perform. roster is the cast provider, also
// injected by the composition root (the Roster port above); the page lists
// those names in the edit band's assignee select and nowhere else — the
// card's chip shows only the name the contract carries on the card itself.
// The todo list fragment endpoints
// retired at KW1; the create endpoint (POST /ui/cards) re-extended the route
// table at KW2 and the edit endpoint (PATCH /ui/cards/{id}) at KW3; the
// delete endpoint (DELETE /ui/cards/{id}) re-extends it at KW4 and the drag
// endpoint at KW5 (see
// workplans/dependencies_kanban.md).
func NewHandler(apiBase string, roster Roster) http.Handler {
	p := &page{
		apiBase: apiBase,
		client:  &http.Client{Timeout: defaultLoadTimeout},
		roster:  roster,
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
	// Drag re-extends it at KW5 (cards ui/08–10): a dropped card's single
	// per-drop request patches here and this module performs
	// PATCH /cards/{id} over HTTP with the contract's column+position body.
	mux.HandleFunc("PATCH /ui/cards/{id}/move", p.handleMove)
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

// handleIndex renders the whole page from the URL — the page keeps no
// filter state of its own. The ?assignee= parameter is handled as the api
// seam handles it (mirror, not import): PRESENCE of the parameter decides,
// the value travels verbatim to the contract. An absent parameter is the
// shipped unfiltered read, byte-for-byte; a present one renders the
// narrowed board (card ui/16: reload replays exactly what the address
// says). A filtered read that fails — the contract's 422 unknown user
// among the failures — states the failure the one stated way a failed
// read is stated (failedTmpl, the card ui/03 surface), never an empty
// board standing in for the truth.
func (p *page) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	values, filtered := r.URL.Query()["assignee"]
	keyword := ""
	if filtered {
		keyword = values[0]
	}
	board, err := p.loadBoardFilter(keyword, filtered)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err != nil {
		// A failed read states the failure — never columns standing in
		// for a truth the server could not give (card ui/03 pins it). The
		// chrome still names the URL's filter, so the dropdown reflects
		// the address the failure came from.
		renderState(w, failedTmpl, p.chromeFor(keyword))
		return
	}
	p.renderBoard(w, columnsOf(board), keyword)
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
