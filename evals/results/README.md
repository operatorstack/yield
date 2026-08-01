# Published summaries

`latest.json` is intentionally small enough for documentation and websites.
It may point to a large raw artifact, but it never embeds transcripts or
temporary repositories.

A promoted behavior result must contain:

- the exact Yield commit and eval case-set digest;
- model and harness versions;
- fixture, repeat, and arm counts;
- aggregate metrics with uncertainty intervals;
- an immutable artifact URI and SHA-256;
- explicit exclusions and untested boundaries.

The current result is marked early and its raw artifact is unpublished. It is
useful for building the reporting surface, not as a final launch claim.
