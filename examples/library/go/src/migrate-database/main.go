// Run a database migration. Replace the illustrative commands with your project commands.
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
		preflight := ctx.RunCommand("dry-run-migration", "printf 'dry run: add users_email_idx concurrently\\n'", 300)
		ctx.Require(preflight.ExitCode == 0, "the migration dry-run succeeds", preflight)

		raw := ctx.AgentTask(
			"review-migration",
			"Review the migration plan for lock risk, irreversible work, and rollback. Return pass only when the plan is safe to apply.",
			map[string]any{"stdout": preflight.Stdout, "stderr": preflight.Stderr},
			json.RawMessage(decisionSchema),
		)
		var decision decision
		if err := json.Unmarshal(raw, &decision); err != nil {
			return yield.Outcome{}, err
		}
		ctx.Require(decision.Status == "pass" && decision.Critical == 0, "the migration plan has acceptable risk", decision)

		approval := ctx.AskUser(
			"approve-migration",
			"Apply the reviewed database migration?",
			yield.Option{Value: "continue", Label: "Continue"},
			yield.Option{Value: "stop", Label: "Stop"},
		)
		if approval != "continue" {
			return yield.Outcome{}, ctx.Refused("the operator declined to continue")
		}

		action := ctx.RunCommand("apply-migration", "printf 'migration applied\\n'", 600)
		ctx.Require(action.ExitCode == 0, "the migration applies cleanly", action)

		verify := ctx.RunCommand("verify-migration", "printf 'migration verification passed\\n'", 300)
		ctx.Require(verify.ExitCode == 0, "the migrated database passes verification", verify)

		return ctx.Complete(map[string]any{"workflow": "migrate-database", "summary": decision.Summary})
	})
}
