# Semantic-disposition conversion evaluation

This evaluation checks one small skill conversion. It uses this operator:

\[
C = clauses(S)
\]

\[
\Pi(S)=\{(c,d,T,r)\mid c\in C\}
\]

Each source clause gets one disposition. `T` lists its destinations. `r` gives
the reason for an exclusion.

## How the projection works

1. Split the source `SKILL.md` into clauses.
2. Assign one disposition to each clause.
3. Map each retained clause to a reachable code or model-facing destination.
4. Give a reason for each excluded clause.
5. Pass the source and projection into flow extraction, writing, and repair.

| Source clause | Disposition | Required destination |
|---|---|---|
| Run tests and stop on failure. | `control` | Code |
| Prefer changed-code evidence. | `guidance` | `SKILL.md` or `agent_task` |
| Ask for approval and explain why. | `both` | Code and model-facing guidance |
| Yarn is only release history. | `excluded` | No destination; give a reason |

Run the paid evaluation:

    npm run eval:conversion

The command starts exactly two fresh Codex sessions. Both use `gpt-5.6-sol`
with medium reasoning. The first session runs the real bootstrapped builder.
The second session judges the generated result and a control-only negative
candidate. Raw transcripts stay under ignored `evals/runs/`.

Run deterministic checks:

    npm run test:conversion -- --force

This is advisory evidence for one four-clause fixture. It does not cover other
skills or conversions.
