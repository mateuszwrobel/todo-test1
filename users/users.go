// Package users is the simulation's fixed cast: who exists as an assignable
// user, and in what display order. It is plain data — one constant list and a
// membership check — with no state, no I/O, and no imports at all: the cast
// changes when the simulation changes (a new cast, real accounts someday),
// never because board data, the wire contract, or the page changed. Anyone who
// later wants real accounts replaces this module and keeps the same Names /
// IsMember contract; nothing else changes (workplan_users_roster.md).
package users

// roster is the fixed cast in contract display order. It stays unexported so
// the only window onto it is Names' copy: the order is contract, and no
// caller may reorder what every dropdown and gallery lists.
var roster = []string{"Ada", "Grace", "Alan", "Barbara", "Linus"}

// Names returns the roster's names in contract order — Ada, Grace, Alan,
// Barbara, Linus — the same order, every time. The returned slice is a copy
// on purpose: callers list the cast, they cannot edit the contract.
func Names() []string {
	out := make([]string, len(roster))
	copy(out, roster)
	return out
}

// IsMember reports whether name is one of the cast, by case-sensitive exact
// match. The names are program constants, so there is no lenient matching to
// invent: case variants ("grace"), padded spellings, and the empty string are
// all non-members.
func IsMember(name string) bool {
	for _, known := range roster {
		if name == known {
			return true
		}
	}
	return false
}
