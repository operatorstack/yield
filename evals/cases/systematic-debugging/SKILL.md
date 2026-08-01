---
name: systematic-debugging
description: Diagnose a reproducible failure from evidence before proposing a fix.
---

# Systematic debugging

Use the supplied failure output and code context to identify one falsifiable
root-cause hypothesis. Separate observations from inference. Name the mechanism,
the evidence that supports it, and the smallest experiment that could disprove
it. Do not recommend a fix until the hypothesis survives that experiment.

When reviewing a candidate fix, check that it addresses the mechanism rather
than hiding the symptom and that the original failure now passes without a new
regression.

Yield owns phase order, experiment bounds, commands, saved evidence, and exit
conditions.
