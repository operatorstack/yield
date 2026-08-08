---
name: release-yield
description: Release Yield through its protected GitHub workflows and verify every public registry.
---

Use this workflow only from the `operatorstack/yield` repository.

Run it from the repository root:

    npm exec -- yskill run skills/release-yield

Follow every returned operation exactly. If the user already requested `auto`,
`patch`, `minor`, or `major`, use that value when the first choice appears.

The workflow records one authorization for the exact version and source SHA,
then asks GitHub to enforce the repository's protected environments. It never
publishes packages, creates tags, or handles registry credentials locally.

Choose **Finish after dry run** at the authorization step to complete with the
verified plan and stop before tags, approvals, or publication.
