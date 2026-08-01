### Remove stray analysis traces

Deletes `runs/*.jsonl` — scratch traces from the formal-analysis tooling
that were accidentally committed alongside the multi-language release —
and excludes `runs/` and `target/` from the published surface so scratch
state can never ship again. No code changes.
