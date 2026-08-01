### Four languages, one certified contract — and a converter

Yield is now multi-language. The canonical `ir/yield.v1` schemas define
everything that crosses a process boundary, and four SDKs implement the
same Locus-certified execution contract: Go (`sdk/yield`), TypeScript
(`sdk/typescript`, Node ≥ 23.6), Python (`sdk/python`, import
`yieldskill`), and Rust (`sdk/rust`, crate `yieldskill`). Non-Go skills
declare their runner in `skill.json`.

The conformance suite (`internal/conformance`) runs the same program in
all four languages through the real supervisor and asserts identical
observable protocol behavior — including loud divergence on a tampered
journal in every SDK.

New examples: `release-checklist` (TypeScript, human-gated deploy),
`env-doctor` (Python, probe/branch/resume-after-human), `data-migration`
(Rust, dry-run → approve → apply → verify), and `convert-skill` — a
converter, itself a Yield skill, that turns an existing prose `SKILL.md`
into a Yield program in the language you pick and completes only when
the generated skill passes its own fixture run.
