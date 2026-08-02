// Conformance program (Go). The SAME program exists in TypeScript, Python,
// and Rust; the harness asserts identical observable protocol behavior.
package main

import (
	"encoding/json"

	"github.com/operatorstack/yield/sdk/yield"
)

func main() {
	yield.Main(func(ctx *yield.Context) (yield.Outcome, error) {
		proceed := ctx.AskUser("q1-proceed", "Proceed with the conformance run?",
			yield.Option{Value: "yes", Label: "Yes"}, yield.Option{Value: "no", Label: "No"})
		if proceed == "no" {
			return yield.Outcome{}, ctx.Refused("operator declined")
		}

		raw := ctx.AgentTask("t2-analyze", "Return {\"n\": <integer>}.",
			map[string]string{"proceed": proceed},
			json.RawMessage(`{"type":"object","required":["n"],"properties":{"n":{"type":"integer"}}}`))
		var t struct {
			N int `json:"n"`
		}
		if err := json.Unmarshal(raw, &t); err != nil {
			return yield.Outcome{}, err
		}
		if t.N == 0 {
			return yield.Outcome{}, ctx.Blocked("n is zero: a true frontier")
		}

		c := ctx.RunCommand("c3-echo", "echo conform-ok", 0)
		ctx.Require(t.N > 0, "n is positive", map[string]int{"n": t.N})
		ctx.Require(c.ExitCode == 0, "the echo command passes", map[string]int{"exit_code": c.ExitCode})

		return ctx.Complete(map[string]int{"n": t.N})
	})
}
