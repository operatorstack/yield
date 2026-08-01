// The reference skill: investigate a failure with bounded, evidence-bound
// discipline. The flow — the part prose skills lose under context pressure
// — is code: at least three hypotheses, cheapest-to-disprove first, at
// most three failed attempts, completion requires a causal chain.
// Judgment — forming and assessing hypotheses — stays with the model.
package main

import (
	"encoding/json"
	"fmt"

	"github.com/operatorstack/yield/sdk/yield"
)

type hypothesis struct {
	ID              string `json:"id"`
	Statement       string `json:"statement"`
	DisproveCommand string `json:"disprove_command"`
}

type assessment struct {
	Refuted     bool   `json:"refuted"`
	CausalChain string `json:"causal_chain"`
}

const hypothesesSchema = `{
  "type": "object",
  "required": ["hypotheses"],
  "properties": {
    "hypotheses": {
      "type": "array",
      "minItems": 3,
      "items": {
        "type": "object",
        "required": ["id", "statement", "disprove_command"],
        "properties": {
          "id": {"type": "string"},
          "statement": {"type": "string"},
          "disprove_command": {"type": "string"}
        }
      }
    }
  }
}`

const assessmentSchema = `{
  "type": "object",
  "required": ["refuted"],
  "properties": {
    "refuted": {"type": "boolean"},
    "causal_chain": {"type": "string"}
  }
}`

func main() {
	yield.Main(func(ctx *yield.Context) (yield.Outcome, error) {
		evidence := ctx.AgentTask("collect-evidence",
			"Collect the observable evidence for the failure under investigation: error output, logs, recent changes. Return {\"observations\": [string]}.",
			nil, json.RawMessage(`{"type":"object","required":["observations"],"properties":{"observations":{"type":"array","items":{"type":"string"}}}}`))

		raw := ctx.AgentTask("form-hypotheses",
			"Produce at least three hypotheses explaining the evidence, ordered cheapest-to-disprove first. Each carries a shell command whose failure would disprove it.",
			json.RawMessage(evidence), json.RawMessage(hypothesesSchema))
		var hs struct {
			Hypotheses []hypothesis `json:"hypotheses"`
		}
		if err := json.Unmarshal(raw, &hs); err != nil {
			return yield.Outcome{}, err
		}

		failures := 0
		for _, h := range hs.Hypotheses {
			if failures >= 3 {
				break
			}
			result := ctx.RunCommand("probe-"+h.ID, h.DisproveCommand, 300)
			assessRaw := ctx.AgentTask("assess-"+h.ID,
				fmt.Sprintf("Hypothesis %q: %s. Given the probe result, is it refuted? If it survives, state the causal chain from root cause to observed failure.", h.ID, h.Statement),
				map[string]any{"hypothesis": h, "probe": result},
				json.RawMessage(assessmentSchema))
			var a assessment
			if err := json.Unmarshal(assessRaw, &a); err != nil {
				return yield.Outcome{}, err
			}
			if a.Refuted {
				failures++
				continue
			}
			ctx.Require(a.CausalChain != "", "the surviving hypothesis states a causal chain", a)
			return ctx.Complete(map[string]any{
				"hypothesis":   h,
				"causal_chain": a.CausalChain,
				"probe_exit":   result.ExitCode,
			})
		}
		return yield.Outcome{}, ctx.Blocked(
			fmt.Sprintf("frontier reached: %d hypotheses refuted with %d failed attempts and none surviving — new evidence is needed, not more guessing", len(hs.Hypotheses), failures))
	})
}
