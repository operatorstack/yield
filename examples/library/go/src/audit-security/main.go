// Audit a change for security. Replace the illustrative commands with your project commands.
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
		preflight := ctx.RunCommand("run-security-checks", "printf 'dependency and secret scans completed\\n'", 300)
		ctx.Require(preflight.ExitCode == 0, "the mechanical security checks complete", preflight)

		raw := ctx.AgentTask(
			"review-trust-boundaries",
			"Review authentication, authorization, input handling, secrets, and trust-boundary changes. Return pass only when no critical risk remains.",
			map[string]any{"stdout": preflight.Stdout, "stderr": preflight.Stderr},
			json.RawMessage(decisionSchema),
		)
		var decision decision
		if err := json.Unmarshal(raw, &decision); err != nil {
			return yield.Outcome{}, err
		}
		ctx.Require(decision.Status == "pass" && decision.Critical == 0, "the change has no critical security finding", decision)

		return ctx.Complete(map[string]any{"workflow": "audit-security", "summary": decision.Summary})
	})
}
