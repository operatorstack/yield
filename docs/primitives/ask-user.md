# `AskUser`: pause for a person

Use `AskUser` when a person must choose, provide missing information, or approve
an irreversible action.

```ts
const answer = ctx.askUser("approve", "Publish this release?", [
  { value: "yes", label: "Publish" },
  { value: "no", label: "Stop" },
]);

if (answer !== "yes") ctx.refused("the user declined publication");
```

The coding agent asks through its normal interface. Yield records the answer
and replays it when the program starts again. The run can wait on disk between
the question and the answer.

Use a closed list of options when only specific values are valid. Use a free
answer when the person needs to provide a path, identifier, or explanation.

## Common mistake

Do not ask for approval after the command has already changed the system. Put
the question before the effect, and use `Refused` when the person says no.
