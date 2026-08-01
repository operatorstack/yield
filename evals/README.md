# Yield evaluations

This directory contains the public, reviewable part of Yield's evaluation
system: case definitions, pinned source identities, conversion programs,
measurement code, validation rules, and small result summaries.

Raw agent transcripts, temporary repositories, command logs, and model
responses do not belong in Git. A full campaign uploads those files as one
immutable artifact bundle and records its URI and SHA-256 in the published
summary. Until that bundle exists, the summary must say `unpublished`.

## Layout

- `cases/` — pinned public source identities plus the thin skill and Yield
  program used for each conversion.
- `scripts/measure-source.mjs` — reproduces the source-size comparison from
  pinned upstream files.
- `scripts/validate.mjs` — fail-closed validation for cases and summaries.
- `results/latest.json` — small website-safe summary. It is not raw evidence.
- `runs/` — local or CI output; ignored by Git and projected releases.

## Run

```bash
npm install
npm run validate
npm run measure
```

`npm run measure` writes a fresh summary under `runs/`. Publishing that result
requires a separate promotion step that binds the raw artifact digest, the
exact Yield commit, model identity, harness version, and case-set digest.

## Claim boundary

Source-size measurements show how much model-facing text and workflow source
the prototypes contain. They do not prove behavioral equivalence. Behavioral
claims require executable fixtures, held-out oracles, repeated model runs, and
the immutable raw artifact named by the result summary.
