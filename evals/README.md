# Yield evaluations

These evaluations test Yield itself. They do not compare Yield with another
tool or company.

The deterministic suite answers two questions:

1. Can each checked-in example workflow reach its expected final result
   through every supported SDK?
2. Does the runtime behave correctly when a run resumes, replays, blocks, or
   encounters changed code?

## Current coverage

- 10 workflow patterns written by this project.
- 4 SDKs: TypeScript, Python, Go, and Rust.
- 40 end-to-end workflow tests.
- 5 runtime checks: resume and complete, repeat the same saved step, stop when
  behavior changes, block when a rule fails, and require approval for changed
  source.

Run the exact suite and refresh the checked-in result:

```bash
cd evals
npm run eval
```

Check that the published result still matches the current source:

```bash
npm test
```

## What a passing result proves

A passing result proves that the tested Yield revision:

- executes each owned workflow test to `completed`;
- runs command steps rather than asking the model to invent their outputs;
- presents requests in the program-defined order;
- resumes from recorded responses;
- returns to the same saved step during replay;
- stops on changed behavior or failed requirements.

## What it does not prove

This suite does not prove that Yield is better than prose, that an agent's
judgment is correct, or that illustrative commands are production-safe. The
fixed test data supplies agent and human responses so the suite can test only
the code-controlled workflow layer.

`results/latest.json` is a compact, website-safe result. Its source hash is
computed from the CLI, engine, protocol, SDKs, example workflows, fixtures, and
evaluation harness. CI reruns the suite instead of trusting that file alone.

## Coding-agent workflow check

The separate `agent/` suite runs the same owned workflow through a real coding
agent in two forms: a long skill, and a thin skill backed by Yield code. It
checks matching step order, gates, responses, and final status. It does not
score the agent's domain judgment or claim that one form is better.

```bash
npm run eval:agent
npm run test:agent
```
