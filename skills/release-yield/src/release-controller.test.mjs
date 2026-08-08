import test from "node:test"
import assert from "node:assert/strict"
import { Blocked, selectNewRun } from "./release-controller.mjs"

const runs = [
  { databaseId: 3, event: "workflow_dispatch", headSha: "abc" },
  { databaseId: 2, event: "push", headSha: "abc" },
  { databaseId: 1, event: "workflow_dispatch", headSha: "old" },
]

test("correlates one new workflow dispatch at the exact source SHA", () => {
  assert.equal(
    selectNewRun(runs, "1,2", { event: "workflow_dispatch", sourceSha: "abc" }).databaseId,
    3,
  )
})

test("returns null until a matching run appears", () => {
  assert.equal(selectNewRun(runs, "1,2,3", { event: "workflow_dispatch", sourceSha: "abc" }), null)
})

test("refuses ambiguous new runs", () => {
  assert.throws(
    () =>
      selectNewRun(
        [...runs, { databaseId: 4, event: "workflow_dispatch", headSha: "abc" }],
        "1,2",
        { event: "workflow_dispatch", sourceSha: "abc" },
      ),
    Blocked,
  )
})
