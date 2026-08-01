// Investigate a failure. Replace the illustrative commands with your project commands.
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
		preflight := ctx.RunCommand("capture-failure", "printf 'failing test captured with recent diff\\n'", 300)
		ctx.Require(preflight.ExitCode == 0, "the failure evidence is captured", preflight)

		raw := ctx.AgentTask(
			"diagnose-cause",
			"Use the failure output and recent change to identify the most likely root cause. Return pass only when the summary states a causal chain.",
			map[string]any{"stdout": preflight.Stdout, "stderr": preflight.Stderr},
			json.RawMessage(decisionSchema),
		)
		var decision decision
		if err := json.Unmarshal(raw, &decision); err != nil {
			return yield.Outcome{}, err
		}
		ctx.Require(decision.Status == "pass" && decision.Critical == 0, "the diagnosis states a supported cause", decision)

		return ctx.Complete(map[string]any{"workflow": "investigate-failure", "summary": decision.Summary})
	})
}
