package api

import (
	"encoding/json"
	"net/http"

	"todo/users" // the roster the GET /users contract body is served from
)

// usersResponse is the GET /users contract body: the cast's names in the one
// fixed display order. The list is the whole contract — no id, no state, no
// paging.
type usersResponse struct {
	Users []string `json:"users"`
}

// handleUsers implements GET /users: the simulated roster, served straight
// from the users module. The api holds no copy of the names — users.Names is
// the single source, so the endpoint and board's validity rule can never
// list two different casts (the roster workplan's boundary: one owner for
// "who exists"; the api only surfaces it, workplan_api_board.md Boundaries
// added 2026-10-07). The order is contract (every dropdown lists the cast in
// that order), and Names hands back its copy for exactly that reason: this
// handler lists the cast, it cannot edit it. users owns no state and no I/O,
// so this endpoint has no failure mode to state — no store call, no error
// path.
func handleUsers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(usersResponse{Users: users.Names()}); err != nil {
			return // status already sent; nothing left to state
		}
	}
}
