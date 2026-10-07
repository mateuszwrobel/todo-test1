package users

import (
	"reflect"
	"strings"
	"testing"
)

// Card users/01 — Roster lists the simulation names.
// Given the program is built
// When  the roster is asked for its names
// Then  it answers Ada, Grace, Alan, Barbara, Linus — in that order, every time
//
//	And membership holds for exactly those five names, case-sensitively
//	And every other string — including empty and case variants like "grace" — is not a member
//
// The order is contract (dropdowns and galleries list the cast in one fixed
// order everywhere), so it is pinned by exact slice equality, and "every time"
// is pinned by asking repeatedly. Names hands out a copy, so a caller
// scribbling on its answer cannot rewrite the contract either — that leg is
// what makes "every time" mean every time, not "until someone edits it".
func TestRosterListsTheSimulationNames(t *testing.T) {
	want := []string{"Ada", "Grace", "Alan", "Barbara", "Linus"}

	for call := 1; call <= 3; call++ {
		got := Names()
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Names() call %d = %v, want %v — the order is contract", call, got, want)
		}
	}

	// The copy leg: mutating an answer changes no later answer.
	scribbled := Names()
	scribbled[0] = "Someone else"
	if got := Names(); !reflect.DeepEqual(got, want) {
		t.Errorf("Names() after a caller mutated an earlier answer = %v, want %v", got, want)
	}
}

// The membership legs, pinned in both directions: each of the five names is a
// member, and every other spelling — the empty string, case variants, padded
// and truncated spellings, arbitrary junk — is not. Matching is case-sensitive
// exact: the cast is program constants, so lenient comparison would invent a
// rule nobody stated.
func TestRosterMembershipIsTheFiveNamesExactly(t *testing.T) {
	members := []string{"Ada", "Grace", "Alan", "Barbara", "Linus"}
	nonMembers := []string{
		"",            // the empty string is explicitly not a member
		"   ",         // padded nothing
		"grace",       // the scenario's case variant
		"GRACE",       // ... and its shouty twin
		"Grace ",      // trailing space — exact match, no trimming
		" ada",        // leading space
		"ADA",         // all caps
		"Alan P.",     // a roster name plus a suffix is not the roster name
		"Ada,Grace",   // a joined pair is not a name
		"Ada\n",       // a newline-pinned name is not a name
		"\u00e9clair", // non-ASCII junk
		"todo",        // a board column is not a person
	}

	for _, name := range members {
		if !IsMember(name) {
			t.Errorf("IsMember(%q) = false, want true — %q is on the roster", name, name)
		}
	}
	for _, name := range nonMembers {
		if IsMember(name) {
			t.Errorf("IsMember(%q) = true, want false — membership holds for exactly the five", name)
		}
	}

	// The set is closed both ways: membership agrees with Names(). Every
	// listed name is a member, and no case variant of a listed name is one —
	// the pinned spellings differ exactly where a lenient rule would let them
	// match.
	for _, name := range Names() {
		if !IsMember(name) {
			t.Errorf("IsMember(%q) = false but Names() lists it — membership must match the cast", name)
		}
		if variant := strings.ToLower(name); IsMember(variant) && variant != name {
			t.Errorf("IsMember(%q) = true — matching is case-sensitive, the lowercased variant is not a member", variant)
		}
	}
}
