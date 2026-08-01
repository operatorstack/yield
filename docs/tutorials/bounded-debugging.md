# Tutorial: bound a debugging loop

“Keep investigating until you find it” has no finish rule. A useful debugging
workflow should say when to stop guessing and ask for new evidence.

The Go investigation example makes the boundary explicit:

1. ask the model for observable evidence;
2. form at least three hypotheses, cheapest to disprove first;
3. run the proposed probe for each hypothesis;
4. allow at most three failed attempts;
5. complete only with a causal chain, otherwise return `Blocked`.

```go
failures := 0
for _, h := range hypotheses {
    if failures >= 3 {
        break
    }
    probe := ctx.RunCommand("probe-"+h.ID, h.DisproveCommand, 300)
    assessment := assess(ctx, h, probe)
    if assessment.Refuted {
        failures++
        continue
    }
    ctx.Require(assessment.CausalChain != "", "a causal chain exists", assessment)
    return ctx.Complete(assessment)
}
return yield.Outcome{}, ctx.Blocked("new evidence is required")
```

The model chooses and assesses hypotheses. Code owns the attempt bound and the
completion rule.

Source: [`examples/investigate/main.go`](../../examples/investigate/main.go).
