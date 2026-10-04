```json
{"task": "decompose module workplans into single-scenario cards", "status": "done", "date": "2026-10-04"}
```

- `workplans/scenarios/<module>/<NN>-<slug>.md` (47 cards: todos 14, api 11, ui 16, server 6): cards are scenario-level units derived from the module workplans — one card per `### Scenario:` block in file order, scenario text copied verbatim, no added behavior, no test names, no estimates; each card's Done-when adds only the archspec verify --strict gate.
