// Test skill: one of each primitive, evidence-bound completion.
package main

import (
	"encoding/json"

	"github.com/operatorstack/yield/sdk/yield"
)

func main() {
	yield.Main(func(ctx *yield.Context) (yield.Outcome, error) {
		scope := ctx.AskUser("confirm-scope", "May this change modify the public API?")

		summary := ctx.AgentTask("summarize", "Summarize the repository.", map[string]string{"scope": scope},
			json.RawMessage(`{"type":"object","required":["summary"],"properties":{"summary":{"type":"string"}}}`))

		tests := ctx.RunCommand("run-tests", "echo test-ok", 30)
		ctx.Require(tests.ExitCode == 0, "the test command passes", tests)

		var s struct {
			Summary string `json:"summary"`
		}
		_ = json.Unmarshal(summary, &s)
		return ctx.Complete(map[string]string{"scope": scope, "summary": s.Summary})
	})
}
