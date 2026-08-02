// Upgrade a dependency. Replace the illustrative commands with your project commands.
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
		preflight := ctx.RunCommand("baseline-tests", "printf 'baseline tests passed\\n'", 300)
		ctx.Require(preflight.ExitCode == 0, "the baseline tests pass", preflight)

		raw := ctx.AgentTask(
			"review-upgrade",
			"Review the dependency upgrade for API changes, migration work, and rollback risk. Return pass only when the change is bounded.",
			map[string]any{"stdout": preflight.Stdout, "stderr": preflight.Stderr},
			json.RawMessage(decisionSchema),
		)
		var decision decision
		if err := json.Unmarshal(raw, &decision); err != nil {
			return yield.Outcome{}, err
		}
		ctx.Require(decision.Status == "pass" && decision.Critical == 0, "the dependency upgrade has a bounded plan", decision)

		approval := ctx.AskUser(
			"approve-upgrade",
			"Apply the reviewed dependency upgrade?",
			yield.Option{Value: "continue", Label: "Continue"},
			yield.Option{Value: "stop", Label: "Stop"},
		)
		if approval != "continue" {
			return yield.Outcome{}, ctx.Refused("the operator declined to continue")
		}

		action := ctx.RunCommand("apply-upgrade", "printf 'dependency upgraded\\n'", 600)
		ctx.Require(action.ExitCode == 0, "the dependency upgrade command succeeds", action)

		verify := ctx.RunCommand("post-upgrade-tests", "printf 'post-upgrade tests passed\\n'", 300)
		ctx.Require(verify.ExitCode == 0, "the tests pass after the dependency upgrade", verify)

		return ctx.Complete(map[string]any{"workflow": "upgrade-dependency", "summary": decision.Summary})
	})
}
