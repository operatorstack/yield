# Tutorial: run a migration safely

A migration is a useful example because ordering matters:

```text
dry-run → inspect plan → ask for approval → apply → verify → complete
```

The Rust example keeps every effect behind that order. If the dry run fails,
approval is never requested. If approval is declined, apply is never executed.
If verification fails, the run cannot complete successfully.

Use your own commands for the dry run, apply, and verification. Yield provides
the execution and control primitives; it does not provide database migration
logic.

Run the fixture-backed example:

```bash
yskill test examples/data-migration
```

Source: [`examples/data-migration/src/main.rs`](../../examples/data-migration/src/main.rs).
