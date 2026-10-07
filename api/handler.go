// Package api exposes the application's JSON wire contract and translates
// HTTP requests into board module operations. It owns no listener and never
// renders HTML. The board endpoints are the kanban contract
// (workplan_api_board.md); the todo surface is fully retired — its last
// surviving endpoint, DELETE /todos/{id}, retired at api/10 when
// DELETE /cards/{id} took its place.
package api

import (
	"net/http"

	"todo/board"
)

// BoardStore is this module's port onto the board contract: the read the
// GET /board endpoint translates (and its filtered sibling, since KW10, for
// the ?assignee= leg), the create POST /cards translates, the change PATCH
// /cards/{id} translates through Change, Move and — since the api/15 slot
// pair — MoveFiltered (the move direction of the contract's "text, column,
// and/or position" line, the pair standing instead of position), and the
// delete DELETE /cards/{id} translates — the board operations the endpoints
// exist to reach, nothing more. Consumer-defined port: the composition root
// injects the concrete store, so no module depends on another's concrete
// type.
type BoardStore interface {
	List() ([]board.ColumnCards, error)
	// ListFiltered serves the GET /board ?assignee= leg: the same three-
	// column listing narrowed to the keyword's matching cards — an exact
	// roster name or the "unassigned" sentinel — with stored positions
	// unchanged. An unknown keyword is board's ErrUnknownAssignee, the
	// same outcome the PATCH legs carry to 422 "unknown user".
	ListFiltered(keyword string) ([]board.ColumnCards, error)
	Create(text string) (board.Card, error)
	// Change mirrors the store contract's fourth direction (the assignee):
	// the PATCH handler parses it into an AssigneeDirection — AssignTo for a
	// name, ClearAssignee for null, nil when the field is absent — and hands
	// it here; the move legs' Move call carries no direction by contract.
	Change(id int64, title *string, column *board.Column, assignee *board.AssigneeDirection) (board.Card, error)
	Move(id int64, column board.Column, position int) (board.Card, error)
	// MoveFiltered serves the PATCH "slot"+"within" pair (card api/15):
	// the target column as a direction (nil = the card's current column)
	// and the slot counted among the keyword's matching cards of that
	// column; the returned card's Position is the resolved ABSOLUTE index.
	// A nil column and the keyword's validity are decided inside the store,
	// so the contract's rank (validity over not-found) holds without a
	// read-first window at the transport.
	MoveFiltered(id int64, column *board.Column, slot int, keyword string) (board.Card, error)
	Delete(id int64) error
}

// NewHandler builds the handler for the JSON contract: GET /board,
// POST /cards, PATCH /cards/{id}, DELETE /cards/{id}, and — since KW9 —
// GET /users, the simulated roster served straight from the users module. The
// todo endpoints are gone (GET /todos retired at api/01, POST /todos at
// api/02, PATCH /todos at api/05, DELETE /todos at api/10 — the board
// endpoints replaced the whole surface). The composition root mounts it on
// its listener.
func NewHandler(boardStore BoardStore) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /board", handleBoard(boardStore))
	mux.HandleFunc("POST /cards", handleCreate(boardStore))
	mux.HandleFunc("PATCH /cards/{id}", handleCardChange(boardStore))
	mux.HandleFunc("DELETE /cards/{id}", handleCardDelete(boardStore))
	// GET /users takes no store: the roster is program data (the users
	// module), not board state, and this endpoint states it verbatim.
	mux.HandleFunc("GET /users", handleUsers())
	return mux
}
