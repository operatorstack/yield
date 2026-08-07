import test from "node:test";
import assert from "node:assert/strict";
import { checkReleaseControl } from "./check-release-control.mjs";

test("repository workflows preserve the supervised release boundary", async () => {
  const result = await checkReleaseControl();
  assert.ok(result.workflows >= 5);
  assert.ok(result.externalActionsPinned > 0);
});
