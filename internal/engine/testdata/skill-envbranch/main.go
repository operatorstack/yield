// Test skill that is DELIBERATELY nondeterministic across executions when
// YIELD_TEST_BRANCH changes: used to prove replay divergence fails loudly.
package main

import (
	"os"

	"github.com/operatorstack/yield/sdk/yield"
)

func main() {
	yield.Main(func(ctx *yield.Context) (yield.Outcome, error) {
		first := ctx.AskUser("first-question", "First?")
		opID := "second-question-a"
		if os.Getenv("YIELD_TEST_BRANCH") == "b" {
			opID = "second-question-b"
		}
		second := ctx.AskUser(opID, "Second?")
		return ctx.Complete(map[string]string{"first": first, "second": second})
	})
}
