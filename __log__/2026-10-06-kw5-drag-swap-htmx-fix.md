```json
{ "status": "in-progress" }
```

# KW5 fix — drag swap keeps htmx wiring

Bug (witnessed by e2e/kw5-drag.js scenario-3 cross-check, reported in
__log__/2026-10-06-kw5-e2e-drag.md): the drag drop handler's fetch swap in
ui/render.go assigned the board fragment via raw innerHTML; the vendored
htmx 2.0.6 removed the MutationObserver auto-scan, so htmx never processed
the injected markup. After an accepted drop the re-rendered cards' edit form
submitted natively (full navigation GET /?title=...) and Delete clicks
issued nothing. htmx-mediated swaps (edit/create/delete, the responseError
handler's htmx.swap calls) were immune.

Fix: the fetch swap site calls htmx.process on the swapped region right
after the innerHTML assignment. Audit of the client script: the drop
handler's assignment is the only raw innerHTML swap site.

E2E: kw5 scenario-3 cross-check witness block converted to hard
assertions — post-drop Edit Save must be exactly one PATCH to the card
endpoint with no URL change, and post-drop Delete exactly one DELETE with
the card gone.
