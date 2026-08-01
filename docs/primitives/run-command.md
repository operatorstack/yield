# `RunCommand`: record what actually ran

Use `RunCommand` when the workflow depends on a command's real exit code or
output. Typical uses are tests, type checks, builds, dry runs, deploy commands,
and verification probes.

```ts
const test = ctx.runCommand("test", "npm test", 300);
ctx.require(test.exit_code === 0, "the tests pass", test);
```

The arguments are:

1. a stable operation ID;
2. the shell command;
3. an optional timeout in seconds.

`yskill` executes the command. The coding agent does not transcribe the result.
The saved response contains `exit_code`, `stdout`, `stderr`, and whether the
command timed out.

## Use the result as evidence

Pass the command result to `Require` when a later completion depends on it:

```ts
const build = ctx.runCommand("build", "npm run build", 600);
ctx.require(build.exit_code === 0, "the production build succeeds", build);
```

This binds the claim to the recorded command result. A failed requirement ends
the run before later operations can execute.

## Common mistake

Do not use `AgentTask` to ask the model whether a command passed. Let Yield run
the command, then use the model only if its output needs interpretation.
