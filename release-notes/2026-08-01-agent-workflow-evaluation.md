### Test workflow control with a real coding agent

Adds a source-bound evaluation that runs one owned release workflow in two
forms: a long skill, and a short skill backed by Yield code. Six cases cover
every stop point and successful completion. The checked-in result records 12
real agent runs with matching step order, requirement results, agent tasks,
user answers, and final status. No Yield response was rejected.

This evaluates workflow control only. It does not grade the agent's technical
judgment or claim that every skill behaves the same after conversion.

Also fixes `yskill` so documented commands accept the run or skill target before
their flags, such as `yskill resume <run-id> --response response.json`.
