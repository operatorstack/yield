# `AgentTask`: keep judgment with the model

Use `AgentTask` for work that needs interpretation: reviewing a diff,
diagnosing a failure, comparing designs, extracting a policy, or proposing a
fix.

```ts
type Diagnosis = { cause: string; confidence: number }

const diagnosis = ctx.agentTask<Diagnosis>(
  "diagnose",
  "Find the most likely cause of this test failure.",
  { stdout: test.stdout, stderr: test.stderr },
  {
    type: "object",
    required: ["cause", "confidence"],
    properties: {
      cause: { type: "string" },
      confidence: { type: "number" },
    },
  },
)
```

The arguments are:

1. a stable operation ID;
2. the instruction;
3. optional structured context;
4. an optional JSON Schema for the response.

The Yield CLI validates the response schema before accepting it. Schema-valid
does not mean true; use `RunCommand`, human approval, or another explicit check
when the workflow needs stronger evidence.

## Common mistake

Do not put retry order, approval rules, or completion policy inside the
instruction. Keep those rules in the surrounding program where they can be
replayed and tested.
