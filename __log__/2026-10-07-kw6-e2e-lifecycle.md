```json
{
  "status": "in-progress",
  "task": "KW6 wave end — e2e lane kw6-lifecycle (restart + migration + in-flight)",
  "wave": "kw6",
  "base": "349e75c8e3683e122960595170d84b455564193a",
  "commit": null
}
```

# KW6 wave end — e2e/kw6-lifecycle.js

New lane re-executing the KW6 wave-end parent scenarios "Board survives
server restart", "Migrate existing todos on first start" and "Repeat
activation while an operation is in flight" against the composed process in
real chromium, plus the browser-bound proofs ui/inflight_test.go deferred to
this lane. kw1–kw5 untouched; Makefile gains the sixth lane line; e2e/README
gains the lane section.
