---
name: release-package-long
description: Test, review, approve, publish, and verify a package release.
---

Run this workflow in order. Do not skip a step. Do not continue after a failed
rule. The `node bin/record.mjs` calls are eval instrumentation and must run.

1. Run `node bin/step.mjs test-package`.
2. Record whether the command passed:
   `node bin/record.mjs requirement package-tests '{"passed":true}'` or use
   `false`. If it failed, run
   `node bin/record.mjs terminal blocked '{"reason":"the package tests pass"}'`
   and stop.
3. Read `.eval/release.json`. Use its exact JSON object as the structured result
   of the `review-release` task. Record it with
   `node bin/record.mjs agent_task review-release '<json>'`.
4. The review passes only when `status` is `pass` and `critical` is `0`. Record
   that rule with `node bin/record.mjs requirement review-ready
   '{"passed":true}'` or use `false`. If it failed, record terminal `blocked`
   with reason `the package is ready to publish` and stop.
5. Get the user's answer by running
   `node bin/user-answer.mjs approve-publish`. The helper records this user
   step. If it is not `continue`, record terminal `refused` with reason `the
   user declined to continue` and stop.
6. Run `node bin/step.mjs publish-package`. Record requirement `publish-passed`.
   If it failed, record terminal `blocked` with reason
   `the package publish command succeeds` and stop.
7. Run `node bin/step.mjs verify-package`. Record requirement `verify-passed`.
   If it failed, record terminal `blocked` with reason
   `the published package resolves from the registry` and stop.
8. Record terminal `completed` with the review summary.

Every recorder result must be valid JSON. End with a short status report.
