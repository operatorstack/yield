# `Require`: make the finish rule executable

Use `Require` when the workflow must not continue unless a claim is true.

```ts
const dryRun = ctx.runCommand("dry-run", "npm run migrate -- --dry-run", 300);
ctx.require(dryRun.exit_code === 0, "the migration dry run succeeds", dryRun);
```

A failed requirement records `requirement_failed` and ends the program at that
point. Later commands are structurally unreachable.

Pass the value supporting the claim as evidence. Yield stores its digest with
the requirement:

```ts
ctx.require(review.critical === 0, "no critical findings remain", review);
```

## What `Require` does not do

`Require` checks the Boolean expression you wrote. It cannot make a model result
true, and it cannot prove that an omitted check was unnecessary. Prefer facts
from `RunCommand` for mechanical checks and explicit human answers for approval.
