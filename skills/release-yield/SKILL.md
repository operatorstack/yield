---
name: release-yield
description: Release Yield through its protected GitHub workflows and verify every public registry.
---

Use this workflow only from the `operatorstack/yield` repository.

Run it from the repository root:

    npm exec -- yskill run skills/release-yield

Follow every returned operation exactly. The workflow asks the operator to
choose release mode, scope, version basis, bump, and release-note source.
Choosing **Stop** at any decision ends the run before the next protected action.

Stable releases currently use the **full Yield train**: one version and source
SHA across npm, PyPI, crates.io, Go, the GitHub tag, and release evidence.
Target-specific publication is not available until it has a compatibility
manifest and verification path; the workflow explains this and stops rather
than guessing a partial release.

Choose **Use the Changeset bump** when pending Changesets describe the release.
Choose an explicit patch, minor, or major bump when the operator wants to
release the exact Git history without a Changeset. With no Changesets, the
workflow offers an explicit bump and deterministic Git-history release notes;
it does not treat this as an npm failure. An explicit release must contain a
commit after the base tag and cannot lower a pending Changeset bump.

Choose **Dry run only** to resolve and verify the immutable plan without
publishing. Choose **Prepare release** to continue to a second authorization
for the exact version, source SHA, note source, and target contract after the
protected dry run succeeds. Minor and major choices require confirmation before
preflight or GitHub workflow dispatch.

The workflow records authorization for the exact plan, then asks GitHub to
enforce protected environments. It never publishes packages, creates tags, or
handles registry credentials locally.
