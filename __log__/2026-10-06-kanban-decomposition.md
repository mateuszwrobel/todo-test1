```json
{"task": "kanban decomposition: sub-workplans, cards, ledger", "status": "done", "date": "2026-10-06"}
```

- Landing kanban decomposition drafts: 4 module sub-workplans (board store, api, ui, server composition) + `dependencies_kanban.md` wave ledger copied verbatim into `workplans/` (`cmp` clean per file), superseded `dependencies.md` gained a one-line pointer to the kanban ledger.
- 46 scenario cards generated mechanically (awk, no hand-typed scenario text) into `workplans/scenarios-kanban/<module>/`: board 14, api 11, ui 13, server 8. Shape mirrors the todo cards exactly. Gates green: per-module scenario counts == card counts, verbatim `cmp` of every card block against its workplan block (46/46), and all 46 unique `<module>/<NN>` refs in the kanban ledger resolve to card files on disk.
