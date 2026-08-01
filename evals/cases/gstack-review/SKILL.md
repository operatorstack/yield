---
name: review
description: Review the current branch for important defects before shipping.
---

# Review

Inspect the supplied diff and evidence. Look for failures tests may miss:
trust-boundary mistakes, unsafe side effects, incomplete error handling,
concurrency problems, and behavior that contradicts the surrounding code.

Return structured findings with `severity`, `confidence`, `category`, `file`,
`line`, `problem`, and `fix`. Use only `critical` or `informational`. Count only
defects caused by this branch. Prefer a small number of specific findings over
general advice.

Yield owns repository checks, ordering, policy validation, saved state, and
completion. Do not reproduce those rules in prose.
