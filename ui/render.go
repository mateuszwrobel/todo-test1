package ui

import (
	"bytes"
	"html/template"
	"net/http"
	"strings"
)

// doneColumnTitle is the display title of the column whose cards render with
// the done treatment. The treatment is a rendering rule keyed on the column
// a card sits in (workplan_ui_board.md decision): the contract carries no
// done field — column membership is the done state — so a per-card done
// indicator would contradict the model, and no done checkbox or toggle
// exists anywhere on the page.
const doneColumnTitle = "Done"

// Page surfaces rendered by this module: the shell (the create band joined it
// with card ui/04, KW2 — an htmx form beside the board area, swapped through
// POST /ui/cards), the board (three column panels in the contract's fixed
// order, cards top-to-bottom in array order), and the stated load-failure
// state. Template-per-surface; procedural.
var pageTmpl = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Board</title>
{{/* Load order is contractual: tokens.css before style.css — style.css
    consumes the design tokens via var(), so the token layer must parse
    first (a later token sheet would flash unstyled pixels). */}}
<link rel="stylesheet" href="/static/tokens.css">
<link rel="stylesheet" href="/static/style.css">
<script src="/static/htmx.min.js" defer></script>
</head>
<body>
<main>
<h1 class="heading">Board</h1>
<div id="board-area">{{.State}}</div>
{{.CreateArea}}
</main>
<script>
// htmx swaps 2xx responses by default; a stated failure arrives as a 4xx
// whose body is already-rendered HTML for the same surface the operation
// acts on. Route those bodies through the same htmx swap engine — no
// browser-side rendering happens here. Carried over from the retired todo
// shell and scoped to the create control; the edit control's refusals join
// this routing (422 states the reason at the re-rendered card, 404 states
// the missing card above the truth without it — both bodies are the board
// area). Delete and drag refusals join at their own cards (KW4, KW5).
document.body.addEventListener('htmx:responseError', function (event) {
  var elt = event.detail && event.detail.elt;
  var text = event.detail.xhr.responseText;
  if (elt && elt.closest && elt.closest('#create-form')) {
    // A create refusal: the already-rendered create-area fragment.
    htmx.swap(document.getElementById('create-area'), text,
              { swapStyle: 'outerHTML' });
  } else if (elt && elt.closest && elt.closest('.edit-form')) {
    // An edit refusal: the already-rendered board area — the editing card
    // with its original text and the stated reason, or the missing-card
    // banner over the truth.
    htmx.swap(document.getElementById('board-area'), text,
              { swapStyle: 'innerHTML' });
  }
});
// The inline edit band: a card's Edit control reveals the band — prefilled
// with the card's title — without any request; Cancel hides it again, so a
// discarded edit sends nothing by construction. Save submits the band's
// form through htmx (PATCH /ui/cards/{id} → the swap paths above).
document.body.addEventListener('click', function (event) {
  var control = event.target.closest
    ? event.target.closest('.card__edit, .edit-form .cancel')
    : null;
  if (!control) return;
  var band = control.closest('.card');
  if (control.classList.contains('cancel')) {
    band.classList.remove('editing');
    return;
  }
  band.classList.add('editing');
  var input = band.querySelector('.edit-form .input');
  if (input) input.focus();
});
</script>
</body>
</html>
`))

// boardTmpl renders the three column panels in the contract's array order
// (the contract fixes the order To Do / In Progress / Done, and the page
// renders the server's truth rather than re-imposing its own). Each card
// carries its edit and delete affordance hooks: the edit band is live since
// KW3 (card ui/06) — the Edit control reveals an inline form prefilled with
// the card's title, and Save submits PATCH /ui/cards/{id} through htmx, on
// every card in every column including Done (J6: done is just a column, a
// done card's text is editable). The delete button stays an inert
// placeholder — no wiring until KW4 (card ui/07).
var boardTmpl = template.Must(template.New("board").Parse(`<div id="board" class="board">
{{- range .}}
<section id="column-{{.Anchor}}" class="column" data-column="{{.Title}}">
<h2 class="column__title">{{.Title}}</h2>
{{if .Cards}}<ul class="column__cards">{{- range .Cards}}
<li id="card-{{.ID}}" class="card{{if .Done}} card--done{{end}}" data-card="{{.ID}}">
<span class="card__title">{{.Title}}</span>
<form class="edit-form" hx-patch="/ui/cards/{{.ID}}" hx-target="#board-area" hx-swap="innerHTML">
<input class="input" type="text" name="title" value="{{.Title}}">
<button type="submit" class="btn btn--primary save">Save</button>
<button type="button" class="btn btn--secondary cancel">Cancel</button>
</form>
<button type="button" class="btn btn--secondary card__edit">Edit</button>
<button type="button" class="btn btn--secondary card__delete">Delete</button>{{if .EditError}}
<p id="edit-error-{{.ID}}" class="error-text">{{.EditError}}</p>{{end}}
</li>
{{- end}}
</ul>{{else}}<p class="column__empty" data-empty="true">No cards</p>{{end}}
</section>
{{- end}}
</div>`))

// failedTmpl is the stated load-failure surface. The retry control re-issues
// the read through GET semantics — recovery stays inside the page reload
// path, same pattern the superseded list surface used.
var failedTmpl = template.Must(template.New("failed").Parse(
	`<div id="load-error" class="panel">Could not load board. <a id="retry" href="/">Retry</a></div>`))

// column is one rendered column panel; card is one rendered card. Done on a
// card is derived, never carried: it is set by the mapping below when the
// card's column is the done column (see doneColumnTitle).
type column struct {
	Title  string // display title as the contract states it
	Anchor string // id-safe form of Title, for the column element's id
	Cards  []card
}

type card struct {
	ID        int64
	Title     string
	Done      bool   // derived from column membership — drives the card--done class
	EditError string // stated refusal rendering at this card, empty when nothing was rejected
}

// columnsOf maps the contract body to the render model: columns in array
// order, cards in array (position) order, the done treatment keyed on the
// column's display title.
func columnsOf(board boardResponse) []column {
	columns := make([]column, 0, len(board.Columns))
	for _, col := range board.Columns {
		out := column{Title: col.Title, Anchor: columnAnchor(col.Title)}
		for _, c := range col.Cards {
			out.Cards = append(out.Cards, card{
				ID:    c.ID,
				Title: c.Title,
				Done:  col.Title == doneColumnTitle,
			})
		}
		columns = append(columns, out)
	}
	return columns
}

// attachEditError puts the contract's stated refusal on one card of the
// render model — the card the user was editing, wherever it sits. The card
// keeps the server's original title (the refusal never applied), and the
// reason renders at the card.
func attachEditError(columns []column, id int64, reason string) {
	for i := range columns {
		for j := range columns[i].Cards {
			if columns[i].Cards[j].ID == id {
				columns[i].Cards[j].EditError = reason
				return
			}
		}
	}
}

// columnAnchor makes an element id out of a display title ("In Progress" →
// "in-progress").
func columnAnchor(title string) string {
	return strings.ToLower(strings.ReplaceAll(title, " ", "-"))
}

func renderBoard(w http.ResponseWriter, columns []column) {
	var b bytes.Buffer
	if err := boardTmpl.Execute(&b, columns); err != nil {
		http.Error(w, "render failed", http.StatusInternalServerError)
		return
	}
	renderPage(w, template.HTML(b.String()))
}

func renderPage(w http.ResponseWriter, state template.HTML) {
	_ = pageTmpl.Execute(w, struct {
		State      template.HTML
		CreateArea template.HTML
	}{state, createAreaHTML(createAreaData{})})
}

func renderState(w http.ResponseWriter, which *template.Template) {
	var b bytes.Buffer
	if err := which.Execute(&b, nil); err != nil {
		http.Error(w, "render failed", http.StatusInternalServerError)
		return
	}
	renderPage(w, template.HTML(b.String()))
}
