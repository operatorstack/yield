# Test workflow effects

`yskill test` runs commands for real. Fixture files provide only the agent and
person responses that the workflow would normally wait for.

Use `fixtures/responses.json` for saved answers. Add `fixtures/test.json` when
a response must also create a deterministic test-only effect:

```json
{
  "version": 1,
  "setup": [["node", "fixtures/setup.mjs"]],
  "after_response": {
    "approve": [["node", "fixtures/apply-approval.mjs"]]
  },
  "teardown": [["node", "fixtures/teardown.mjs"]]
}
```

The runtime passes each `after_response` command that response as JSON on
standard input. Commands use argument arrays and never a shell. `setup` runs
first. `teardown` runs after success or failure. Every hook receives
`YIELD_FIXTURE=1` so test behavior stays separate from live runs.

Keep hooks small and repeatable. They should prepare or clean fixture state,
not replace the workflow behavior being tested.

For Rust, a fixture helper may add another binary under `src/bin/`. New
workflows name the primary workflow binary in `skill.json`, so Cargo still
runs the workflow without asking you to choose a binary.
