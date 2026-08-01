# Coding-agent workflow equivalence

This suite checks one Yield product claim with a real coding agent:

> Moving workflow control from a long skill into Yield code preserves the
> tested step order and final result.

It does not score general coding ability. The release data, command outcomes,
and user answers are deliberately simple. The same Codex model runs two owned
representations:

1. a long `SKILL.md` containing every step and branch;
2. a thin `SKILL.md` that follows the same workflow in Yield code.

Six cases cover the important branches: failed tests, failed review rule, user
refusal, failed publish, failed verification, and successful completion.

## Evidence

The long-skill arm records actual command calls and structured step results.
The Yield arm is scored from its append-only run log. The scorer verifies:

- the same ordered step IDs;
- the same final status;
- real command evidence for every command step;
- accepted `agent_task` and `ask_user` responses in the Yield log;
- matching requirement results and terminal events;
- no rejected response envelopes.

Raw Codex transcripts and run logs stay under ignored `evals/runs/`. The compact
result in `results/latest-agent.json` contains counts, normalized traces, model
identity, CLI version, token usage, timings, and a source hash covering the
harness, fixtures, CLI, engine, protocol, and TypeScript SDK.

## Run

Use the current Codex login:

```bash
cd evals
npm run eval:agent
npm run test:agent
```

CI can instead provide `CODEX_API_KEY`. Set `EVAL_AGENT_MODEL` and
`EVAL_AGENT_REASONING` to make a different model configuration explicit.
