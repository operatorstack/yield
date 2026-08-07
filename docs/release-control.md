# Release control

Yield treats a stable release as a supervised discrete-event system, not as a
sequence of loosely related CI jobs.

The minimal ZCA slice is one release intent, one immutable source SHA, one
global version, and one controller. The two distribution slices are the
SDK/CLI package and its six platform runtimes. Registry publishers are adapters
that may retry the same version; they cannot allocate a new one.

## Safety policy

The supervisor disables three forbidden transitions:

1. publishing without validated Changeset intent and protected-main authority;
2. publishing different source/version bindings across the seven npm packages;
3. finalizing a GitHub release before the npm publisher receipt matches.

Stable planning is read-only. Tag creation and stable npm publication cross
protected GitHub environments. Runtime
packages publish before the SDK/CLI package. Registry delay leaves the release
draft and retryable at the same version.

The existing OperatorStack private registry is a non-blocking mirror. It
consumes the same immutable tag and can catch up later, but its availability
does not prevent the public release from completing.

Repository checks additionally refuse mutable action references, default write
permissions, workflow PR approval, administrator branch-protection bypass,
mutable `v*` tags, projection sync, direct private tag publication, and
long-lived npm credentials.

## Formal boundary

The canonical models are:

- [`release-supervisor-plant.json`](locus/release-supervisor-plant.json)
- [`release-supervisor-closed-loop.json`](locus/release-supervisor-closed-loop.json)

The formal checks must be rerun whenever these workflows change. Their result
identifiers belong to the exact model revision, so this document does not keep
stale identifiers.

The claim remains **advisory** until the committed workflows run successfully.
Permanent npm failure can stop public release progress. Administrator or npm
account compromise remains outside the controller. The private mirror is
independent and cannot block public release completion.
