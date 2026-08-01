# Tutorial: check and review a branch

Many review skills mix two different jobs in prose:

- mechanical checks that should really execute;
- judgment that needs a coding agent.

Yield gives each job a clear owner.

```ts
const check = ctx.runCommand("check", "npm run typecheck", 300);
ctx.require(check.exit_code === 0, "typecheck passes", check);

const review = ctx.agentTask<Review>(
  "review",
  "Review the branch for correctness, security, and data-loss risks.",
  undefined,
  reviewSchema,
);

ctx.require(review.critical === 0, "no critical findings remain", review);
return review;
```

## Why this split helps

The agent still reads the diff and finds problems. It cannot skip the type
check, silently turn a failed check into success, or complete while the declared
critical count is non-zero.

Use a schema containing the fields your later code actually reads. Avoid a huge
review schema when the gate only needs a small stable result.

The [quickstart](../quickstart.md) builds this workflow from an empty directory.
