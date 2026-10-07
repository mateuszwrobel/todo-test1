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
// object carrying "title", "column" and/or "position" — hand the present
// fields to the board operations (the contract's "at least one field"
// distinction), and map outcomes to transport at one site keyed on the
// board's typed errors with errors.Is (the api workplan's mapping-table
// decision). Success is 200 with the updated Card encoded straight from
// board's JSON tags — the contract's four fields (id, title, column,
// position), so a move answers the card in its NEW column and position
// (card api/06). The table now covers the full board.Change and board.Move
// outcome enumeration: not-found api/07, invalid-column api/08 (the enum
// guard is the same typed outcome in both operations), the title class
// api/05, the shape-only empty-change refusal api/09, and the done freeze
// (api/05 amendment 2026-10-07, user decision): a title present for a card
// whose CURRENT column is done is refused with one stated 422 before
// anything moves — every leg that reaches Change answers it, and Move never
// carries a title, so moving out of Done stays the one-request unlock.
//
// Field combinations route through applyPatch below: the title/column legs
// keep Change's shipped mapping (title edit, bottom-append column move),
// column+position is the card's move leg straight to Move, and position
// alone is the same-column reorder — the contract lists position as an
// independent field under "and/or", so the handler finds the card's current
// column through the read and Moves into it.
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
// JSON null all mean "field absent"; the pointers stay nil. A column value
// is handed over as-is — the enum guard is board's, so a bad name arrives
// as board's invalid-column outcome rather than a second copy here.
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
		}

		// Card api/09 — the contract's "at least one field" clause is a
		// request-shape rule, so this module states the refusal: an empty
		// object — and every other no-fields body (absent keys, JSON null
		// values, empty body, well-formed JSON that is not an object) — is
		// rejected here, 422 with the rule stated. Any one of the three
		// fields satisfies the clause; position included (the contract
		// lists it beside title and column). Unlike the title class, where
		// the empty value is handed to board so its required outcome words
		// the refusal once, no store outcome carries this rule; the
		// all-nil call is never made and nothing reaches storage.
		if title == nil && column == nil && position == nil {
			errorJSON(w, http.StatusUnprocessableEntity, "at least one field is required")
			return
		}

		updated, err := applyPatch(store, id, title, column, position)
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
func applyPatch(store BoardStore, id int64, title *string, column *board.Column, position *int) (board.Card, error) {
	switch {
	case position == nil:
		return store.Change(id, title, column)
	case title != nil && column != nil:
		// The returned card is superseded by Move's placement below.
		if _, err := store.Change(id, title, column); err != nil {
			return board.Card{}, err
		}
		return store.Move(id, *column, *position)
	case title != nil:
		updated, err := store.Change(id, title, nil)
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
	case errors.Is(err, board.ErrDoneFrozen):
		// Card api/05 amendment 2026-10-07 (contract-level done freeze, user
		// decision): a title aimed at a card whose current column is done is
		// refused — 422 with one stated message, mirroring the retired todo
		// app's frozen-todo class verbatim in shape ("cannot edit a done
		// todo", api/change.go at 8195fde) restated for cards. The board's
		// typed error is the single rule source; its freeze check precedes
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
