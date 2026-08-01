// Test skill whose requirement fails: the run must close blocked, never
// completed.
package main

import (
	"github.com/operatorstack/yield/sdk/yield"
)

func main() {
	yield.Main(func(ctx *yield.Context) (yield.Outcome, error) {
		tests := ctx.RunCommand("run-tests", "exit 3", 30)
		ctx.Require(tests.ExitCode == 0, "the test command passes", tests)
		return ctx.Complete(map[string]string{"unreachable": "yes"})
	})
}
