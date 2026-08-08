// convert-skill: the converter is itself a Yield skill. It turns an
// existing prose SKILL.md into a Yield program in the operator's chosen
// language, and it completes ONLY when the generated skill passes its own
// fixture run under yskill test. A transcription alone cannot complete the
// conversion.
//
// Division of labor: the program owns the pipeline order, the language
// menu, the retry bound, and the evidence gate. The model owns projecting
// each source clause, extracting the flow, and writing the code and guidance.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/operatorstack/yield/sdk/yield"
)

const flowSchema = `{
  "type": "object",
  "required": ["summary", "steps"],
  "properties": {
    "summary": {"type": "string", "minLength": 1},
    "steps": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "required": ["id", "kind", "description"],
        "properties": {
          "id": {"type": "string", "minLength": 1},
          "kind": {"enum": ["ask_user", "agent_task", "run_command", "branch", "require"]},
          "description": {"type": "string", "minLength": 1}
        }
      }
    }
  }
}`

const projectionSchema = `{
  "type": "object",
  "required": ["clauses", "ready", "unresolved"],
  "additionalProperties": false,
  "properties": {
    "clauses": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "required": ["source_clause", "disposition", "destinations", "reason"],
        "additionalProperties": false,
        "properties": {
          "source_clause": {"type": "string", "minLength": 1},
          "disposition": {"enum": ["control", "guidance", "both", "excluded"]},
          "destinations": {
            "type": "array",
            "items": {
              "type": "object",
              "required": ["kind", "target"],
              "additionalProperties": false,
              "properties": {
                "kind": {"enum": ["code", "skill", "agent_task"]},
                "target": {"type": "string", "minLength": 1}
              }
            }
          },
          "reason": {"type": "string"}
        }
      }
    },
    "ready": {"type": "boolean"},
    "unresolved": {"type": "array", "items": {"type": "string", "minLength": 1}}
  }
}`

const filesSchema = `{
  "type": "object",
  "required": ["files"],
  "properties": {
    "files": {"type": "array", "minItems": 2, "items": {"type": "string", "minLength": 1}}
  }
}`

// shellQuote single-quotes a value for sh -c interpolation.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func main() {
	yield.Main(func(ctx *yield.Context) (yield.Outcome, error) {
		source := ctx.AskUser("source-path", "Path to the prose skill directory (contains SKILL.md)?")

		prose := ctx.RunCommand("read-prose", fmt.Sprintf("cat %s/SKILL.md", shellQuote(source)), 60)
		ctx.Require(prose.ExitCode == 0, "the source SKILL.md is readable", map[string]int{"exit_code": prose.ExitCode})

		lang := ctx.AskUser("pick-language", "Target language for the generated program?",
			yield.Option{Value: "go", Label: "Go"},
			yield.Option{Value: "typescript", Label: "TypeScript"},
			yield.Option{Value: "python", Label: "Python"},
			yield.Option{Value: "rust", Label: "Rust"})

		projectionRaw := ctx.AgentTask("project-semantics",
			"Map every source clause exactly once using Pi(S)={(c,d,T,r)|c in clauses(S)}. "+
				"YAML frontmatter is metadata, not a clause. Each top-level bullet is exactly one clause, including a compound sentence. For prose without bullets, treat each paragraph as one clause. "+
				"Use disposition control, guidance, both, or excluded. Control needs a reachable code destination. "+
				"Guidance needs a reachable skill or agent_task destination. Both needs both kinds. "+
				"Excluded needs no destination and a non-empty reason. Name concrete destinations that the writer can create. "+
				"Use the target language's canonical entrypoint for code: main.ts, main.py, main.go, or src/main.rs. "+
				"Report uncertainty in unresolved and set ready false. Do not write files.",
			map[string]string{"skill_md": prose.Stdout, "language": lang},
			json.RawMessage(projectionSchema))
		var projection struct {
			Ready      bool     `json:"ready"`
			Unresolved []string `json:"unresolved"`
		}
		if err := json.Unmarshal(projectionRaw, &projection); err != nil {
			return yield.Outcome{}, err
		}
		if !projection.Ready || len(projection.Unresolved) > 0 {
			return yield.Outcome{}, ctx.Blocked("the semantic projection has unresolved source clauses")
		}

		flowRaw := ctx.AgentTask("extract-flow",
			"Read the prose skill and its semantic projection below. Extract its implicit control flow as ordered steps: "+
				"questions to the user (ask_user), model judgment (agent_task), commands (run_command), "+
				"branches, and verification points (require). Preserve the skill's intent; do not invent steps.",
			map[string]any{"skill_md": prose.Stdout, "projection": json.RawMessage(projectionRaw)},
			json.RawMessage(flowSchema))

		dest := ctx.AskUser("dest-path", "Directory to write the converted skill into?")

		written := ctx.AgentTask("write-skill",
			"Write the converted Yield skill into the destination directory: the program "+
				"(main.go / main.ts / main.py / src/main.rs per the chosen language, using that language's SDK "+
				"from this repository), a thin SKILL.md (keep useful judgment, examples, tone, and tool advice; delegate sequencing to "+
				"`yskill run .`), a skill.json runner manifest, and fixtures/responses.json with a "+
				"happy-path scripted response for every ask_user and agent_task operation. Follow every semantic disposition. "+
				"Thin does not mean deleting useful guidance. "+
				"Return {\"files\": [paths you actually wrote]}.",
			map[string]any{"source": prose.Stdout, "projection": json.RawMessage(projectionRaw), "flow": json.RawMessage(flowRaw), "language": lang, "destination": dest},
			json.RawMessage(filesSchema))

		// The evidence gate with a bounded repair loop: at most two repair
		// attempts, then an honest blocked terminal. The generated skill
		// must run under the REAL supervisor; ${YSKILL:-yskill} lets
		// harnesses point at a specific binary.
		verifyCmd := fmt.Sprintf(`"${YSKILL:-yskill}" test %s`, shellQuote(dest))
		test := ctx.RunCommand("verify-generated", verifyCmd, 600)
		for attempt := 1; test.ExitCode != 0 && attempt <= 2; attempt++ {
			ctx.AgentTask(fmt.Sprintf("fix-generated-%d", attempt),
				"The generated skill failed its fixture run. Fix the files in place and return "+
					"{\"files\": [paths you changed]}.",
				map[string]any{
					"destination": dest,
					"source":      prose.Stdout,
					"projection":  json.RawMessage(projectionRaw),
					"stdout":      test.Stdout,
					"stderr":      test.Stderr,
				},
				json.RawMessage(filesSchema))
			test = ctx.RunCommand(fmt.Sprintf("verify-generated-retry-%d", attempt), verifyCmd, 600)
		}
		if test.ExitCode != 0 {
			return yield.Outcome{}, ctx.Blocked(
				"the generated skill still fails its fixture run after two repair attempts — the flow extraction or the fixtures need human review")
		}

		ctx.Require(test.ExitCode == 0,
			"the generated skill passes its fixture run under yskill test",
			map[string]any{"exit_code": test.ExitCode})

		var files struct {
			Files []string `json:"files"`
		}
		_ = json.Unmarshal(written, &files)
		return ctx.Complete(map[string]any{
			"language":    lang,
			"files":       files.Files,
			"verified":    true,
			"destination": dest,
		})
	})
}
