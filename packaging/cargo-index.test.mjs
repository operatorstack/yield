import assert from "node:assert/strict";
import test from "node:test";
import { record } from "./cargo-index.mjs";

test("runtime crate index records have no dependencies", () => {
  assert.deepEqual(record("yieldskill-runtime-linux-amd64", "1.2.3", "a".repeat(64)).deps, []);
});

test("public crate pins all six runtime packages to the same version", () => {
  const result = record("yieldskill", "1.2.3", "b".repeat(64));
  const runtimes = result.deps.filter((dep) => dep.name.startsWith("yieldskill-runtime-"));
  assert.equal(runtimes.length, 6);
  assert.ok(runtimes.every((dep) => dep.req === "=1.2.3"));
  assert.ok(runtimes.every((dep) => dep.registry.includes("get.operatorstack.systems")));
  assert.equal(new Set(runtimes.map((dep) => dep.target)).size, 6);
});
