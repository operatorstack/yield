---
name: release-yield
description: Release Yield through its protected GitHub workflows and verify every public registry.
---

Use this workflow only from the `operatorstack/yield` repository.

Run it from the repository root:

    npm exec -- yskill run skills/release-yield

Follow every returned operation exactly. If the user already requested `auto`,
`patch`, `minor`, or `major`, use that value when the bump choice appears.

Choose **Dry run only** to resolve and verify the immutable release plan without
publishing. Choose **Prepare release** to continue to a second authorization for
the exact version and source SHA after the protected dry run succeeds.

Minor and major intent must be confirmed before preflight or GitHub workflow
dispatch. Cancelling that confirmation ends the run without GitHub activity.

The workflow records one authorization for the exact version and source SHA,
then asks GitHub to enforce the repository's protected environments. It never
publishes packages, creates tags, or handles registry credentials locally.

Dry-run-only returns the version, tag, source SHA, Changesets, and workflow URL,
then stops before tags, approvals, or publication.
