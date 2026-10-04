package todos

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// MaxTitleLength is the stated title-length limit: characters (Unicode code
// points) of the trimmed title. Callers state the rule by referencing this
// constant — it never hardcodes the number.
const MaxTitleLength = 500

// Invalid-text outcomes of Create. They carry no HTTP semantics; status-code
// mapping is api's decision.
var (
	// ErrTitleRequired is the empty / whitespace-only outcome.
	ErrTitleRequired = errors.New("title is required")
	// ErrTitleTooLong is the over-the-limit outcome; ErrTitleRequired's
	// sibling — both are the contract's invalid-text result.
	ErrTitleTooLong = errors.New("title exceeds the character limit")
)

// validateTitle applies the module's text rules — the single source of truth
// every caller shares: trim, non-empty, at most MaxTitleLength characters.
// The returned string is the canonical (trimmed) form stored and read back.
func validateTitle(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	switch {
	case trimmed == "":
		return "", ErrTitleRequired
	case utf8.RuneCountInString(trimmed) > MaxTitleLength:
		return "", ErrTitleTooLong
	}
	return trimmed, nil
}
