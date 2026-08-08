# Tutorial: pause for approval before publishing

An approval belongs before the command that changes the system.

```ts
const approval = ctx.askUser("approve", "Publish this release?", [
  { value: "yes", label: "Publish" },
  { value: "no", label: "Stop" },
])

if (approval !== "yes") ctx.refused("the user declined publication")

const publish = ctx.runCommand("publish", "npm run publish", 600)
ctx.require(publish.exit_code === 0, "the publish command succeeds", publish)
return { published: true }
```

Yield does not provide publishing logic. `npm run publish` is your command and
your repository remains responsible for what it does. Yield records the
approval, runs the command, and prevents completion when the exit code fails.

See [`examples/release-checklist`](../../examples/release-checklist/main.ts) for
a fixture-backed TypeScript program that also asks the model to draft release
notes.
