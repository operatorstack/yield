# Tutorials

Each tutorial starts with an engineering job, not a protocol concept.

1. [Check and review a branch](code-review.md) — combine deterministic checks
   with model judgment.
2. [Pause for approval before publishing](approval.md) — put the human gate
   before the effect.
3. [Resume after a person fixes the environment](environment-repair.md) — wait
   on disk, then recheck.
4. [Bound a debugging loop](bounded-debugging.md) — stop guessing after a
   declared number of attempts.
5. [Run a migration safely](data-migration.md) — dry-run, approve, apply, and
   verify in order.

The repository contains fixture-backed versions of these patterns. Run an
example with `yskill test examples/<name>` before adapting it.
