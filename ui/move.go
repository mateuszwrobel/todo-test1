package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// The drag-to-move surface (cards ui/08–10, KW5): dragging is the page's
// only movement mechanic (workplan_ui_board.md decision — no button or menu
// fallback exists), and every accepted drop arrives here as exactly ONE
// request carrying the target column and the drop position. The endpoint
// performs the move by issuing PATCH {apiBase}/cards/{id} over real HTTP
// with the contract's {"column": ..., "position": ...} body — or, while
// the URL carries a filter, with the contract's move pair {"column": ...,
// "slot": ..., "within": ...} instead (card ui/18, KW10: the slot counts
// only the visible matching cards, the server resolves it into whole-board
// truth) — one contract call per accepted drop, the module's half of the
// one-request rule the scenario pins — and answers the resulting state as a
// swap fragment targeting #board-area: the fresh board re-read, never a
// local guess (mutations re-render from server truth, same decision as
// edit.go and delete.go; under the pair the re-read carries the same
// filter), so the card lands exactly where the store renormalized it and
// the source column packs its gap.
//
// The position rides the contract's own semantics — the index within the
// target column AFTER the card's removal (board.Move removes before it
// inserts, same-column included). The page's client math reads exactly that
// number off the rendered DOM (the shell's dropPosition, render.go) and this
// handler passes it through untouched; out-of-range values are board's
// clamp rule, not restated here.
//
// The rejection legs reuse the shipped machinery verbatim — no second
// mechanism: the stale card (404) takes the shared stale-failure surface
// (stale.go), byte-for-byte the one surface the edit and delete verbs
// serve, and a contract refusal (422, in practice the invalid-column
// guard) is stated AT the dragged card over the unchanged truth exactly as
// the edit refusal is (attachEditError) — validation stays its own class
// with its own surface, card ui/11 consolidated only the stale one.

// columnKeysByName mirrors the contract's fixed column enum between the
// display titles the page renders (and the drag carries, straight off the
// column panel's data attribute) and the storage keys the contract takes.
// The trio is contract-fixed — the same fixed trio doneColumnTitle keys the
// done treatment on — so this mirror has no other owner. An unknown name
// passes through verbatim: the enum guard is board's, never restated here,
// so a name outside the trio surfaces as the contract's own invalid-column
// refusal through the shared 422 arm.
var columnKeysByName = map[string]string{
	"To Do":       "todo",
	"In Progress": "in_progress",
	"Done":        "done",
}

// handleMove — PATCH /ui/cards/{id}/move. It performs the move by calling
// the api contract over HTTP (never in-process), then answers the resulting
// state as a swap fragment: on success the fresh board (200) — the re-read
// is the lands-at-the-drop-position guarantee, including the source column
// closing its gap and a Done landing rendering with the done treatment
// because column membership is the done state; on a stale card (404) the
// shared stale-failure surface (stale.go, mirrored status — the one surface
// all three verbs serve); on a contract refusal (422) the truth with the
// contract's reason stated at the dragged card, mirrored status; anything
// else the plain error at the contract's status (edit's and delete's
// default arm).
func (p *page) handleMove(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid card id", http.StatusBadRequest)
		return
	}
	// The drop payload the shell's one request site sends: the target
	// column's display title and the drop index — as the contract's
	// absolute position when the page is unfiltered, or as the slot+within
	// pair when the URL carries a filter (card ui/18, KW10): slot is the
	// index counted among the VISIBLE cards of the target column after the
	// card's removal, read off the rendered DOM exactly like position, and
	// within names the active filter the count was taken under. A payload
	// outside those two complete shapes — missing column, position beside
	// a pair, half a pair — is a client defect, not a contract state:
	// transport error, the module's standing shape-only split. The pair
	// legs' semantics (keyword validity, slot range) belong to the
	// contract, never restated here.
	var drop struct {
		Column   *string `json:"column"`
		Position *int    `json:"position"`
		Slot     *int    `json:"slot"`
		Within   *string `json:"within"`
	}
	if err := json.NewDecoder(r.Body).Decode(&drop); err != nil || drop.Column == nil {
		http.Error(w, "invalid move payload", http.StatusBadRequest)
		return
	}
	filtered := drop.Slot != nil && drop.Within != nil
	if drop.Position != nil == filtered {
		// exactly one complete leg: position XOR slot+within
		http.Error(w, "invalid move payload", http.StatusBadRequest)
		return
	}
	var status int
	var reason string
	if filtered {
		status, reason = p.patchCardMoveFiltered(r, id, *drop.Column, *drop.Slot, *drop.Within)
	} else {
		status, reason = p.patchCardMove(r, id, *drop.Column, *drop.Position)
	}

	switch status {
	case http.StatusOK:
		// The re-read is the whole Then-clause: the card renders in the
		// target column at the drop position because the server lists it
		// there, and it is gone from the source column because the server
		// no longer lists it there. Under the pair the re-read carries the
		// same filter the drop was counted under — the view the user is
		// looking at is the filtered one, and the hidden truth underneath
		// is revealed only by clearing the filter (card ui/18's second
		// clause is the full-board GET the pair's resolution guarantees).
		var b bytes.Buffer
		var err error
		if filtered {
			err = p.writeFilteredBoardAreaFragment(&b, *drop.Within)
		} else {
			err = p.writeBoardAreaFragment(&b)
		}
		if err != nil {
			http.Error(w, "render failed", http.StatusInternalServerError)
			return
		}
		writeHTML(w, http.StatusOK, b)
	case http.StatusUnprocessableEntity:
		// The contract's refusal stated AT the dragged card over the
		// server's unchanged truth — the drag's use of the shipped
		// reason-at-the-card mechanism (attachEditError), not a copy.
		var b bytes.Buffer
		if board, err := p.loadBoard(); err == nil {
			columns := columnsOf(board)
			attachEditError(columns, id, reason)
			err = writeFragment(&b, boardTmpl, p.viewFor(columns, ""))
		} else {
			err = writeFragment(&b, failedTmpl, nil)
		}
		if err != nil {
			http.Error(w, "render failed", http.StatusInternalServerError)
			return
		}
		writeHTML(w, http.StatusUnprocessableEntity, b)
	case http.StatusNotFound:
		// The dragged card vanished behind the page's back: the shared
		// stale-failure surface — the same banner over the same truth the
		// edit and delete verbs serve (stale.go). Nothing is left looking
		// moved.
		p.writeStaleFailure(w, reason)
	default:
		http.Error(w, reason, status)
	}
}

// patchCardMove performs the unfiltered move write: PATCH {apiBase}/cards/{id}
// over real HTTP with the contract's {"column": ..., "position": ...} body —
// the display title mapped through the fixed enum mirror above, the position
// passed through untouched. It returns the contract's status and, on a
// refusal, its stated reason; the message's single owner stays upstream
// (api/board), this module only carries it.
func (p *page) patchCardMove(r *http.Request, id int64, column string, position int) (status int, reason string) {
	body, err := json.Marshal(map[string]any{
		"column":   columnKey(column),
		"position": position,
	})
	if err != nil {
		return http.StatusInternalServerError, "could not move card"
	}
	return p.sendCardPatch(r, id, body)
}

// patchCardMoveFiltered performs the pair's move write (card ui/18): the
// same PATCH over real HTTP carrying the contract's move pair —
// {"column": ..., "slot": ..., "within": ...} instead of position. slot is
// the index among the cards visible under within, counted after the card's
// removal — the number the page read off the rendered DOM; resolving it to
// the whole-board position is the contract's one job (api/15, board's
// MoveFiltered), never second-guessed here.
func (p *page) patchCardMoveFiltered(r *http.Request, id int64, column string, slot int, within string) (status int, reason string) {
	body, err := json.Marshal(map[string]any{
		"column": columnKey(column),
		"slot":   slot,
		"within": within,
	})
	if err != nil {
		return http.StatusInternalServerError, "could not move card"
	}
	return p.sendCardPatch(r, id, body)
}

// sendCardPatch is the move legs' one write site: send the marshaled PATCH
// body to the contract and read back its status and stated reason. The
// fallbacks cover only a body that states nothing: a refusal from the real
// contract always carries its own wording.
func (p *page) sendCardPatch(r *http.Request, id int64, body []byte) (status int, reason string) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPatch,
		fmt.Sprintf("%s/cards/%d", p.apiBase, id), bytes.NewReader(body))
	if err != nil {
		return http.StatusInternalServerError, "could not move card"
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return http.StatusBadGateway, "could not reach the board contract"
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body) // drain for connection reuse
		return http.StatusOK, ""
	}
	buf, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	var parsed map[string]string
	if json.Unmarshal(buf, &parsed) == nil && parsed["error"] != "" {
		return resp.StatusCode, parsed["error"]
	}
	if resp.StatusCode == http.StatusNotFound {
		return resp.StatusCode, "no such card"
	}
	return resp.StatusCode, "could not move card"
}

// columnKey translates a column's display title to the contract's storage
// key; names outside the fixed trio pass through verbatim so the guard and
// its wording stay upstream (see columnKeysByName).
func columnKey(title string) string {
	if key, ok := columnKeysByName[title]; ok {
		return key
	}
	return title
}
