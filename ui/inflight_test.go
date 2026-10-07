package ui

import (
	"strconv"
	"strings"
	"testing"
)

// The in-flight blocking seams (card ui/12, journeys' interaction decision
// carried over from the todo app): the control that triggers an operation is
// disabled from the moment the request leaves until the response arrives, so
// a repeat activation of that same control cannot start a second operation.
// The scope the card's text decides is TRIGGERED-ONLY: the scenario re-acts
// "the same control", and the journeys name one control per operation (the
// create input and its button, a card's edit-save, its delete control, the
// card being dragged) — never the whole board at once.
//
// Two honest server-side halves live here. Markup seams: every htmx control
// declares its own blocking through hx-disabled-elt, statically knowable per
// rendered element; the bundled htmx restores each element on every settled
// path (its onload leg runs for 2xx AND 4xx, onerror covers transport
// failure, onabort for aborts), so "ready again exactly when the response
// arrives" needs no page JS for those verbs. Script seams: the drag fetch is
// the one site with no library to lean on, so its disable and restore call
// sites are pinned structurally — like the fetch-count pin of ui/08, this
// proves the mechanism exists and sits on the one settle path, not that a
// live browser blocks a repeat gesture. The gestures themselves —
// double-clicked Delete sending one request, a dragged card refusing a
// second drag until its PATCH answers, Enter-key re-submit against a disabled
// save — are deferred to the wave-end e2e lane.

// Card ui/12 (markup seams) — every operation's triggered control carries
// its own in-flight block in the rendered page: the create form covers both
// input and button (a live input would re-submit on Enter while the request
// is in flight), each card's edit form disables its save control, each
// card's delete control disables itself. Delete counts in every column,
// Done included; the edit-form seam rides cards outside Done only (the
// done freeze of 2026-10-07 — a Done card renders no edit form, so there
// is no save control to block).
func TestInFlightBlockingWiringIsOnEveryControl(t *testing.T) {
	uiSrv := uiServer(t, boardAPI(t).URL)

	_, page := getPage(t, uiSrv.URL+"/")

	if !strings.Contains(page, `hx-disabled-elt="#create-form .input, #create-form button[type=submit]"`) {
		createForm := page[strings.Index(page, `<form id="create-form"`):]
		if len(createForm) > 200 {
			createForm = createForm[:200]
		}
		t.Errorf("create form lacks its in-flight block over input and button:\n%s", createForm)
	}

	for _, id := range []int64{1, 2, 3, 4} {
		idStr := strconv.FormatInt(id, 10)
		card := cardHTML(t, page, id)

		if id != 4 { // cards outside Done — the canned board's Done card is 4
			if !strings.Contains(card, `hx-disabled-elt="#card-`+idStr+` .save"`) {
				t.Errorf("card %d edit form lacks its save-control block:\n%s", id, card)
			}
		} else if strings.Contains(card, `.save"`) {
			t.Errorf("done card %d carries a save-control block for a frozen edit form:\n%s", id, card)
		}

		// The delete control's block is "this" — the element itself — and it
		// must sit on the very element carrying hx-delete, nowhere else.
		at := strings.Index(card, `class="btn btn--secondary card__delete"`)
		if at < 0 {
			t.Fatalf("card %d has no delete control:\n%s", id, card)
		}
		tag := card[at : at+strings.Index(card[at:], ">")]
		if !strings.Contains(tag, `hx-delete="/ui/cards/`+idStr+`"`) ||
			!strings.Contains(tag, `hx-disabled-elt="this"`) {
			t.Errorf("card %d delete control lacks its self-block or its verb:\n%s", id, tag)
		}
	}
}

// Card ui/12 (script seams) — the drag side's blocking exists and rides the
// one settle path: the drop disables the dragged card before the single
// fetch leaves (both by clearing the pending state's marker and the card's
// draggable), dragstart refuses to start a drag on the card whose drop is
// still unanswered, and exactly one finally restores the control — after the
// catch, so the swallowed transport leg settles it too. Success, stated
// failure and network error therefore share the one ready-again path. What
// a live browser does with those guards (repeat gestures against a pending
// request) is the wave-end e2e lane's proof.
func TestDragInFlightBlocksOnOneSettlePath(t *testing.T) {
	uiSrv := uiServer(t, boardAPI(t).URL)

	_, page := getPage(t, uiSrv.URL+"/")
	script := page[strings.LastIndex(page, "<script>"):]

	if n := strings.Count(script, "fetch("); n != 1 {
		t.Errorf("page script has %d request sites, want the one per-drop fetch untouched", n)
	}
	if n := strings.Count(script, ".finally("); n != 1 {
		t.Errorf("page script has %d settle paths, want exactly one finally", n)
	}

	dropLeg := script[strings.Index(script, "addEventListener('drop'"):strings.Index(script, "addEventListener('dragend'")]
	disable := strings.Index(dropLeg, "dragPending = pending")
	draggableOff := strings.Index(dropLeg, "pending.draggable = false")
	request := strings.Index(dropLeg, "fetch(")
	catch := strings.Index(dropLeg, ".catch(")
	settle := strings.Index(dropLeg, ".finally(")
	for _, check := range []struct {
		name string
		at   int
	}{
		{"the pending-state disable", disable},
		{"the draggable-off disable", draggableOff},
		{"the request site", request},
		{"the transport-failure leg", catch},
		{"the settle path", settle},
	} {
		if check.at < 0 {
			t.Errorf("drop leg lost %s:\n%.600s", check.name, dropLeg)
		}
	}
	// Disable precedes the request (blocked from the moment it leaves) and
	// the restore follows the catch (every outcome, network error included,
	// lands on the one finally).
	if disable >= 0 && request >= 0 && disable > request {
		t.Errorf("drop leg disables the card only after the request leaves")
	}
	if draggableOff >= 0 && request >= 0 && draggableOff > request {
		t.Errorf("drop leg clears draggable only after the request leaves")
	}
	if catch >= 0 && settle >= 0 && settle < catch {
		t.Errorf("settle path runs before the transport-failure leg — network errors would never restore the control")
	}
	if at := strings.Index(dropLeg, "pending.draggable = true"); at < settle {
		t.Errorf("the restore call site is not inside the finally:\n%.600s", dropLeg)
	}
	if !strings.Contains(dropLeg[settle:], "dragPending = null") {
		t.Errorf("the finally does not clear the pending state — the control would stay blocked")
	}

	dragStartLeg := script[strings.Index(script, "addEventListener('dragstart'"):strings.Index(script, "addEventListener('dragover'")]
	if at := strings.Index(dragStartLeg, "card === dragPending"); at < 0 {
		t.Errorf("dragstart lost its in-flight guard:\n%.400s", dragStartLeg)
	} else if fetchAt := strings.Index(dragStartLeg, "draggedCard = card"); fetchAt >= 0 && fetchAt < at {
		t.Errorf("dragstart arms the drag before consulting the in-flight guard")
	}
}
