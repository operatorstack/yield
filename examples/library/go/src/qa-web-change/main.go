// QA a web change. Replace the illustrative commands with your project commands.
package main

import (
	"encoding/json"

	"github.com/operatorstack/yield/sdk/yield"
)

type decision struct {
	Status   string `json:"status"`
	Critical int    `json:"critical"`
	Summary  string `json:"summary"`
}

const decisionSchema = `{"type":"object","required":["status","critical","summary"],"properties":{"status":{"enum":["pass","needs_work"]},"critical":{"type":"integer","minimum":0},"summary":{"type":"string","minLength":1}}}`

func main() {
	yield.Main(func(ctx *yield.Context) (yield.Outcome, error) {
		preflight := ctx.RunCommand("build-web", "printf 'build passed; changed routes: / and /settings\\n'", 300)
		ctx.Require(preflight.ExitCode == 0, "the web application builds", preflight)

		raw := ctx.AgentTask(
			"test-changed-routes",
			"Test the changed routes at desktop and mobile sizes, including keyboard navigation and form errors. Return pass only when no blocking regression remains.",
			map[string]any{"stdout": preflight.Stdout, "stderr": preflight.Stderr},
			json.RawMessage(decisionSchema),
		)
		var decision decision
		if err := json.Unmarshal(raw, &decision); err != nil {
			return yield.Outcome{}, err
		}
		ctx.Require(decision.Status == "pass" && decision.Critical == 0, "the changed routes have no blocking regression", decision)

		return ctx.Complete(map[string]any{"workflow": "qa-web-change", "summary": decision.Summary})
	})
}
