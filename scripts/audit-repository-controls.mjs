#!/usr/bin/env node
import process from "node:process";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";

const requiredChecks = [
  "Go and agent registration (ubuntu-latest)",
  "Go and agent registration (macos-latest)",
  "Go and agent registration (windows-latest)",
  "Release authority and full validation",
];

function expect(condition, message) {
  if (!condition) throw new Error(message);
}

export function auditRepositoryControls({ workflow, actions, protection, rulesets, pypiEnvironment }) {
  expect(workflow.default_workflow_permissions === "read", "default workflow permissions must be read-only");
  expect(workflow.can_approve_pull_request_reviews === false, "workflows must not approve pull requests");
  expect(actions.enabled === true && actions.sha_pinning_required === true, "Actions must require immutable SHA references");
  expect(protection.enforce_admins?.enabled === true, "administrators must not bypass main protection");
  expect(protection.required_status_checks?.strict === true, "required checks must run against current main");
  expect(protection.allow_force_pushes?.enabled === false, "main must reject force pushes");
  expect(protection.allow_deletions?.enabled === false, "main must reject deletion");
  const contexts = new Set(protection.required_status_checks?.contexts ?? []);
  for (const check of requiredChecks) expect(contexts.has(check), `main is missing required check: ${check}`);
  const tagRule = rulesets.find((ruleset) => ruleset.name === "Immutable Yield release tags");
  expect(tagRule?.target === "tag" && tagRule.enforcement === "active", "immutable release-tag ruleset must be active");
  expect(pypiEnvironment?.name === "pypi-production", "pypi-production environment must exist");
  expect(pypiEnvironment.deployment_branch_policy?.protected_branches === true, "PyPI production must accept protected branches only");
  const reviewerRules = pypiEnvironment.protection_rules?.filter(({ type }) => type === "required_reviewers") ?? [];
  expect(reviewerRules.some(({ reviewers }) => reviewers?.some(({ reviewer }) => reviewer?.login === "bigboateng")), "PyPI production must require bigboateng review");
  expect(reviewerRules.every(({ prevent_self_review }) => prevent_self_review === false), "PyPI production must permit the authorized operator to approve the first release");
  return { requiredChecks: requiredChecks.length, immutableTagRuleset: tagRule.id, pypiEnvironment: pypiEnvironment.name };
}

async function github(path) {
  const repository = process.env.GITHUB_REPOSITORY ?? "operatorstack/yield";
  const response = await fetch(`https://api.github.com/repos/${repository}/${path}`, {
    headers: {
      Accept: "application/vnd.github+json",
      Authorization: `Bearer ${process.env.GH_TOKEN}`,
      "X-GitHub-Api-Version": "2022-11-28",
    },
  });
  if (!response.ok) throw new Error(`${path}: GitHub HTTP ${response.status}`);
  return response.json();
}

async function main() {
  expect(process.env.GH_TOKEN, "GH_TOKEN is required");
  const [workflow, actions, protection, rulesets, pypiEnvironment] = await Promise.all([
    github("actions/permissions/workflow"),
    github("actions/permissions"),
    github("branches/main/protection"),
    github("rulesets"),
    github("environments/pypi-production"),
  ]);
  const result = auditRepositoryControls({ workflow, actions, protection, rulesets, pypiEnvironment });
  console.log(`repository-controls: ${result.requiredChecks} required checks, immutable tag ruleset ${result.immutableTagRuleset}, and ${result.pypiEnvironment} verified`);
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  main().catch((error) => { console.error(`repository-controls: ${error.message}`); process.exit(1); });
}
