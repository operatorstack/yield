// Publish an iOS build. Replace the illustrative commands with your project commands.
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
		preflight := ctx.RunCommand("archive-ios", "printf 'iOS archive and tests passed\\n'", 300)
		ctx.Require(preflight.ExitCode == 0, "the iOS archive and tests pass", preflight)

		raw := ctx.AgentTask(
			"review-ios-release",
			"Review the iOS release metadata, versioning, privacy notes, and rollout risk. Return pass only when the build is ready for upload.",
			map[string]any{"stdout": preflight.Stdout, "stderr": preflight.Stderr},
			json.RawMessage(decisionSchema),
		)
		var decision decision
		if err := json.Unmarshal(raw, &decision); err != nil {
			return yield.Outcome{}, err
		}
		ctx.Require(decision.Status == "pass" && decision.Critical == 0, "the iOS build is ready for upload", decision)

		approval := ctx.AskUser(
			"approve-ios-upload",
			"Upload this iOS build to App Store Connect?",
			yield.Option{Value: "continue", Label: "Continue"},
			yield.Option{Value: "stop", Label: "Stop"},
		)
		if approval != "continue" {
			return yield.Outcome{}, ctx.Refused("the operator declined to continue")
		}

		action := ctx.RunCommand("upload-ios", "printf 'iOS build uploaded\\n'", 600)
		ctx.Require(action.ExitCode == 0, "the iOS upload command succeeds", action)

		verify := ctx.RunCommand("verify-ios-processing", "printf 'uploaded build entered processing\\n'", 300)
		ctx.Require(verify.ExitCode == 0, "the uploaded iOS build entered processing", verify)

		return ctx.Complete(map[string]any{"workflow": "publish-ios", "summary": decision.Summary})
	})
}
