```json
{
  "task": "W5 replay — delete onto main after W3 landed",
  "status": "in-progress",
  "date": "2026-10-04",
  "wave": "W5 (replay of 0cac87a..2074ff0, originally based on 49a3e22)",
  "base": "26cc902 (W3 toggle landed)",
  "cards": "todos/10, todos/11, api/10, api/11, ui/13 + e2e harness"
}
```

Replay of the verified W5 delete wave onto main after W3 landed the same
shared surface. Cherry-picks of the original card commits, oldest→newest,
one commit per card; collisions on the shared files resolved so the final
state is W3 + W5 coexisting: the `/todos/{id}` mount line deduped (W3 added
it already), api port + route appends keep both Change and Delete, the
ui route registry keeps toggle + delete, Makefile + e2e README keep both
waves' append-only sections. Any duplicated not-found error or error-JSON
helper from W5 converges on W3's existing definition — no second source of
truth.
