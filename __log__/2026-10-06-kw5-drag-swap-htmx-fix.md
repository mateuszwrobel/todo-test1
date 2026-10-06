```json
{ "status": "done" }
```

# KW5 fix — drag swap keeps htmx wiring

Bug (witnessed by e2e/kw5-drag.js scenario-3 cross-check, reported in
__log__/2026-10-06-kw5-e2e-drag.md): the drag drop handler's fetch swap in
ui/render.go assigned the board fragment via raw innerHTML; the vendored
htmx 2.0.6 removed the MutationObserver auto-scan, so htmx never processed
the injected markup. After an accepted drop the re-rendered cards' edit form
submitted natively (full navigation GET /?title=..., edit discarded) and
Delete clicks issued nothing — until the next full render. htmx-mediated
swaps (edit/create/delete requests, the responseError handler's htmx.swap
calls) were immune because htmx processes what it swaps itself.

Fix: the fetch swap site now calls htmx.process on the swapped region right
after the innerHTML assignment. Chosen over routing the fetch answer
through htmx.swap/htmx.ajax: it keeps fetch as the page's single per-drop
request site (the one-request rule the lane pins) and is the minimal change
at exactly the site that bypassed htmx — the kw5 lane's probe had already
proven the identical markup PATCHes/DELETEs correctly once processed.
Audit of the client script: the drop handler's assignment is the page's
only raw innerHTML swap site — every other swap goes through htmx itself
(htmx.swap ×2 in the responseError handler, hx-swap="innerHTML" attribute
swaps in the create/edit/delete controls).

E2E: kw5 scenario-3 cross-check witness block converted to hard
assertions — after a fetch-swapped drop, an edit Save is exactly one PATCH
to the card endpoint with no URL change (a native submit would navigate)
and the title updated; then a re-order drop re-injects the markup and a
Delete on it is exactly one DELETE with the card gone. The delete leg's
gesture lands mid-viewport (an in-place re-order drop) with a
detached-node wait proving the swap landed before the Delete click — the
empty-Done-placeholder geometry auto-scrolled the taller post-edit page
mid-drag and released outside every column. The witnessed product-bug
prose in e2e/README.md is replaced by the pinned-behavior note.

Verification: go test -count=1 ./... green; full make e2e green — all five
lanes, kw5 now proving the wired post-drop path; gofmt/vet/archspec verify
green; negative check (htmx.process commented out) turns the cross-check
red with zero PATCHes — the pin is load-bearing.

