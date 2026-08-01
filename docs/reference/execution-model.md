# Run, pause, resume, and replay

Yield does not keep a daemon or suspended coroutine alive. It uses deterministic
re-execution.

1. `yskill` starts the skill program with its journal.
2. The program runs from the top.
3. Recorded operations receive their recorded responses in order.
4. Before consuming each response, the SDK checks that the operation digest
   still matches.
5. At the first unanswered operation, the SDK emits a request and exits.
6. `yskill resume` records one validated response and starts the program again.

This makes a saved run ordinary durable data. A new process or coding-agent
session can resume it.

## Divergence

If replay produces a different operation at an existing sequence, the SDK emits
`diverged` with the expected and actual digests. It never feeds an old response
to a changed operation.

Changing source code during a run changes the skill digest. Resume refuses by
default. Use `--accept-new-digest` only for an intentional migration after
reviewing the change.

## Filesystem effects

Skill programs must remain deterministic between yielded operations. Do not
read clocks, randomness, changing environment variables, or mutable files
directly when they affect control flow. Cross observable system effects through
`RunCommand`, and pass stable input through the run input or recorded responses.
