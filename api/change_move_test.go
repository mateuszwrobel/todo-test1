package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"todo/board"
)

// cardID extracts the identifier from a card body decoded by the helpers in
// change_test.go.
func cardID(card map[string]interface{}) int64 {
	return int64(card["id"].(float64))
}

// assertColumnLayout pins one column's List truth: cards in the given title
// order at contiguous positions 0..n-1 — the contract's position rule read
// back after every accepted move.
func assertColumnLayout(t *testing.T, list []board.ColumnCards, column string, want ...string) {
	t.Helper()
	for _, col := range list {
		if col.Name != board.Column(column) {
			continue
		}
		if len(col.Cards) != len(want) {
			t.Fatalf("column %q holds %+v, want the %d card(s) %v", column, col.Cards, len(want), want)
		}
		for i, card := range col.Cards {
			if card.Title != want[i] || card.Position != i {
				t.Errorf("column %q position %d = %+v, want %q at position %d", column, i, card, want[i], i)
			}
		}
		return
	}
	t.Fatalf("board listing has no column %q: %+v", column, list)
}

// moveToColumn relocates a card through the shipped column-only direction
// (Change's bottom-append) — tests seed other columns the way a client does.
func moveToColumn(t *testing.T, url string, id int64, column string) {
	t.Helper()
	if resp, body := patchCard(t, url, id, fmt.Sprintf(`{"column": %q}`, column)); resp.StatusCode != http.StatusOK {
		t.Fatalf("seed move to %s: status = %d, want 200 (body %s)", column, resp.StatusCode, body)
	}
}

// Card api/06 — Patch move returns the moved card.
// Given a card exists with identifier N in the todo column
// When  a client patches {"column": "in_progress", "position": 1} to /cards/N
// Then  the response is 200
//
//	And the body is the card with column "in_progress" and position 1
//
// Tested against the real board store. The scenario needs in_progress to
// hold one card so index 1 is a real slot rather than a past-end clamp — the
// anchor is placed there through the shipped column-only direction. The
// body is the full Card: identity and text survive, the NEW column and
// position are what the wire shows, and the List read agrees.
func TestPatchCardMoveReturnsMovedCard(t *testing.T) {
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	mover := createCardThroughAPI(t, srv.URL, "moving card")
	anchor := createCardThroughAPI(t, srv.URL, "in progress anchor")
	moveToColumn(t, srv.URL, cardID(anchor), "in_progress")

	resp, body := patchCard(t, srv.URL, cardID(mover), `{"column": "in_progress", "position": 1}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	card := assertCardKeys(t, body)
	if card["column"] != "in_progress" {
		t.Errorf("column = %v, want %q", card["column"], "in_progress")
	}
	if card["position"] != float64(1) {
		t.Errorf("position = %v, want 1 — the card lands at the patched index", card["position"])
	}
	if card["title"] != "moving card" {
		t.Errorf("title = %v, want %q — a move changes placement only", card["title"], "moving card")
	}
	if card["id"] != mover["id"] {
		t.Errorf("id = %v, want %v — a move keeps the identifier", card["id"], mover["id"])
	}

	// List truth: the moved card sits at in_progress position 1 after the
	// anchor, and the source column closed its gap with no other card.
	list := boardSnapshot(t, store)
	assertColumnLayout(t, list, "in_progress", "in progress anchor", "moving card")
	assertColumnLayout(t, list, "todo")
	assertColumnLayout(t, list, "done")
}

// The placement direction beyond the single scenario row: the patched index
// is the target column's order AFTER the card is removed from it (board/06
// semantics surfaced at the wire) — top insert, bottom insert, same-column
// reorder through column+position (the column the card is already in), and
// the out-of-range clamps the board documents, all answered with the card's
// real landing position.
func TestPatchCardMovePlacementRows(t *testing.T) {
	rows := []struct {
		name       string
		moveTitle  string // which todo card to move (created top-to-bottom)
		body       string
		wantPos    float64
		wantTodo   []string
		wantInProg []string
		wantDone   []string
	}{
		{
			name:       "insert at the top of the target",
			moveTitle:  "card b",
			body:       `{"column": "in_progress", "position": 0}`,
			wantPos:    0,
			wantTodo:   []string{"card a", "card c"},
			wantInProg: []string{"card b", "anchor"},
			wantDone:   []string{},
		},
		{
			name:       "insert at the bottom of the target",
			moveTitle:  "card b",
			body:       `{"column": "in_progress", "position": 1}`,
			wantPos:    1,
			wantTodo:   []string{"card a", "card c"},
			wantInProg: []string{"anchor", "card b"},
			wantDone:   []string{},
		},
		{
			name:       "past the end clamps to the bottom",
			moveTitle:  "card c",
			body:       `{"column": "done", "position": 99}`,
			wantPos:    0,
			wantTodo:   []string{"card a", "card b"},
			wantInProg: []string{"anchor"},
			wantDone:   []string{"card c"},
		},
		{
			name:       "negative clamps to the top",
			moveTitle:  "card b",
			body:       `{"column": "in_progress", "position": -3}`,
			wantPos:    0,
			wantTodo:   []string{"card a", "card c"},
			wantInProg: []string{"card b", "anchor"},
			wantDone:   []string{},
		},
		{
			name:       "same column with an index reorders",
			moveTitle:  "card c",
			body:       `{"column": "todo", "position": 0}`,
			wantPos:    0,
			wantTodo:   []string{"card c", "card a", "card b"},
			wantInProg: []string{"anchor"},
			wantDone:   []string{},
		},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			store := openBoardStore(t)
			srv := httptest.NewServer(NewHandler(store))
			defer srv.Close()

			cards := map[string]int64{}
			for _, title := range []string{"card a", "card b", "card c"} {
				cards[title] = cardID(createCardThroughAPI(t, srv.URL, title))
			}
			anchor := cardID(createCardThroughAPI(t, srv.URL, "anchor"))
			moveToColumn(t, srv.URL, anchor, "in_progress")

			resp, body := patchCard(t, srv.URL, cards[row.moveTitle], row.body)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body %s)", resp.StatusCode, body)
			}
			card := assertCardKeys(t, body)
			if card["position"] != row.wantPos {
				t.Errorf("body position = %v, want %v (body %s)", card["position"], row.wantPos, body)
			}
			if card["id"] != float64(cards[row.moveTitle]) {
				t.Errorf("body id = %v, want %v — a move keeps the identifier", card["id"], cards[row.moveTitle])
			}

			list := boardSnapshot(t, store)
			assertColumnLayout(t, list, "todo", row.wantTodo...)
			assertColumnLayout(t, list, "in_progress", row.wantInProg...)
			assertColumnLayout(t, list, "done", row.wantDone...)
		})
	}
}

// The contract lists position as an independent field ("text, column,
// and/or position"), so {"position": p} alone is a patch, not a refusal:
// the same-column reorder leg. The handler finds the card's current column
// through the contract's own read and places at p there (board/07 at the
// wire) — including in a column other than todo, and at the clamped ends.
func TestPatchCardPositionAloneReordersWithinColumn(t *testing.T) {
	rows := []struct {
		name     string
		body     string
		wantPos  float64
		wantTodo []string
	}{
		{"down to the bottom", `{"position": 2}`, 2, []string{"card a", "card c", "card b"}},
		{"up to the top", `{"position": 0}`, 0, []string{"card b", "card a", "card c"}},
		{"past the end clamps to the bottom", `{"position": 99}`, 2, []string{"card a", "card c", "card b"}},
		{"negative clamps to the top", `{"position": -1}`, 0, []string{"card b", "card a", "card c"}},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			store := openBoardStore(t)
			srv := httptest.NewServer(NewHandler(store))
			defer srv.Close()

			createCardThroughAPI(t, srv.URL, "card a")
			b := cardID(createCardThroughAPI(t, srv.URL, "card b"))
			createCardThroughAPI(t, srv.URL, "card c")

			resp, body := patchCard(t, srv.URL, b, row.body)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body %s)", resp.StatusCode, body)
			}
			card := assertCardKeys(t, body)
			if card["column"] != "todo" {
				t.Errorf("column = %v, want todo — position alone stays in the current column", card["column"])
			}
			if card["position"] != row.wantPos {
				t.Errorf("body position = %v, want %v (body %s)", card["position"], row.wantPos, body)
			}

			list := boardSnapshot(t, store)
			assertColumnLayout(t, list, "todo", row.wantTodo...)
		})
	}

	// The lookup follows the card wherever it stands: a card parked in
	// done reorders within done, not within todo.
	t.Run("reorders in the column the card actually stands in", func(t *testing.T) {
		store := openBoardStore(t)
		srv := httptest.NewServer(NewHandler(store))
		defer srv.Close()

		first := cardID(createCardThroughAPI(t, srv.URL, "done first"))
		second := cardID(createCardThroughAPI(t, srv.URL, "done second"))
		moveToColumn(t, srv.URL, first, "done")
		moveToColumn(t, srv.URL, second, "done")

		resp, body := patchCard(t, srv.URL, first, `{"position": 1}`)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %s)", resp.StatusCode, body)
		}
		card := assertCardKeys(t, body)
		if card["column"] != "done" || card["position"] != float64(1) {
			t.Errorf("body = %s, want column done position 1", body)
		}
		assertColumnLayout(t, boardSnapshot(t, store), "done", "done second", "done first")
	})
}

// Card api/07 extended to the move legs — "patches any fields": every
// body shape the move direction adds must answer the same stated 404 for
// an unknown identifier, and the board must be exactly as it was. The
// position-alone leg resolves through the read lookup, so its miss never
// reaches a write either.
func TestPatchCardMoveUnknownIDIsStated404(t *testing.T) {
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	createCardThroughAPI(t, srv.URL, "the only card")
	before := boardSnapshot(t, store)

	const missingID = 999
	for _, body := range []string{
		`{"column": "done", "position": 0}`,
		`{"column": "in_progress", "position": -2}`,
		`{"position": 3}`,
		`{"position": -1}`,
		`{"title": "renamed", "position": 0}`,
		`{"title": "renamed", "column": "done", "position": 2}`,
	} {
		resp, got := patchCard(t, srv.URL, missingID, body)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d for body %s, want 404 (body %s)", resp.StatusCode, body, got)
		}
		assertCreateError(t, []byte(got), "no such card")
	}

	assertBoardUnchanged(t, store, before)
}

// Card api/08 extended to the move legs: the enum guard is board's first
// act in both Change and Move, so an invalid column alongside a position
// (and even alongside a valid title) refuses with the stated 422 and the
// board untouched — no partial move, no partial rename.
func TestPatchCardMoveInvalidColumnIsStated422(t *testing.T) {
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	card := createCardThroughAPI(t, srv.URL, "steady card")
	id := cardID(card)
	before := boardSnapshot(t, store)

	for _, body := range []string{
		`{"column": "someday", "position": 1}`,
		`{"column": "BACKLOG", "position": 0}`,
		`{"title": "renamed", "column": "nope", "position": 0}`,
	} {
		resp, got := patchCard(t, srv.URL, id, body)
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d for body %s, want 422 (body %s)", resp.StatusCode, body, got)
		}
		assertCreateError(t, []byte(got), "invalid column")
	}

	assertBoardUnchanged(t, store, before)
	if list := boardSnapshot(t, store); len(list[0].Cards) != 1 || list[0].Cards[0].ID != id || list[0].Cards[0].Position != 0 {
		t.Errorf("card changed after the refusal: %+v, want it alone in todo at position 0", list[0].Cards)
	}
}

// A position that is not an integer number is the module's shape-violation
// class — 400 invalid request, beside the non-string title and column —
// decided before anything reaches the store. Fractions and values outside
// int range are NOT clamped: the contract's position is an integer index,
// so the literal itself has the wrong shape. A negative integer is the
// opposite case — well-formed, handled by the board's clamp (pinned in the
// placement rows above).
func TestPatchCardPositionShapeViolationsAre400(t *testing.T) {
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	card := createCardThroughAPI(t, srv.URL, "untouched card")
	id := cardID(card)
	before := boardSnapshot(t, store)

	for _, body := range []string{
		`{"position": "two"}`,                               // a string is not a number
		`{"position": 1.5}`,                                 // a fraction is not an index
		`{"position": true}`,                                // not a number
		`{"position": [2]}`,                                 // not a number
		`{"position": 12345678901234567890123456789}`,       // outside int range — not an index literal
		`{"title": "x", "position": "first"}`,               // the class survives a valid sibling field
		`{"title": "x", "column": "done", "position": 0.5}`, // same, beside a full move
	} {
		resp, got := patchCard(t, srv.URL, id, body)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d for body %s, want 400 (body %s)", resp.StatusCode, body, got)
		}
		assertCreateError(t, []byte(got), "invalid request")
	}

	assertBoardUnchanged(t, store, before)
}

// Card api/09's "at least one field" clause now spans three fields: a body
// whose only key is a JSON null position supplies no field, so it belongs
// to the empty-body refusal, not to the reorder leg.
func TestPatchCardPositionNullIsStated422(t *testing.T) {
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	card := createCardThroughAPI(t, srv.URL, "card standing pat")
	id := cardID(card)
	before := boardSnapshot(t, store)

	for _, body := range []string{
		`{"position": null}`,
		`{"title": null, "column": null, "position": null}`,
	} {
		resp, got := patchCard(t, srv.URL, id, body)
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d for body %s, want 422 (body %s)", resp.StatusCode, body, got)
		}
		assertCreateError(t, []byte(got), "at least one field is required")
	}

	assertBoardUnchanged(t, store, before)
}

// The contract lets one request carry title and the move directions
// together. The title direction reaches board's text rule before anything
// moves, so every stated refusal lands with the board exactly as it was;
// once accepted, the card shows new text and landed placement at once.
func TestPatchCardTitleAndMoveTogether(t *testing.T) {
	t.Run("title with same-column position renames and reorders", func(t *testing.T) {
		store := openBoardStore(t)
		srv := httptest.NewServer(NewHandler(store))
		defer srv.Close()

		first := cardID(createCardThroughAPI(t, srv.URL, "card a"))
		createCardThroughAPI(t, srv.URL, "card c")

		resp, body := patchCard(t, srv.URL, first, `{"title": "renamed", "position": 1}`)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %s)", resp.StatusCode, body)
		}
		card := assertCardKeys(t, body)
		if card["title"] != "renamed" || card["column"] != "todo" || card["position"] != float64(1) {
			t.Errorf("body = %s, want title renamed in todo at position 1", body)
		}
		assertColumnLayout(t, boardSnapshot(t, store), "todo", "card c", "renamed")
	})

	t.Run("title with column and position lands all three", func(t *testing.T) {
		store := openBoardStore(t)
		srv := httptest.NewServer(NewHandler(store))
		defer srv.Close()

		mover := cardID(createCardThroughAPI(t, srv.URL, "mover"))
		createCardThroughAPI(t, srv.URL, "stays")
		anchor := cardID(createCardThroughAPI(t, srv.URL, "anchor"))
		moveToColumn(t, srv.URL, anchor, "in_progress")

		resp, body := patchCard(t, srv.URL, mover, `{"title": "renamed moved", "column": "in_progress", "position": 0}`)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %s)", resp.StatusCode, body)
		}
		card := assertCardKeys(t, body)
		if card["title"] != "renamed moved" || card["column"] != "in_progress" || card["position"] != float64(0) {
			t.Errorf("body = %s, want renamed moved in in_progress at position 0", body)
		}
		list := boardSnapshot(t, store)
		assertColumnLayout(t, list, "in_progress", "renamed moved", "anchor")
		assertColumnLayout(t, list, "todo", "stays")
	})

	t.Run("a refused title blocks the move entirely", func(t *testing.T) {
		store := openBoardStore(t)
		srv := httptest.NewServer(NewHandler(store))
		defer srv.Close()

		card := createCardThroughAPI(t, srv.URL, "steady card")
		id := cardID(card)
		before := boardSnapshot(t, store)

		resp, body := patchCard(t, srv.URL, id, `{"title": "", "column": "done", "position": 0}`)
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422 (body %s)", resp.StatusCode, body)
		}
		assertCreateError(t, []byte(body), "title is required")
		assertBoardUnchanged(t, store, before)
	})
}
