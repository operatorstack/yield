# Outcomes: complete, blocked, or refused

Every run should end honestly.

## Complete

Return the useful result when the workflow has satisfied its requirements:

```ts
return { published: true, version }
```

TypeScript and Python complete by returning. Go uses `ctx.Complete(value)` and
Rust returns `Ok(value)`.

## Blocked

Use `Blocked` when the workflow cannot continue without new information or a
real-world change:

```ts
ctx.blocked("three probes failed; new evidence is required")
```

Blocked is not an error to hide. It records the frontier so another session can
understand why the work stopped.

## Refused

Use `Refused` when the workflow deliberately declines to perform an action:

```ts
if (approval !== "yes") ctx.refused("release not approved")
```

Refused is useful for rejected approvals, unsafe requests, or policy choices.
It is distinct from a missing dependency or exhausted investigation.
