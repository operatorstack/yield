// Repair a CI failure. Replace the illustrative commands with your project commands.
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
		preflight := ctx.RunCommand("capture-ci-log", "printf 'ci log: test shard 2 failed after cache restore\\n'", 300)
		ctx.Require(preflight.ExitCode == 0, "the failing CI evidence is captured", preflight)

		raw := ctx.AgentTask(
			"plan-ci-repair",
			"Diagnose the CI failure and describe the smallest supported repair. Return pass only when the repair is tied to the observed log.",
			map[string]any{"stdout": preflight.Stdout, "stderr": preflight.Stderr},
			json.RawMessage(decisionSchema),
		)
		var decision decision
		if err := json.Unmarshal(raw, &decision); err != nil {
			return yield.Outcome{}, err
		}
		ctx.Require(decision.Status == "pass" && decision.Critical == 0, "the CI repair is supported by the failure evidence", decision)

		action := ctx.RunCommand("apply-ci-repair", "printf 'ci repair applied\\n'", 600)
		ctx.Require(action.ExitCode == 0, "the CI repair command succeeds", action)

		verify := ctx.RunCommand("rerun-ci-check", "printf 'failing CI check now passes\\n'", 300)
		ctx.Require(verify.ExitCode == 0, "the previously failing CI check passes", verify)

		return ctx.Complete(map[string]any{"workflow": "repair-ci", "summary": decision.Summary})
	})
}
