# Convert an existing prose skill

Use conversion after you have run one ordinary Yield workflow. Conversion is a
separate job from the runtime itself: the converter helps extract and encode a
policy; Yield then executes and verifies the resulting program.

The repository includes [`examples/convert-skill`](../examples/convert-skill/),
a converter that is itself a Yield skill.

## What moves, and what stays

Keep these in the thin `SKILL.md`:

- the goal and when the skill should be used;
- domain context the model needs;
- judgment criteria and useful examples;
- the instruction to start and resume the Yield program.

Move these into the program:

- required order;
- branches and retry limits;
- commands whose output must be observed;
- approval points;
- claims required for completion;
- `Blocked` and `Refused` outcomes.

## Run the converter

From a checkout of the public repository:

```bash
go build -o /tmp/yskill ./cmd/yskill
cd examples/convert-skill
YSKILL=/tmp/yskill /tmp/yskill run .
```

The workflow asks for:

1. the source directory containing `SKILL.md`;
2. a target language;
3. a destination directory.

The coding agent extracts the implicit flow and writes the generated files. The
converter then runs the generated skill's own fixtures under `yskill test`. It
allows two bounded repair attempts and returns `Blocked` if the fixture run
still fails.

## What “verified” means here

The converter verifies that the generated program executes its declared fixture
path. It does **not** prove that the extracted policy is behaviorally equivalent
to every reading of the original prose.

Review the conversion as a policy change:

- map every load-bearing source instruction to code, retained model judgment,
  or an explicit exclusion;
- add a positive fixture and at least one negative or refusal path;
- test bypass attempts and failure states;
- replay a completed run to check determinism;
- keep performance or token-reduction claims separate from runtime correctness.

The converter cannot report success before the generated fixture run passes.
