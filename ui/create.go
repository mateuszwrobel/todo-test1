package ui

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io"
	"net/http"
)

// The create surface (card ui/04, KW2): an input and an Add button posting
// through htmx so the board swap lands in place without a page reload, plus
// the stated error the create area shows when the contract refuses (card
// ui/05). Template-per-surface, in the create feature's own file — the
// structure mirrors the retired todo create surface (git 1ff09e5^:ui/create.go,
// retired at KW1) with the swap target retargeted from the list area to the
// board area.

var createAreaTmpl = template.Must(template.New("create-area").Parse(
	`<div id="create-area"{{if .OOB}} hx-swap-oob="outerHTML"{{end}}>
<form id="create-form" hx-post="/ui/cards" hx-target="#board-area" hx-swap="innerHTML" hx-disabled-elt="#create-form button[type=submit]">
<input class="input" type="text" name="title" value="{{.Value}}" placeholder="What needs doing?">
<button type="submit" class="btn btn--primary">Add</button>
{{/* Hint wording mirrors the api contract's title limit (owned upstream by
    board.MaxTextLen = 500; ui may not import board, so this static line
    matches the contract instead of deriving from the constant). */}}
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

// handleCreate — POST /ui/cards. It performs the create by calling the api
// contract over HTTP (never in-process), then answers the resulting state as
// a swap fragment: on success the fresh board state — a re-read, not a local
// guess, so the new card lands wherever the server put it (bottom of To Do)
// — plus a reset create area (201); on a contract refusal the stated reason
// in the create area with the typed text preserved (the contract's status is
// mirrored so the page's error path stays true to the contract, and the
// shell's htmx:responseError routing swaps the refusal fragment in place).
func (p *page) handleCreate(w http.ResponseWriter, r *http.Request) {
	title := r.FormValue("title")
	status, reason := p.createCard(title)

	if status == http.StatusCreated {
		var b bytes.Buffer
		if err := p.writeBoardAreaFragment(&b); err != nil {
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

// createCard performs the module's create write: POST {apiBase}/cards over
// real HTTP with the contract's {"title": ...} body. It returns the
// contract's status and, on a refusal, its stated reason — the message's
// single owner stays upstream (api/board), this module only carries it.
func (p *page) createCard(title string) (status int, reason string) {
	body, err := json.Marshal(map[string]string{"title": title})
	if err != nil {
		return http.StatusInternalServerError, "could not create card"
	}
	resp, err := p.client.Post(p.apiBase+"/cards", "application/json", bytes.NewReader(body))
	if err != nil {
		return http.StatusBadGateway, "could not reach the board contract"
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusCreated {
		return http.StatusCreated, ""
	}
	return resp.StatusCode, "could not create card"
}

// writeBoardAreaFragment writes the current board state as the swap content
// for #board-area: the three columns from a fresh read, or the stated
// load-failure surface — never a blank or stale stand-in. The re-read is
// deliberate: order and column membership are server truth (workplan
// re-render decision), and a fresh GET /board is exactly that truth.
func (p *page) writeBoardAreaFragment(w io.Writer) error {
	board, err := p.loadBoard()
	if err != nil {
		return writeFragment(w, failedTmpl, nil)
	}
	return writeFragment(w, boardTmpl, columnsOf(board))
}

func writeFragment(w io.Writer, tmpl *template.Template, data any) error {
	var b bytes.Buffer
	if err := tmpl.Execute(&b, data); err != nil {
		return err
	}
	_, err := w.Write(b.Bytes())
	return err
}
