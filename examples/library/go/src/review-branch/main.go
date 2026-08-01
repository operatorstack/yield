// Review a branch. Replace the illustrative commands with your project commands.
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
		preflight := ctx.RunCommand("check-branch", "printf 'typecheck and tests passed\\n'", 300)
		ctx.Require(preflight.ExitCode == 0, "the branch passes mechanical checks", preflight)

		raw := ctx.AgentTask(
			"review-diff",
			"Review the branch for correctness, security, data-loss risks, and missing tests. Return pass only when no critical finding remains.",
			map[string]any{"stdout": preflight.Stdout, "stderr": preflight.Stderr},
			json.RawMessage(decisionSchema),
		)
		var decision decision
		if err := json.Unmarshal(raw, &decision); err != nil {
			return yield.Outcome{}, err
		}
		ctx.Require(decision.Status == "pass" && decision.Critical == 0, "the review has no critical findings", decision)

		return ctx.Complete(map[string]any{"workflow": "review-branch", "summary": decision.Summary})
	})
}
