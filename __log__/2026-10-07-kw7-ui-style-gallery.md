```json
{
  "status": "done",
  "task": "ui/13 — Every state renders in the gallery (styling + gallery; visual baselines land in a separate follow-up lane)",
  "card": "workplans/scenarios-kanban/ui/13-every-state-renders-in-the-gallery.md",
  "wave": "kw7",
  "base": "05f60bc"
}
```

# Every state renders in the gallery — board styling pass

Behavior-frozen by construction: the diff touches only the two stylesheets,
the gallery, one e2e landing-point retarget, and this entry. No template,
handler, or shell-script line changed — every pinned selector/attr
(`card--done`, `column__empty`, `#board-area`, `#missing-card`,
`error-text`, `data-column`, `draggable`, `data-card`, `.card__edit`,
`.card__delete`, `.save`, `.cancel`, `create-*`) survives because its
producing markup was never edited.

What changed and why:

- **Board layout to k1–k7.** The page shell places its three blocks with
  grid (heading left + create band right on the header line, board
  full-width beneath) — placement only, DOM order unchanged, so the
  swap-target wiring stays byte-identical. `.board` becomes a three-column
  equal-gap grid; columns render as light rounded wells with card rows on
  thin dividers; the create rejection outlines the input red above its
  stated reason (k2). The retired todo-era rules (`#todo-list`, `.row`,
  checkbox done-toggle) left with this pass — the workplan's "the style
  layer survives and gains board components" line.
- **Hover-gated controls paint via opacity, not visibility.** The elements
  stay in the DOM always; only the paint is hover/focus-gated (k6/k7).
  Opacity rather than the mockups' implied visibility-gating: the e2e lanes
  click Edit/Delete directly without a pre-hover, and Playwright
  actionability refuses `visibility:hidden` while accepting a transparent
  element with an intact hit box. Deviation noted here deliberately.
- **Done treatment is pure CSS.** A small green circular check on the
  title's `::before` plus muted title color — column membership remains
  the done state (workplan decision), the check is paint on it, and the
  no-done-control pins (`assertNoDoneControl`, no-checkbox) stay honored
  because no markup or control was added.
- **Edit band flows in place** (k6): input stretches, Save/Cancel follow,
  the row's Edit/Delete step out while open. `min-width:0` on the band
  form is load-bearing — without it the band holds its min-content width
  and overflows the card, moving Save off its own hit box (found by kw3
  under the new layout; verified live).
- **Drag chrome consolidated, geometry untouched.** Same shell-toggled
  classes, same token-referenced colors; the indicator stays 2px + 6px
  margins (the ~14px mid-drag reflow the drag lanes' drop math assumes),
  the column gutter stays outside every panel, and the source slot keeps a
  full dashed ring even as a list's last row.
- **Token layer gains three board colors** (`--color-bg-column`,
  `--color-done`, `--color-text-done`) with mockup color_spec values, named
  per convention so style.css stays hex-free (drift-gated). Gallery chips
  auto-track declarations, so no test churn was needed for them.
- **Gallery = the whole observable-state inventory**, one page, every
  section a real surface-template rendering against deterministic fixtures:
  populated board / whole-board empty / load failure / create band ready /
  create rejected / edit band open / edit refused at card / stale banner
  over the truth / drag frozen mid-gesture. Mid-interaction states freeze
  the shell-script class hooks (documented static-stand-in precedent: the
  `.is-focus` field example); stale and edit-error reuse the live
  `writeStaleFailure`/`attachEditError` paths. Six board atoms join the
  components block. `stories_test.go` evolves honestly with the expanded
  inventory (section list, per-section contract pins, example ids) — the
  pinned `storyBoard()` fixture stayed byte-identical.
- **e2e retarget (one, harness-side, noted per card):** kw5 scenario 2's
  top-of-column park point. Under side-by-side columns the wrapped titles
  make cards tall enough that the pre-measured fy-0.25 point stays inside
  the card the insertion line shifts, so chromium closed the gesture with
  a dragleave and dropped nothing. The point now parks just above the
  live first card (mirror of scenario 2b's clamp) — same behavior tested,
  drop math unchanged; reasoning is recorded in the lane comment.

Verification: `go build ./...`, `go test -count=1 ./...`, `gofmt`,
`go vet`, `archspec verify --strict` all green; the FULL six-lane
`make e2e` run on this tree exited 0 (KW1–KW6 all scenarios passed,
mid-gesture witnesses intact). The wave-end verification lane reruns the
browser suite independently — this lane's own run was backgrounded by
lifetime necessity and its verdict captured from the completed log.
Visual-snapshot baselines over the `#state-*` anchors remain a separate
follow-up lane per the card.
