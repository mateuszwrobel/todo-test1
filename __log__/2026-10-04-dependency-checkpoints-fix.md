```json
{
  "task": "replace fake-parity lane rule with integration checkpoints",
  "status": "done",
  "date": "2026-10-04"
}
```

The old lane rule required an in-flight card to also run the real dependency once it landed — unworkable mid-flight: a lane cannot re-verify against code that is still merging in another worktree, and a card cannot claim an integration result it never observed. Replaced with explicit checkpoints CP1–CP3 that fire when the depended-on code exists: fakes retire at the checkpoint, drift surfaces there per operation, not as a moving completion gate under an active card. Parent scenarios reframed accordingly — they are acceptance re-execution of the already-planned BDD against the composed process after server/01, not a separate work lane; lane-E is just the label for that Playwright run. Card-done gate stays observable: contract fakes green + archspec findings naming only not-yet-landed packages.
