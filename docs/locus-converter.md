# Locus derivation — the skill converter

`examples/convert-skill` turns an existing prose `SKILL.md` into a Yield
program in the operator's chosen language (Go / TypeScript / Python /
Rust). The converter is itself a Yield skill — the pipeline that makes
skills reliable is the pipeline that converts them.

## Verdicts (models in `docs/locus/`)

| model | operator | verdict |
|---|---|---|
| `convert-verified.json` | control.supervisory-rw | **controllable**, no violations — shipping an unverified conversion is preventable |
| `convert-verified.json` | control.nonblockingness | **nonblocking** — every conversion ends at `SHIPPED_VERIFIED` or an honest `REPORTED_BLOCKED` |
| `convert-verified.json` vs `convert-transcribed.json` | verification.safety-reachability (rival designs, `drv-…`) | **decided** — the executed-verification design satisfies `forbidden-unreachable`; completing on the model's transcription is rejected with the verbatim trace `extract_flow → pick_language → generate_program → complete_untested → SHIPPED_UNVERIFIED` |

The decided comparison is the design's spine: **a conversion that was
never executed is never "done".** `complete_untested` stays in the
alphabet with no transition — the program's `Require(test.exit_code == 0)`
is the refusing mechanism, and the two-attempt repair loop ends in an
honest `Blocked`, keeping the pipeline nonblocking.

## Division of labor

| code owns | model owns |
|---|---|
| pipeline order (read → extract → choose → generate → verify) | reading the prose and extracting the implicit flow |
| the language menu (`ask_user`, closed set) | writing the program, thin SKILL.md, runner manifest, fixtures |
| the repair bound (≤ 2 attempts) | repairing a failing generation |
| the evidence gate (`yskill test` exit code, observed by the supervisor) | — |

## Verified end-to-end without a model

`fixtures/responses.json` scripts a conversion whose destination is an
existing valid skill, so `yskill test examples/convert-skill` exercises
the full machinery — including the **nested** `yskill test` of the
"generated" skill (`${YSKILL:-yskill}` lets harnesses pin the binary).
What the scripted run cannot exercise is the model actually writing good
code; that is exactly the part the evidence gate exists to check at live
time.
