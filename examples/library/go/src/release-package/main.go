// Release a package. Replace the illustrative commands with your project commands.
package main

import (
	"encoding/json"

	"github.com/operatorstack/yield/internal/protocol"
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
		preflight := ctx.RunCommand("test-package", "printf 'package tests passed\\n'", 300)
		ctx.Require(preflight.ExitCode == 0, "the package tests pass", preflight)

		raw := ctx.AgentTask(
			"review-release",
			"Review the pending package release for breaking changes, missing notes, and rollback risk. Return pass only when it is ready to publish.",
			map[string]any{"stdout": preflight.Stdout, "stderr": preflight.Stderr},
			json.RawMessage(decisionSchema),
		)
		var decision decision
		if err := json.Unmarshal(raw, &decision); err != nil {
			return yield.Outcome{}, err
		}
		ctx.Require(decision.Status == "pass" && decision.Critical == 0, "the package is ready to publish", decision)

		approval := ctx.AskUser(
			"approve-publish",
			"Publish this package release?",
			protocol.Option{Value: "continue", Label: "Continue"},
			protocol.Option{Value: "stop", Label: "Stop"},
		)
		if approval != "continue" {
			return yield.Outcome{}, ctx.Refused("the operator declined to continue")
		}

		action := ctx.RunCommand("publish-package", "printf 'package published\\n'", 600)
		ctx.Require(action.ExitCode == 0, "the package publish command succeeds", action)

		verify := ctx.RunCommand("verify-package", "printf 'published package resolved from registry\\n'", 300)
		ctx.Require(verify.ExitCode == 0, "the published package resolves from the registry", verify)

		return ctx.Complete(map[string]any{"workflow": "release-package", "summary": decision.Summary})
	})
}
