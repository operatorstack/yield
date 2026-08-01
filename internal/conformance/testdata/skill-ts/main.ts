// Conformance program (TypeScript). The SAME program exists in Go, Python,
// and Rust; the harness asserts identical observable protocol behavior.
import { defineSkill } from "../../../../sdk/typescript/src/index.ts";

defineSkill((ctx) => {
  const proceed = ctx.askUser("q1-proceed", "Proceed with the conformance run?", [
    { value: "yes", label: "Yes" },
    { value: "no", label: "No" },
  ]);
  if (proceed === "no") ctx.refused("operator declined");

  const t = ctx.agentTask<{ n: number }>(
    "t2-analyze",
    'Return {"n": <integer>}.',
    { proceed },
    { type: "object", required: ["n"], properties: { n: { type: "integer" } } },
  );
  if (t.n === 0) ctx.blocked("n is zero: a true frontier");

  const c = ctx.runCommand("c3-echo", "echo conform-ok");
  ctx.require(t.n > 0, "n is positive", { n: t.n });
  ctx.require(c.exit_code === 0, "the echo command passes", { exit_code: c.exit_code });

  return { n: t.n };
});
