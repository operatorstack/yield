import test from "node:test"
import assert from "node:assert/strict"
import { auditRepositoryControls } from "./audit-repository-controls.mjs"

function controls(overrides = {}) {
  return {
    workflow: { default_workflow_permissions: "read", can_approve_pull_request_reviews: false },
    actions: { enabled: true, sha_pinning_required: true },
    protection: {
      enforce_admins: { enabled: true },
      required_status_checks: {
        strict: true,
        contexts: [
          "Go and agent registration (ubuntu-latest)",
          "Go and agent registration (macos-latest)",
          "Go and agent registration (windows-latest)",
          "Release authority and full validation",
        ],
      },
      allow_force_pushes: { enabled: false },
      allow_deletions: { enabled: false },
    },
    rulesets: [
      { id: 1, name: "Immutable Yield release tags", target: "tag", enforcement: "active" },
    ],
    pypiEnvironment: {
      name: "pypi-production",
      deployment_branch_policy: { protected_branches: true, custom_branch_policies: false },
      protection_rules: [
        {
          type: "required_reviewers",
          prevent_self_review: false,
          reviewers: [{ reviewer: { login: "bigboateng" }, type: "User" }],
        },
      ],
    },
    ...overrides,
  }
}

test("accepts the complete repository control surface", () => {
  assert.equal(auditRepositoryControls(controls()).requiredChecks, 4)
})

test("refuses a bypassable administrator or mutable action reference policy", () => {
  assert.throws(
    () =>
      auditRepositoryControls(
        controls({ protection: { ...controls().protection, enforce_admins: { enabled: false } } }),
      ),
    /administrators/,
  )
  assert.throws(
    () =>
      auditRepositoryControls(
        controls({ actions: { enabled: true, sha_pinning_required: false } }),
      ),
    /immutable SHA/,
  )
})

test("refuses an unreviewed PyPI production environment", () => {
  assert.throws(
    () =>
      auditRepositoryControls(
        controls({
          pypiEnvironment: {
            name: "pypi-production",
            deployment_branch_policy: { protected_branches: true },
            protection_rules: [],
          },
        }),
      ),
    /require bigboateng review/,
  )
})
