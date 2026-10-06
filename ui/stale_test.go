package ui

import (
	"net/http"
	"strings"
	"testing"

	"todo/board"
)

// The stale-operation surface (card ui/11): ONE named surface — the banner
// over the server's truth, stated by stale.go's writeStaleFailure and
// swapped into #board-area by every client arm — serving edit, drag-move
// and delete identically. Each entry below acts the way that verb's shipped
// client acts: the edit seam is htmx's form PATCH, the move seam is the
// drop handler's one fetch, the delete seam is htmx's DELETE.

// staleVerb is one card operation as its client issues it.
type staleVerb struct {
	name string
	act  func(t *testing.T, uiURL string, id int64) (int, string)
}

var staleVerbs = []staleVerb{
	{"edit", func(t *testing.T, u string, id int64) (int, string) {
		return patchCardForm(t, u, id, "ghost edit")
	}},
	{"move", func(t *testing.T, u string, id int64) (int, string) {
		return dragMove(t, u, id, "In Progress", 0)
	}},
	{"delete", func(t *testing.T, u string, id int64) (int, string) {
		return clickDelete(t, u, id)
	}},
}

// boardOf extracts the board element from a fragment or a full page — the
// region the failure surface rides on. The board element terminates at the
// last section's close followed by the board's own </div>, so the slice is
// the complete truth render and nothing else.
func boardOf(t *testing.T, html string) string {
	t.Helper()
	const open = `<div id="board" class="board">`
	const closeTag = `</section>
</div>`
	start := strings.Index(html, open)
	if start < 0 {
		t.Fatalf("no board element:\n%s", html)
	}
	end := strings.Index(html[start:], closeTag)
	if end < 0 {
		t.Fatalf("board element unterminated:\n%.400s", html[start:])
	}
	return html[start : start+end+len(closeTag)]
}

// Card ui/11 — Stale operation states the failure, for every verb and one
// surface.
// Given the page shows a card that the server no longer holds — produced
// end-to-end through the real store: the card exists and renders, then is
// deleted out-of-band before the operation, exactly the other-session case
// the journeys name —
// When  the user edits, drags, or deletes that card
// Then  the operation is not applied — the card is not left looking as if
//
//	it changed — and the page states that the card does not exist, in the
//	contract's own words, at the ONE named surface every verb shares: the
//	same #missing-card banner element, over the board re-rendered as
//	exactly the server's truth (board diff = nothing but the truth), at
//	the mirrored 404 — and byte-for-byte the same fragment for all three
//	verbs. A reload reads that same truth, so the reload arm resolves the
//	staleness the same page the failure swapped in already shows.
func TestStaleOperationsServeOneFailureSurfaceAcrossVerbs(t *testing.T) {
	var bodies []string // one fragment per verb, compared at the end

	for _, verb := range staleVerbs {
		verb := verb
		t.Run(verb.name, func(t *testing.T) {
			store, uiSrv := realChain(t, "Gone soon", "Living on")

			// Given the page shows the card that is about to vanish.
			_, page := getPage(t, uiSrv.URL+"/")
			if !strings.Contains(page, `id="card-1"`) || !strings.Contains(page, `id="card-2"`) {
				t.Fatalf("initial page shows neither both cards:\n%s", page)
			}

			// Staleness, honestly produced: deleted straight from the real
			// store — invisible to the page, as a delete in another session
			// or another tab would be.
			if err := store.Delete(1); err != nil {
				t.Fatalf("out-of-band delete: %v", err)
			}

			status, frag := verb.act(t, uiSrv.URL, 1)

			// The contract's 404, mirrored onto the fragment, for this verb.
			if status != http.StatusNotFound {
				t.Fatalf("%s: status = %d, want 404 (body %s)", verb.name, status, frag)
			}

			// The ONE named surface: the shared banner element, stating the
			// contract's exact wording (this module never re-words it).
			if !strings.Contains(frag,
				`<div id="missing-card" class="banner" role="alert">no such card</div>`) {
				t.Errorf("%s: the shared stale-failure banner is missing:\n%s", verb.name, frag)
			}

			// Not applied, nothing faked: the stale card appears nowhere,
			// under its old title or its attempted new one.
			if strings.Contains(frag, `id="card-1"`) || strings.Contains(frag, "Gone soon") ||
				strings.Contains(frag, "ghost edit") {
				t.Errorf("%s: stale card left looking changed:\n%s", verb.name, frag)
			}

			// The truth re-rendered under the statement — the survivor
			// intact.
			if !strings.Contains(frag, `id="card-2"`) || !strings.Contains(frag, "Living on") {
				t.Errorf("%s: board truth not re-rendered under the failure:\n%s", verb.name, frag)
			}

			// "Nothing else changes": the fragment's board is exactly what
			// a fresh load of the page renders — the truth, byte for byte.
			_, fresh := getPage(t, uiSrv.URL+"/")
			if got, want := boardOf(t, frag), boardOf(t, fresh); got != want {
				t.Errorf("%s: board under the failure is not exactly the truth:\n%s\n---\n%s",
					verb.name, got, want)
			}

			// The store never moved for any verb.
			if got := cardTitles(t, store, board.Todo); len(got) != 1 || got[0] != "Living on" {
				t.Errorf("%s: store To Do column = %q, want only the survivor", verb.name, got)
			}

			bodies = append(bodies, frag)
		})
	}

	// One surface, not three look-alikes: against the same board truth, the
	// three verbs' stale answers are the same fragment byte for byte.
	for i := 1; i < len(bodies); i++ {
		if bodies[i] != bodies[0] {
			t.Errorf("%s's stale fragment differs from edit's — not one surface:\n%s\n---\n%s",
				staleVerbs[i].name, bodies[0], bodies[i])
		}
	}
}
