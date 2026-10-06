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
// shell and scoped to the create control; the edit and delete controls'
// refusals route here too: a 422 states the reason at the re-rendered
// card, a 404 is the shared stale-failure surface (stale.go) — banner over
// the truth, swapped into #board-area the same way for every verb. The
// drag's answers arrive through the fetch below and take the very same
// fragments into the very same region — one surface per failure class
// across all three verbs.
document.body.addEventListener('htmx:responseError', function (event) {
  var elt = event.detail && event.detail.elt;
  var text = event.detail.xhr.responseText;
  if (elt && elt.closest && elt.closest('#create-form')) {
    // A create refusal: the already-rendered create-area fragment.
    htmx.swap(document.getElementById('create-area'), text,
              { swapStyle: 'outerHTML' });
  } else if (elt && elt.closest &&
             (elt.closest('.edit-form') || elt.closest('.card__delete'))) {
    // An edit or delete refusal: the already-rendered board area — the
    // editing card with its original text and the stated reason, the
    // missing-card banner over the truth, or (delete) the same truth after
    // the card dropped out.
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
// Drag-to-move (cards ui/08–10, KW5): HTML5 DnD, no libraries, independent
// of htmx. Every card in every column — Done included, done is just a
// column — is draggable by the card itself. During the drag the source
// renders as a dashed slot and a blue insertion line marks the landing gap
// BEFORE release, so the indicator always reflects the index the request
// will carry (the workplan's edge-of-column risk mitigation; mockups k3/k4
// for the slot, the line, and the title-only drag chip). An accepted drop
// issues EXACTLY ONE PATCH — the single fetch site below is the page's only
// per-drop request — carrying the target column and the drop position, and
// the answer (server truth, stated failures included) replaces the board
// area: swap, never reload, and never a local guess. An abandoned drag —
// released outside any column (nothing there accepts the drop, so it never
// fires), or cancelled (ESC cancels natively, surfacing as dragend without
// a drop) — never reaches the fetch: the card takes its slot back and the
// board is untouched (card ui/10). A drop that lands the card where it
// already sits is NOT abandonment by the card's definition (abandonment is
// dropping outside any valid target), so it rides the one-request rule and
// the re-normalized truth comes back unchanged — the client computes the
// payload, it does not second-guess accepted drops.
var draggedCard = null;   // the <li> mid-drag; null when no drag is active
var dropIndicator = null; // the insertion line, parked at the landing gap

function ensureDropIndicator() {
  if (!dropIndicator) {
    dropIndicator = document.createElement('li');
    dropIndicator.className = 'drop-indicator';
  }
  return dropIndicator;
}

// dropPosition is the whole payload math, isolated as pure DOM arithmetic:
// the number of rendered cards ahead of the insertion line in the target
// column, NOT counting the dragged card's own slot — which is exactly the
// contract's index-after-removal (board.Move removes before inserting,
// same-column included), read off the list the user is looking at. The
// indicator and the request therefore cannot disagree: both are this count.
function dropPosition(column, dragged) {
  var scope = column.querySelector('.column__cards') || column;
  var position = 0;
  for (var node = scope.firstElementChild; node && node !== dropIndicator; node = node.nextElementSibling) {
    if (node.classList.contains('card') && node !== dragged) position++;
  }
  return position;
}

function endDragVisuals() {
  if (draggedCard) draggedCard.classList.remove('card--source');
  if (dropIndicator && dropIndicator.parentNode) dropIndicator.parentNode.removeChild(dropIndicator);
  var marked = document.querySelectorAll('.column--drop-target');
  for (var i = 0; i < marked.length; i++) marked[i].classList.remove('column--drop-target');
  var chips = document.querySelectorAll('.drag-chip');
  for (var c = 0; c < chips.length; c++) chips[c].parentNode.removeChild(chips[c]);
  draggedCard = null;
}

document.body.addEventListener('dragstart', function (event) {
  var card = event.target.closest ? event.target.closest('.card') : null;
  if (!card) return;
  draggedCard = card;
  event.dataTransfer.effectAllowed = 'move';
  event.dataTransfer.setData('text/plain', card.dataset.card);
  card.classList.add('card--source');
  // The drag chip carries the title alone — no action labels (mockups k3/k4).
  var chip = document.createElement('div');
  chip.className = 'drag-chip';
  var title = card.querySelector('.card__title');
  chip.textContent = title ? title.textContent : '';
  document.body.appendChild(chip);
  event.dataTransfer.setDragImage(chip, 12, 12);
});

document.body.addEventListener('dragover', function (event) {
  if (!draggedCard) return;
  var column = event.target.closest ? event.target.closest('.column') : null;
  // Outside a column nothing is accepted: no preventDefault, so the browser
  // shows no drop cursor and drop never fires there — an abandoned drag
  // sends nothing because the fetch site is unreachable from it (ui/10).
  if (!column) return;
  event.preventDefault();
  event.dataTransfer.dropEffect = 'move';
  var columns = document.querySelectorAll('.column');
  for (var i = 0; i < columns.length; i++) {
    columns[i].classList.toggle('column--drop-target', columns[i] === column);
  }
  var scope = column.querySelector('.column__cards') || column;
  var line = ensureDropIndicator();
  var before = null;
  var cards = scope.querySelectorAll('.card');
  for (var j = 0; j < cards.length; j++) {
    var box = cards[j].getBoundingClientRect();
    if (event.clientY < box.top + box.height / 2) { before = cards[j]; break; }
  }
  if (line.parentNode) line.parentNode.removeChild(line);
  if (before) scope.insertBefore(line, before); else scope.appendChild(line);
});

document.body.addEventListener('drop', function (event) {
  if (!draggedCard) return;
  var column = event.target.closest ? event.target.closest('.column') : null;
  if (!column) return; // abandoned outside any column — zero requests
  event.preventDefault();
  var line = ensureDropIndicator();
  var scope = column.querySelector('.column__cards') || column;
  if (line.parentNode !== scope) {
    if (line.parentNode) line.parentNode.removeChild(line);
    scope.appendChild(line);
  }
  var id = draggedCard.dataset.card;
  var position = dropPosition(column, draggedCard);
  var target = column.dataset.column;
  endDragVisuals(); // the slot, line, and chip are drag chrome; the swap
  // below replaces the markup with the server's truth
  fetch('/ui/cards/' + id + '/move', {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ column: target, position: position })
  }).then(function (resp) {
    var type = resp.headers.get('Content-Type');
    if (type && type.indexOf('text/html') === 0) {
      // Success fragments and the stated 404/422 fragments are both already
      // rendered HTML for the board surface — the same swap-the-region
      // machinery every other operation uses, just issued by fetch instead
      // of htmx. A stale drop's body is the shared stale-failure surface
      // (stale.go), landing in the same #board-area as every other verb's.
      return resp.text().then(function (html) {
        document.getElementById('board-area').innerHTML = html;
      });
    }
    return resp.text().then(function () {}); // plain error leg: board keeps
    // the last server truth (the stale-failure surface owns the 404 class)
  }).catch(function () {
    // Transport failure: the board still shows the last server truth.
  });
});

document.body.addEventListener('dragend', function () {
  // Fires at the end of EVERY drag — after an accepted drop (already
  // cleaned, the swap pending or landed) and after every abandoned one
  // (card back into its slot, nothing sent). It issues no requests itself.
  endDragVisuals();
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
// done card's text is editable). The delete control is live since KW4 (card
// ui/07): its click issues hx-delete DELETE /ui/cards/{id} through htmx — no
// confirmation dialog, the card text states activation, not confirmation —
// and the answer swaps the truth without the card into #board-area, on every
// card in every column including Done (a done card is as deletable as any
// other; done is just a column). Since KW5 (cards ui/08–10) every card is
// also draggable by the card itself — the page's only movement mechanic —
// and the shell script below carries the drag machinery.
var boardTmpl = template.Must(template.New("board").Parse(`<div id="board" class="board">
{{- range .}}
<section id="column-{{.Anchor}}" class="column" data-column="{{.Title}}">
<h2 class="column__title">{{.Title}}</h2>
{{if .Cards}}<ul class="column__cards">{{- range .Cards}}
<li id="card-{{.ID}}" class="card{{if .Done}} card--done{{end}}" data-card="{{.ID}}" draggable="true">
<span class="card__title">{{.Title}}</span>
<form class="edit-form" hx-patch="/ui/cards/{{.ID}}" hx-target="#board-area" hx-swap="innerHTML">
<input class="input" type="text" name="title" value="{{.Title}}">
<button type="submit" class="btn btn--primary save">Save</button>
<button type="button" class="btn btn--secondary cancel">Cancel</button>
</form>
<button type="button" class="btn btn--secondary card__edit">Edit</button>
<button type="button" class="btn btn--secondary card__delete" hx-delete="/ui/cards/{{.ID}}" hx-target="#board-area" hx-swap="innerHTML">Delete</button>{{if .EditError}}
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
