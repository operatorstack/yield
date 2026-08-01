// Triage an issue. Replace the illustrative commands with your project commands.
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
		preflight := ctx.RunCommand("read-issue", "printf 'issue: intermittent timeout after retry change\\n'", 300)
		ctx.Require(preflight.ExitCode == 0, "the issue report is available", preflight)

		raw := ctx.AgentTask(
			"classify-issue",
			"Classify severity, identify missing evidence, and propose exactly one next action. Return pass only when the summary is actionable.",
			map[string]any{"stdout": preflight.Stdout, "stderr": preflight.Stderr},
			json.RawMessage(decisionSchema),
		)
		var decision decision
		if err := json.Unmarshal(raw, &decision); err != nil {
			return yield.Outcome{}, err
		}
		ctx.Require(decision.Status == "pass" && decision.Critical == 0, "the issue has one actionable next step", decision)

		return ctx.Complete(map[string]any{"workflow": "triage-issue", "summary": decision.Summary})
	})
}
