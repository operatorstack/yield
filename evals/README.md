# Yield evaluations

These evaluations test Yield itself. They do not compare Yield with another
tool or company.

The deterministic suite answers two questions:

1. Can each checked-in example skill workflow reach its expected final result
   through every supported SDK?
2. Does the runtime behave correctly when a run resumes, replays, blocks, or
   encounters changed code?

## Current coverage

- 10 skill workflow patterns written by this project.
- 4 SDKs: TypeScript, Python, Go, and Rust.
- 40 end-to-end workflow tests.
- 8 runtime checks: response locking and recovery, declared user choices,
  resume, replay, changed behavior, failed requirements, and changed source.

Run the exact suite and refresh the checked-in result:

```bash
cd evals
npm run eval
```

Check a deliberately refreshed result against the current source:

```bash
npm test
```

## What a passing result proves

A passing result proves that the tested Yield revision:

- executes each owned skill workflow test to `completed`;
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

`results/latest.json` is a compact, website-safe result pinned to the Yield
version that was explicitly evaluated. CI does not rerun evaluations or require
their source hashes to follow ordinary product changes. Run the evaluation
manually when new evidence is needed, then review and commit its receipt.

## Coding-agent workflow check

The separate `agent/` suite runs the same owned workflow through a real coding
agent in two forms: a long skill, and a thin skill backed by Yield code. It
checks matching step order, gates, responses, and final status. It does not
score the agent's domain judgment or claim that one form is better.

```bash
npm run eval:agent
npm run test:agent
```
