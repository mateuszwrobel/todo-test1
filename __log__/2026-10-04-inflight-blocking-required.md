```json
{
  "task": "promote in-flight control blocking to workplan scenario",
  "status": "done",
  "date": "2026-10-04"
}
```

User decision: in-flight control blocking is now required behavior — promoted from the journeys-doc UI note into the workplan's acceptance contract, with the journeys doc aligned to match.
- Workplan (`workplans/workplan_todo_application.md`): added acceptance scenario "Repeat activation while an operation is in flight" after "Operation on missing todo"; appended the Decisions line for request-to-response control disabling (rationale: duplicate mutation for an in-flight action never leaves the page; rejected: server-side duplicate suppression — indistinguishable from a legitimate repeat). Modularity Behavior Analysis's "Page interaction" bullet extended with the in-flight blocking clause. API section untouched: client-side behavior, zero contract surface.
- Journeys (`workplans/user-journeys.md`): "Interaction decision — in-flight control blocking" section reworded — now traces to the workplan scenario instead of being a UI decision layered on it; mechanics unchanged. J6 context cites the scenario name where it says the decision closes double-submit; remaining reachable paths (second tab / restart against different data file / direct file edits) kept. Cross-cutting "Control in-flight" row verified — no footnote claimed non-required status, left as-is. All seven open questions verbatim.
- Testability note: the scenario is verifiable with a ui-level test using a delayed-response stub (activate control twice, assert one request and one resulting state transition); the test lands with the ui module's tests.
