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
