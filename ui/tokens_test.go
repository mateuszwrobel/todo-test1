package ui

import (
	"regexp"
	"strings"
	"testing"
)

// W10 part 1 — the token layer. tokens.css carries every design value in
// one :root block; style.css consumes them and must stay hex-free; every
// page shell loads the token sheet before its consumer so var() references
// resolve at first paint.

func tokenURL(t *testing.T, suffix string) string {
	t.Helper()
	return uiServer(t, "http://127.0.0.1:1").URL + suffix
}

func TestTokensCSSIsServed(t *testing.T) {
	status, body := getPage(t, tokenURL(t, "/static/tokens.css"))
	if status != 200 || !strings.Contains(body, ":root") {
		t.Fatalf("GET /static/tokens.css = %d, want 200 with a :root block (body head: %.80s)",
			status, body)
	}
}

func TestStyleCSSCarriesNoRawHex(t *testing.T) {
	// The token layer owns every color: the consumer stylesheet may not
	// restate one. (rgba()/var() literals are not hex; raw #rgb/#rrggbb
	// anywhere in style.css is the drift this gate catches.)
	_, style := getPage(t, tokenURL(t, "/static/style.css"))
	if hex := regexp.MustCompile(`#[0-9a-fA-F]{3,8}`).FindString(style); hex != "" {
		t.Errorf("style.css restates a raw color %s outside tokens.css", hex)
	}
}

// styleVarRefRe names every var(--token) reference in a consumer
// stylesheet, tolerant of whitespace and the fallback form var(--x, …).
var styleVarRefRe = regexp.MustCompile(`var\(\s*(--[a-z0-9-]+)`)

func TestStyleCSSVarReferencesResolveToDeclaredTokens(t *testing.T) {
	// The consumer side of the drift gate: every var() reference in
	// style.css must name a token tokens.css declares. The gallery chip
	// gate cannot catch a rename — chips are generated from the same
	// tokens.css parse as the inventory, so a renamed token whose old
	// name still sits in a style.css var() stays chip-green. This gate
	// is the one that sees the consumers: chips track declarations,
	// references are pinned here.
	_, style := getPage(t, tokenURL(t, "/static/style.css"))
	_, tokens := getPage(t, tokenURL(t, "/static/tokens.css"))
	refs := styleVarRefRe.FindAllStringSubmatch(style, -1)
	if len(refs) == 0 {
		t.Fatal("style.css carries no var() references — this pin parses nothing")
	}
	declared := map[string]bool{}
	for _, m := range customPropRe.FindAllStringSubmatch(tokens, -1) {
		declared[m[1]] = true
	}
	for _, m := range refs {
		if !declared[m[1]] {
			t.Errorf("style.css uses var(%s), which tokens.css :root does not declare", m[1])
		}
	}
}

func TestPagesLoadTokensBeforeStyle(t *testing.T) {
	for _, path := range []string{"/", storyGalleryPath} {
		_, page := getPage(t, tokenURL(t, path))
		tokensAt := strings.Index(page, `href="/static/tokens.css"`)
		styleAt := strings.Index(page, `href="/static/style.css"`)
		if tokensAt < 0 || styleAt < 0 {
			t.Errorf("%s links both stylesheets: tokens at %d, style at %d", path, tokensAt, styleAt)
			continue
		}
		if tokensAt > styleAt {
			t.Errorf("%s loads style.css before tokens.css — var() refs would miss the token layer", path)
		}
	}
}
