package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"todo/board"
)

// handleCardChange implements PATCH /cards/{id}: parse the body — a JSON
// object carrying "title", "column", "position", "assignee", and/or the
// move pair "slot"+"within" — hand the present fields to the board
// operations (the contract's "at least one field" distinction), and map
// outcomes to transport at one site keyed on the board's typed errors with
// errors.Is (the api workplan's mapping-table decision). Success is 200 with
// the updated Card encoded straight from board's JSON tags — the contract's
// five fields (id, title, column, position, assignee; the assignee as name
// or null, api/13), so a move answers the card in its NEW column and
// position (card api/06) — the slot pair's move likewise, its position the
// ABSOLUTE index the slot resolved to (card api/15) — and an assignee edit
// answers the assignment it just made. The table now covers the full
// board.Change and board.Move outcome enumeration plus the filtered move's
// additions: not-found api/07, invalid-column api/08 (the enum guard is the
// same typed outcome in both operations), the title class api/05, the
// shape-only empty-change refusal api/09, the unknown-user class api/13
// (shared by the pair's "within", card api/15), the slot-range class
// api/15, and the done freeze (api/05 amendment 2026-10-07, user decision,
// widened by api/13): a title or an assignee present for a card whose
// CURRENT column is done is refused with one stated 422 before anything
// moves — every leg that reaches Change answers it, and Move and
// MoveFiltered never carry either, so moving out of Done stays the
// one-request unlock — the filtered move being such a placement.
//
// Field combinations route through applyPatch below: the title/column legs
// keep Change's shipped mapping (title edit, bottom-append column move),
// column+position is the card's move leg straight to Move, position alone is
// the same-column reorder — the contract lists position as an independent
// field under "and/or", so the handler finds the card's current column
// through the read and Moves into it — and the "slot"+"within" pair, which
// stands instead of position as the filtered move leg, is one MoveFiltered
// call after the handler's pair-shape guards have refused every illegal
// company (position beside it, half a pair, an edit direction beside it).
//
// The title direction reaches board's text rule — the same single rule
// source create uses — so its refusals are worded identically across verbs:
// ErrTextRequired → 422 "title is required", ErrTextTooLong → 422 stating
// the limit with the number formatted from board.MaxTextLen, so PATCH and
// POST cannot word the same error class differently.
//
// Request validation is shape-only (the api workplan's decision): malformed
// JSON, a known field present with a wrong type, and a non-numeric id are
// transport errors — 400 `{"error": "invalid request"}` — the module's
// standing convention, kept from create. A position must be an integer
// number: the body decodes with UseNumber so the literal is kept, and a
// value that is not a number, is fractional, or overflows int is the same
// shape-violation class as a non-string title. A negative integer is a
// well-formed number and stays legal — out-of-range indexes are board's
// clamp rule, not a shape violation, and Move lands them at the stated
// end of the column. Semantic rules (blank text, the character limit, the
// column enum) stay owned by the board module and are reached through its
// typed outcomes, never restated here.
//
// Body-shape decisions mirror create: an empty body (io.EOF), a well-formed
// JSON body that is not an object, an absent key, and a key whose value is
// JSON null all mean "field absent"; the pointers stay nil. The assignee
// field is the stated exception (api/13): its null is the CLEAR direction,
// not an absence — the contract spells the edit pair "roster name or null".
// A column value is handed over as-is — the enum guard is board's, so a bad
// name arrives as board's invalid-column outcome rather than a second copy
// here; an assignee name likewise, never screened here.
func handleCardChange(store BoardStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			errorJSON(w, http.StatusBadRequest, "invalid request")
			return
		}

		// An empty body is io.EOF — it carries no fields, so it belongs to
		// the shape path below, not the transport error. Anything else the
		// decoder rejects (broken syntax, truncation) is malformed JSON:
		// 400 invalid request (create's standing split). UseNumber keeps
		// numbers literal so the position check below reads the exact text
		// the client sent.
		decoder := json.NewDecoder(r.Body)
		decoder.UseNumber()
		var body any
		if err := decoder.Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			errorJSON(w, http.StatusBadRequest, "invalid request")
			return
		}

		var title *string
		var column *board.Column
		var position *int
		var slot *int
		var within *string
		var assignee *board.AssigneeDirection
		if obj, ok := body.(map[string]any); ok {
			if v, present := obj["title"]; present && v != nil {
				text, ok := v.(string)
				if !ok { // present but not a string — not the contract's body shape
					errorJSON(w, http.StatusBadRequest, "invalid request")
					return
				}
				title = &text
			}
			if v, present := obj["column"]; present && v != nil {
				name, ok := v.(string)
				if !ok { // same transport class as a non-string title
					errorJSON(w, http.StatusBadRequest, "invalid request")
					return
				}
				col := board.Column(name)
				column = &col
			}
			if v, present := obj["position"]; present && v != nil {
				number, ok := v.(json.Number)
				if !ok { // not a number at all — the wrong-type class again
					errorJSON(w, http.StatusBadRequest, "invalid request")
					return
				}
				// The contract's position is an integer index (0-based,
				// contiguous), so only an integer literal has the contract's
				// shape. A fraction or a value outside int range is a shape
				// violation, not an index to clamp; a negative integer is
				// well-formed and boards over to Move's clamp.
				index, err := strconv.Atoi(number.String())
				if err != nil {
					errorJSON(w, http.StatusBadRequest, "invalid request")
					return
				}
				position = &index
			}
			// The slot half of the move pair (card api/15) has position's
			// shape rules — an integer literal or the standing wrong-type/
			// non-integer transport class, 400 — and the standing leniency
			// for a well-formed negative: the contract's slot range is a
			// semantic rule ("slot": <int ≥ 0>), so a negative integer
			// travels to board and comes back as ErrInvalidSlot, the same
			// one-rule-one-owner pattern the column enum follows. The
			// within half is a plain string field: its roster validity is
			// board's rule through the users contract, never screened here;
			// the literal "unassigned" is the sentinel keyword, board's to
			// interpret. JSON null on either half means absent, this
			// handler's standing rule (assignee's null-direction is the one
			// stated exception, and it is not a move field).
			if v, present := obj["slot"]; present && v != nil {
				number, ok := v.(json.Number)
				if !ok { // not a number at all — the wrong-type class again
					errorJSON(w, http.StatusBadRequest, "invalid request")
					return
				}
				index, err := strconv.Atoi(number.String())
				if err != nil { // fraction or overflow: position's exact class
					errorJSON(w, http.StatusBadRequest, "invalid request")
					return
				}
				slot = &index
			}
			if v, present := obj["within"]; present && v != nil {
				name, ok := v.(string)
				if !ok { // same transport class as a non-string title
					errorJSON(w, http.StatusBadRequest, "invalid request")
					return
				}
				within = &name
			}
			// The assignee field (card api/13) is the one field whose JSON
			// null CARRIES a direction instead of meaning absent: the
			// contract spells clear as null ("assignee": <roster name or
			// null>), so a present key branches on its value — null is the
			// ClearAssignee direction, a string is AssignTo, absent leaves
			// the card's assignee alone. A value that is neither (a number,
			// a bool, an array) is the same wrong-type transport class as a
			// non-string title: 400. Roster names are NOT checked here —
			// validity is board's rule through the users contract
			// (shape-only validation here), so an unknown name arrives as
			// board's ErrUnknownAssignee, never a second copy of the rule.
			if v, present := obj["assignee"]; present {
				if v == nil {
					assignee = board.ClearAssignee()
				} else {
					name, ok := v.(string)
					if !ok {
						errorJSON(w, http.StatusBadRequest, "invalid request")
						return
					}
					assignee = board.AssignTo(name)
				}
			}
		}

		// Card api/09 — the contract's "at least one field" clause is a
		// request-shape rule, so this module states the refusal: an empty
		// object — and every other no-fields body (absent keys, JSON null
		// values, empty body, well-formed JSON that is not an object) — is
		// rejected here, 422 with the rule stated. Any one of the fields
		// satisfies the clause; position included (the contract lists it
		// beside title and column), assignee likewise — including its
		// null spelling, which is a direction (clear), not an absence
		// (card api/13) — and the move pair's halves likewise, since
		// "slot"+"within" stand INSTEAD of position (api workplan
		// amendment 2026-10-07). Unlike the title class, where the empty
		// value is handed to board so its required outcome words the
		// refusal once, no store outcome carries this rule; the all-nil
		// call is never made and nothing reaches storage.
		if title == nil && column == nil && position == nil && assignee == nil && slot == nil && within == nil {
			errorJSON(w, http.StatusUnprocessableEntity, "at least one field is required")
			return
		}

		// The pair's shape rules (card api/15, api workplan amendment
		// 2026-10-07: errors "422 stated (slot+position combined, slot
		// without within, negative slot)"). These are body-shape rules —
		// which fields a body may carry together — so this module states
		// the refusals at one site, exactly as api/09's "at least one
		// field" clause is stated: no board call, nothing read, nothing
		// written. The pair stands INSTEAD of position ("slot": <int ≥ 0>
		// + "within": <exact roster name or "unassigned"> — "a move
		// positioned among the matching cards"): position beside slot is
		// contradictory targeting and refused; half a pair locates nothing
		// and is refused; and because the contract spells the pair a MOVE,
		// the edit directions it would otherwise combine with at this verb
		// — title, assignee — are refused beside it: a slot body that also
		// edits would answer two contracts at once, and no card asks for
		// that combo. A negative slot is NOT refused here: it is well-
		// formed JSON of the contract's field type, the contract's range
		// is board's rule, and board answers it as ErrInvalidSlot through
		// the one mapping site.
		if position != nil && slot != nil {
			errorJSON(w, http.StatusUnprocessableEntity, "position and slot are mutually exclusive")
			return
		}
		if slot != nil && within == nil {
			errorJSON(w, http.StatusUnprocessableEntity, "slot requires within")
			return
		}
		if within != nil && slot == nil {
			errorJSON(w, http.StatusUnprocessableEntity, "within requires slot")
			return
		}
		if slot != nil && (title != nil || assignee != nil) {
			errorJSON(w, http.StatusUnprocessableEntity, "cannot combine slot with title or assignee")
			return
		}

		updated, err := applyPatch(store, id, title, column, position, slot, within, assignee)
		if err != nil {
			writeChangeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(updated); err != nil {
			return // status already sent; nothing left to state
		}
	}
}

// applyPatch routes one validated PATCH body onto the board operations,
// honoring each field's presence:
//
//   - position absent — Change as shipped: title edit, column bottom-append,
//     or both in one transaction (the api/05 and api/08 legs and the
//     title+column combination, all unchanged by the move card).
//   - column + position — board.Move, the card's move leg: target column and
//     target index in one transaction.
//   - title + position — Change carries the title first (its text rule is
//     the only title validator, and it runs before any write; under the
//     2026-10-07 done freeze a done card's title direction is refused here
//     too), then Move places the card in the column it now stands in. Two
//     writes, not one transaction: the cards demand no cross-field
//     atomicity, and every stated refusal (title class, not-found, done
//     freeze) lands on the first call with the board untouched — only an
//     internal store error can split them.
//   - title + column + position — Change validates text and column together
//     (appending to the target's bottom), then Move places at the index.
//     Change's enum guard runs before its first write, so an invalid column
//     refuses with the board untouched exactly like the column-only leg.
//   - position alone — the same-column reorder: the contract's "and/or"
//     makes position an independent field, so the handler finds the card's
//     current column through the contract's own read and Moves into it.
//     A miss is the stated not-found outcome without touching storage.
//
// An assignee direction (card api/13) joins the Change call each routed leg
// already makes — or, on a leg that was a bare Move, makes one first (Change
// edits the assignee before Move places): every validity and freeze decision
// then stays at Change's single ordering — roster validity first, then the
// text rules, then the enum, then not-found, then the freeze — instead of a
// second ordering invented at the transport. The assignee-nil legs route
// exactly as before this increment, operation for operation.
//
// The slot+within pair (card api/15) routes last and alone: the guard site
// in the handler has already established that a present slot travels with a
// present within and with no title, no assignee and no position, so the pair
// is ONE MoveFiltered call — the column direction passed through as stated
// (nil re-slots within the card's current column, board resolves that inside
// the transaction, keeping validity's rank over not-found without a
// read-first window). board owns every semantic decision of the leg —
// keyword validity, the column enum, slot non-negativity, existence — in
// its one order; this function only names the operation.
func applyPatch(store BoardStore, id int64, title *string, column *board.Column, position *int, slot *int, within *string, assignee *board.AssigneeDirection) (board.Card, error) {
	switch {
	case slot != nil:
		return store.MoveFiltered(id, column, *slot, *within)
	case position == nil:
		return store.Change(id, title, column, assignee)
	case title != nil && column != nil:
		// The returned card is superseded by Move's placement below.
		if _, err := store.Change(id, title, column, assignee); err != nil {
			return board.Card{}, err
		}
		return store.Move(id, *column, *position)
	case title != nil:
		updated, err := store.Change(id, title, nil, assignee)
		if err != nil {
			return board.Card{}, err
		}
		return store.Move(id, updated.Column, *position)
	case column != nil && assignee != nil:
		// The Change-first pattern of the title+column+position leg: Change
		// screens the column enum and the assignee before any write and
		// appends at the target's bottom; Move then places at the index.
		if _, err := store.Change(id, nil, column, assignee); err != nil {
			return board.Card{}, err
		}
		return store.Move(id, *column, *position)
	case assignee != nil:
		// Position reorder plus an assignee edit: Change carries the edit
		// (validity and freeze decided at its one ordering) and answers the
		// card's current column; Move then reorders within it. A Done card
		// refuses here — the assignee makes this an edit, not a bare move.
		updated, err := store.Change(id, nil, nil, assignee)
		if err != nil {
			return board.Card{}, err
		}
		return store.Move(id, updated.Column, *position)
	case column != nil:
		return store.Move(id, *column, *position)
	default:
		// Honest read-then-move window: a column change racing in between this
		// List lookup and Move resolves last-write-wins per the parent
		// contract, and the single-conn store (board/store.go MaxOpenConns(1))
		// bounds the interleaving.
		current, found, err := currentColumn(store, id)
		if err != nil {
			return board.Card{}, err
		}
		if !found {
			// The same typed outcome Move and Change answer for a missing
			// id, so the one mapping site words the refusal identically.
			return board.Card{}, board.ErrCardNotFound
		}
		return store.Move(id, current, *position)
	}
}

// currentColumn finds one card's column through the contract's read. It
// exists for the position-alone reorder leg, whose target column is the
// card's current one — the minimal honest lookup, the same List that backs
// GET /board, with no second read path.
func currentColumn(store BoardStore, id int64) (board.Column, bool, error) {
	list, err := store.List()
	if err != nil {
		return "", false, err
	}
	for _, col := range list {
		for _, card := range col.Cards {
			if card.ID == id {
				return col.Name, true, nil
			}
		}
	}
	return "", false, nil
}

// writeChangeError is the one outcome → transport mapping site for the
// PATCH verb, keyed on the board's typed outcomes (api workplan decision).
// It covers both operations the handler reaches: Change's full enumeration
// and Move's subset (not-found and invalid-column; the text classes are
// unreachable from a move but belong to the same table, and the contract
// demands their wording wherever they do appear).
func writeChangeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, board.ErrUnknownAssignee):
		// Card api/13 — the contract's stated unknown-user body. The
		// validity rule has one owner (the users module, screened by board
		// before anything else is consulted — Change's ordering for the
		// assignee direction, MoveFiltered's for the pair's "within", so
		// card api/15's "within names a stranger to the roster" answers
		// this same body). The board's ordering is contract — roster
		// validity "outranking text rules, not-found, and the done freeze"
		// (board/store.go, Change) and outranking even the slot guards and
		// the missing target for the move pair (board/store_filter.go) —
		// and every assignee-bearing leg routes through that one screen,
		// so the combined requests (unknown + Done, unknown + blank title,
		// unknown + missing id, unknown + bad slot) arrive here already
		// decided in favor of this class.
		errorJSON(w, http.StatusUnprocessableEntity, "unknown user")
	case errors.Is(err, board.ErrCardNotFound):
		// Card api/07 — the contract's stated not-found body. The board's
		// existence check is its transaction's first read (and the reorder
		// lookup is a pure read), so every rejected patch left the board
		// exactly as it was.
		errorJSON(w, http.StatusNotFound, "no such card")
	case errors.Is(err, board.ErrInvalidColumn):
		// Card api/08 — the contract's stated column refusal. The enum
		// guard lives in board (shape-only validation here), and it runs
		// before any write in both Change and Move, so the card is unchanged.
		errorJSON(w, http.StatusUnprocessableEntity, "invalid column")
	case errors.Is(err, board.ErrInvalidSlot):
		// Card api/15 — the contract's stated slot-range refusal ("422
		// stated (slot+position combined, slot without within, negative
		// slot)", api workplan amendment 2026-10-07; the first two are
		// body-shape rules stated at the handler, this one is board's
		// semantic rule and arrives typed). The guard runs before the
		// transaction opens in MoveFiltered, so the card is unchanged —
		// the same one-rule-one-owner pattern as the column enum.
		errorJSON(w, http.StatusUnprocessableEntity, "invalid slot")
	case errors.Is(err, board.ErrDoneFrozen):
		// Card api/05 amendment 2026-10-07, widened by the assignee contract
		// (api/13): a title OR an assignee aimed at a card whose current
		// column is done is refused — 422 with one stated message, one site
		// for both directions, mirroring the retired todo app's frozen-todo
		// class verbatim in shape ("cannot edit a done todo", api/change.go
		// at 8195fde) restated for cards. The board's typed error is the
		// single rule source — the freeze is one rule, so the message is
		// one message wherever it appears — and its freeze check precedes
		// any write, so the card is unchanged.
		errorJSON(w, http.StatusUnprocessableEntity, "cannot edit a done card")
	case errors.Is(err, board.ErrTextRequired):
		errorJSON(w, http.StatusUnprocessableEntity, "title is required")
	case errors.Is(err, board.ErrTextTooLong):
		// The limit number has one owner — the board constant — and
		// this is create's exact wording, so the class reads the
		// same from either verb.
		errorJSON(w, http.StatusUnprocessableEntity,
			fmt.Sprintf("title exceeds the %d character limit", board.MaxTextLen))
	default:
		http.Error(w, "failed to change card", http.StatusInternalServerError)
	}
}

// errorJSON writes the contract's one error shape: { "error": message }.
func errorJSON(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
