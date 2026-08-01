// convert-skill: the converter is itself a Yield skill. It turns an
// existing prose SKILL.md into a Yield program in the operator's chosen
// language, and it completes ONLY when the generated skill passes its own
// fixture run under yskill test — the Locus-decided design
// (docs/locus/convert-verified.json vs convert-transcribed.json: shipping
// on the model's transcription is rejected with a violating trace).
//
// Division of labor: the program owns the pipeline order, the language
// menu, the retry bound, and the evidence gate. The model owns reading
// the prose, extracting the flow, and writing the code.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/operatorstack/yield/internal/protocol"
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

		flowRaw := ctx.AgentTask("extract-flow",
			"Read the prose skill below and extract its implicit control flow as ordered steps: "+
				"questions to the user (ask_user), model judgment (agent_task), commands (run_command), "+
				"branches, and verification points (require). Preserve the skill's intent; do not invent steps.",
			map[string]string{"skill_md": prose.Stdout},
			json.RawMessage(flowSchema))

		lang := ctx.AskUser("pick-language", "Target language for the generated program?",
			protocol.Option{Value: "go", Label: "Go"},
			protocol.Option{Value: "typescript", Label: "TypeScript"},
			protocol.Option{Value: "python", Label: "Python"},
			protocol.Option{Value: "rust", Label: "Rust"})

		dest := ctx.AskUser("dest-path", "Directory to write the converted skill into?")

		written := ctx.AgentTask("write-skill",
			"Write the converted Yield skill into the destination directory: the program "+
				"(main.go / main.ts / main.py / src/main.rs per the chosen language, using that language's SDK "+
				"from this repository), a THIN SKILL.md (keep the original prose voice, delegate sequencing to "+
				"`yskill run .`), a skill.json runner manifest (omit for Go), and fixtures/responses.json with a "+
				"happy-path scripted response for every ask_user and agent_task operation. "+
				"Return {\"files\": [paths you actually wrote]}.",
			map[string]any{"flow": json.RawMessage(flowRaw), "language": lang, "destination": dest},
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
