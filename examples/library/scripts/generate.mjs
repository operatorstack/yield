import { mkdir, writeFile } from "node:fs/promises"
import { execFile as execFileCallback } from "node:child_process"
import { dirname, join, resolve } from "node:path"
import { fileURLToPath } from "node:url"
import { promisify } from "node:util"

const libraryDir = resolve(dirname(fileURLToPath(import.meta.url)), "..")
const languages = ["typescript", "python", "go", "rust"]
const execFile = promisify(execFileCallback)

const patterns = [
  {
    slug: "review-branch",
    title: "Review a branch",
    summary: "Run mechanical checks, inspect the diff, and stop on critical findings.",
    preflightId: "check-branch",
    preflightCommand: "printf 'typecheck and tests passed\\n'",
    preflightClaim: "the branch passes mechanical checks",
    decisionId: "review-diff",
    instruction:
      "Review the branch for correctness, security, data-loss risks, and missing tests. Return pass only when no critical finding remains.",
    decisionClaim: "the review has no critical findings",
  },
  {
    slug: "investigate-failure",
    title: "Investigate a failure",
    summary: "Capture evidence, test the likely cause, and finish only with a causal explanation.",
    preflightId: "capture-failure",
    preflightCommand: "printf 'failing test captured with recent diff\\n'",
    preflightClaim: "the failure evidence is captured",
    decisionId: "diagnose-cause",
    instruction:
      "Use the failure output and recent change to identify the most likely root cause. Return pass only when the summary states a causal chain.",
    decisionClaim: "the diagnosis states a supported cause",
  },
  {
    slug: "qa-web-change",
    title: "QA a web change",
    summary: "Build first, exercise changed routes, and finish only with no blocking regressions.",
    preflightId: "build-web",
    preflightCommand: "printf 'build passed; changed routes: / and /settings\\n'",
    preflightClaim: "the web application builds",
    decisionId: "test-changed-routes",
    instruction:
      "Test the changed routes at desktop and mobile sizes, including keyboard navigation and form errors. Return pass only when no blocking regression remains.",
    decisionClaim: "the changed routes have no blocking regression",
  },
  {
    slug: "release-package",
    title: "Release a package",
    summary: "Test, review the release, ask for approval, publish, and verify.",
    preflightId: "test-package",
    preflightCommand: "printf 'package tests passed\\n'",
    preflightClaim: "the package tests pass",
    decisionId: "review-release",
    instruction:
      "Review the pending package release for breaking changes, missing notes, and rollback risk. Return pass only when it is ready to publish.",
    decisionClaim: "the package is ready to publish",
    approvalId: "approve-publish",
    approvalQuestion: "Publish this package release?",
    actionId: "publish-package",
    actionCommand: "printf 'package published\\n'",
    actionClaim: "the package publish command succeeds",
    verifyId: "verify-package",
    verifyCommand: "printf 'published package resolved from registry\\n'",
    verifyClaim: "the published package resolves from the registry",
  },
  {
    slug: "triage-issue",
    title: "Triage an issue",
    summary: "Read the report, classify impact, and return one evidence-backed next action.",
    preflightId: "read-issue",
    preflightCommand: "printf 'issue: intermittent timeout after retry change\\n'",
    preflightClaim: "the issue report is available",
    decisionId: "classify-issue",
    instruction:
      "Classify severity, identify missing evidence, and propose exactly one next action. Return pass only when the summary is actionable.",
    decisionClaim: "the issue has one actionable next step",
  },
  {
    slug: "repair-ci",
    title: "Repair a CI failure",
    summary: "Read the failed job, apply one supported repair, and rerun the failing check.",
    preflightId: "capture-ci-log",
    preflightCommand: "printf 'ci log: test shard 2 failed after cache restore\\n'",
    preflightClaim: "the failing CI evidence is captured",
    decisionId: "plan-ci-repair",
    instruction:
      "Diagnose the CI failure and describe the smallest supported repair. Return pass only when the repair is tied to the observed log.",
    decisionClaim: "the CI repair is supported by the failure evidence",
    actionId: "apply-ci-repair",
    actionCommand: "printf 'ci repair applied\\n'",
    actionClaim: "the CI repair command succeeds",
    verifyId: "rerun-ci-check",
    verifyCommand: "printf 'failing CI check now passes\\n'",
    verifyClaim: "the previously failing CI check passes",
  },
  {
    slug: "upgrade-dependency",
    title: "Upgrade a dependency",
    summary: "Establish a baseline, review compatibility, approve the change, and rerun tests.",
    preflightId: "baseline-tests",
    preflightCommand: "printf 'baseline tests passed\\n'",
    preflightClaim: "the baseline tests pass",
    decisionId: "review-upgrade",
    instruction:
      "Review the dependency upgrade for API changes, migration work, and rollback risk. Return pass only when the change is bounded.",
    decisionClaim: "the dependency upgrade has a bounded plan",
    approvalId: "approve-upgrade",
    approvalQuestion: "Apply the reviewed dependency upgrade?",
    actionId: "apply-upgrade",
    actionCommand: "printf 'dependency upgraded\\n'",
    actionClaim: "the dependency upgrade command succeeds",
    verifyId: "post-upgrade-tests",
    verifyCommand: "printf 'post-upgrade tests passed\\n'",
    verifyClaim: "the tests pass after the dependency upgrade",
  },
  {
    slug: "migrate-database",
    title: "Run a database migration",
    summary: "Dry-run, inspect risk, approve, apply, and verify the real result.",
    preflightId: "dry-run-migration",
    preflightCommand: "printf 'dry run: add users_email_idx concurrently\\n'",
    preflightClaim: "the migration dry-run succeeds",
    decisionId: "review-migration",
    instruction:
      "Review the migration plan for lock risk, irreversible work, and rollback. Return pass only when the plan is safe to apply.",
    decisionClaim: "the migration plan has acceptable risk",
    approvalId: "approve-migration",
    approvalQuestion: "Apply the reviewed database migration?",
    actionId: "apply-migration",
    actionCommand: "printf 'migration applied\\n'",
    actionClaim: "the migration applies cleanly",
    verifyId: "verify-migration",
    verifyCommand: "printf 'migration verification passed\\n'",
    verifyClaim: "the migrated database passes verification",
  },
  {
    slug: "audit-security",
    title: "Audit a change for security",
    summary: "Collect mechanical audit output, inspect trust boundaries, and reject critical risk.",
    preflightId: "run-security-checks",
    preflightCommand: "printf 'dependency and secret scans completed\\n'",
    preflightClaim: "the mechanical security checks complete",
    decisionId: "review-trust-boundaries",
    instruction:
      "Review authentication, authorization, input handling, secrets, and trust-boundary changes. Return pass only when no critical risk remains.",
    decisionClaim: "the change has no critical security finding",
  },
  {
    slug: "publish-ios",
    title: "Publish an iOS build",
    summary: "Archive, review release metadata, approve upload, publish, and verify processing.",
    preflightId: "archive-ios",
    preflightCommand: "printf 'iOS archive and tests passed\\n'",
    preflightClaim: "the iOS archive and tests pass",
    decisionId: "review-ios-release",
    instruction:
      "Review the iOS release metadata, versioning, privacy notes, and rollout risk. Return pass only when the build is ready for upload.",
    decisionClaim: "the iOS build is ready for upload",
    approvalId: "approve-ios-upload",
    approvalQuestion: "Upload this iOS build to App Store Connect?",
    actionId: "upload-ios",
    actionCommand: "printf 'iOS build uploaded\\n'",
    actionClaim: "the iOS upload command succeeds",
    verifyId: "verify-ios-processing",
    verifyCommand: "printf 'uploaded build entered processing\\n'",
    verifyClaim: "the uploaded iOS build entered processing",
  },
]

const decisionSchema = {
  type: "object",
  required: ["status", "critical", "summary"],
  properties: {
    status: { enum: ["pass", "needs_work"] },
    critical: { type: "integer", minimum: 0 },
    summary: { type: "string", minLength: 1 },
  },
}

function quoted(value) {
  return JSON.stringify(value)
}

function indent(lines, spaces) {
  const prefix = " ".repeat(spaces)
  return lines.map((line) => (line ? prefix + line : line))
}

function resultLines(pattern, language) {
  if (language === "typescript")
    return ["return { workflow: " + quoted(pattern.slug) + ", summary: decision.summary }"]
  if (language === "python")
    return ['return {"workflow": ' + quoted(pattern.slug) + ', "summary": decision["summary"]}']
  if (language === "go")
    return [
      'return ctx.Complete(map[string]any{"workflow": ' +
        quoted(pattern.slug) +
        ', "summary": decision.Summary})',
    ]
  return ['Ok(json!({"workflow": ' + quoted(pattern.slug) + ', "summary": decision["summary"]}))']
}

function addOptionalTypeScriptSteps(body, pattern) {
  if (pattern.approvalId) {
    body.push(
      "",
      "const approval = ctx.askUser(" +
        quoted(pattern.approvalId) +
        ", " +
        quoted(pattern.approvalQuestion) +
        ", [",
      '  { value: "continue", label: "Continue" },',
      '  { value: "stop", label: "Stop" },',
      "])",
      'if (approval !== "continue") ctx.refused("the operator declined to continue")',
    )
  }
  if (pattern.actionId) {
    body.push(
      "",
      "const action = ctx.runCommand(" +
        quoted(pattern.actionId) +
        ", " +
        quoted(pattern.actionCommand) +
        ", 600)",
      "ctx.require(action.exit_code === 0, " + quoted(pattern.actionClaim) + ", action)",
    )
  }
  if (pattern.verifyId) {
    body.push(
      "",
      "const verify = ctx.runCommand(" +
        quoted(pattern.verifyId) +
        ", " +
        quoted(pattern.verifyCommand) +
        ", 300)",
      "ctx.require(verify.exit_code === 0, " + quoted(pattern.verifyClaim) + ", verify)",
    )
  }
}

function renderTypeScript(pattern) {
  const body = [
    "const preflight = ctx.runCommand(" +
      quoted(pattern.preflightId) +
      ", " +
      quoted(pattern.preflightCommand) +
      ", 300)",
    "ctx.require(preflight.exit_code === 0, " + quoted(pattern.preflightClaim) + ", preflight)",
    "",
    "const decision = ctx.agentTask<Decision>(",
    "  " + quoted(pattern.decisionId) + ",",
    "  " + quoted(pattern.instruction) + ",",
    "  { stdout: preflight.stdout, stderr: preflight.stderr },",
    "  decisionSchema,",
    ")",
    'ctx.require(decision.status === "pass" && decision.critical === 0, ' +
      quoted(pattern.decisionClaim) +
      ", decision)",
  ]
  addOptionalTypeScriptSteps(body, pattern)
  body.push("", ...resultLines(pattern, "typescript"))
  return [
    "// " + pattern.title + ". Replace the illustrative commands with your project commands.",
    'import { defineSkill } from "../../../../sdk/typescript/src/index.ts";',
    "",
    'type Decision = { status: "pass" | "needs_work"; critical: number; summary: string };',
    "const decisionSchema = " + JSON.stringify(decisionSchema, null, 2) + ";",
    "",
    "defineSkill((ctx) => {",
    ...indent(body, 2),
    "});",
    "",
  ].join("\n")
}

function addOptionalPythonSteps(body, pattern) {
  if (pattern.approvalId) {
    body.push(
      "",
      "approval = ctx.ask_user(",
      "    " + quoted(pattern.approvalId) + ",",
      "    " + quoted(pattern.approvalQuestion) + ",",
      '    options=[{"value": "continue", "label": "Continue"}, {"value": "stop", "label": "Stop"}],',
      ")",
      'if approval != "continue":',
      '    ctx.refused("the operator declined to continue")',
    )
  }
  if (pattern.actionId) {
    body.push(
      "",
      "action = ctx.run_command(" +
        quoted(pattern.actionId) +
        ", " +
        quoted(pattern.actionCommand) +
        ", 600)",
      "ctx.require(action.exit_code == 0, " + quoted(pattern.actionClaim) + ", action)",
    )
  }
  if (pattern.verifyId) {
    body.push(
      "",
      "verify = ctx.run_command(" +
        quoted(pattern.verifyId) +
        ", " +
        quoted(pattern.verifyCommand) +
        ", 300)",
      "ctx.require(verify.exit_code == 0, " + quoted(pattern.verifyClaim) + ", verify)",
    )
  }
}

function renderPython(pattern) {
  const body = [
    "preflight = ctx.run_command(" +
      quoted(pattern.preflightId) +
      ", " +
      quoted(pattern.preflightCommand) +
      ", 300)",
    "ctx.require(preflight.exit_code == 0, " + quoted(pattern.preflightClaim) + ", preflight)",
    "",
    "decision = ctx.agent_task(",
    "    " + quoted(pattern.decisionId) + ",",
    "    " + quoted(pattern.instruction) + ",",
    '    context={"stdout": preflight.stdout, "stderr": preflight.stderr},',
    "    schema=DECISION_SCHEMA,",
    ")",
    'ctx.require(decision["status"] == "pass" and decision["critical"] == 0, ' +
      quoted(pattern.decisionClaim) +
      ", decision)",
  ]
  addOptionalPythonSteps(body, pattern)
  body.push("", ...resultLines(pattern, "python"))
  return [
    "# " + pattern.title + ". Replace the illustrative commands with your project commands.",
    "import sys",
    "from pathlib import Path",
    "",
    'sys.path.insert(0, str(Path(__file__).resolve().parents[4] / "sdk" / "python"))',
    "from yieldskill import define_skill  # noqa: E402",
    "",
    "DECISION_SCHEMA = " + JSON.stringify(decisionSchema, null, 2),
    "",
    "def program(ctx):",
    ...indent(body, 4),
    "",
    "define_skill(program)",
    "",
  ].join("\n")
}

function addOptionalGoSteps(body, pattern) {
  if (pattern.approvalId) {
    body.push(
      "",
      "approval := ctx.AskUser(",
      "  " + quoted(pattern.approvalId) + ",",
      "  " + quoted(pattern.approvalQuestion) + ",",
      '  yield.Option{Value: "continue", Label: "Continue"},',
      '  yield.Option{Value: "stop", Label: "Stop"},',
      ")",
      'if approval != "continue" {',
      '  return yield.Outcome{}, ctx.Refused("the operator declined to continue")',
      "}",
    )
  }
  if (pattern.actionId) {
    body.push(
      "",
      "action := ctx.RunCommand(" +
        quoted(pattern.actionId) +
        ", " +
        quoted(pattern.actionCommand) +
        ", 600)",
      "ctx.Require(action.ExitCode == 0, " + quoted(pattern.actionClaim) + ", action)",
    )
  }
  if (pattern.verifyId) {
    body.push(
      "",
      "verify := ctx.RunCommand(" +
        quoted(pattern.verifyId) +
        ", " +
        quoted(pattern.verifyCommand) +
        ", 300)",
      "ctx.Require(verify.ExitCode == 0, " + quoted(pattern.verifyClaim) + ", verify)",
    )
  }
}

function renderGo(pattern) {
  const goTick = String.fromCharCode(96)
  const body = [
    "preflight := ctx.RunCommand(" +
      quoted(pattern.preflightId) +
      ", " +
      quoted(pattern.preflightCommand) +
      ", 300)",
    "ctx.Require(preflight.ExitCode == 0, " + quoted(pattern.preflightClaim) + ", preflight)",
    "",
    "raw := ctx.AgentTask(",
    "  " + quoted(pattern.decisionId) + ",",
    "  " + quoted(pattern.instruction) + ",",
    '  map[string]any{"stdout": preflight.Stdout, "stderr": preflight.Stderr},',
    "  json.RawMessage(decisionSchema),",
    ")",
    "var decision decision",
    "if err := json.Unmarshal(raw, &decision); err != nil {",
    "  return yield.Outcome{}, err",
    "}",
    'ctx.Require(decision.Status == "pass" && decision.Critical == 0, ' +
      quoted(pattern.decisionClaim) +
      ", decision)",
  ]
  addOptionalGoSteps(body, pattern)
  body.push("", ...resultLines(pattern, "go"))
  return [
    "// " + pattern.title + ". Replace the illustrative commands with your project commands.",
    "package main",
    "",
    "import (",
    '  "encoding/json"',
    "",
    '  "github.com/operatorstack/yield/sdk/yield"',
    ")",
    "",
    "type decision struct {",
    "  Status string " + goTick + 'json:"status"' + goTick,
    "  Critical int " + goTick + 'json:"critical"' + goTick,
    "  Summary string " + goTick + 'json:"summary"' + goTick,
    "}",
    "",
    "const decisionSchema = " + goTick + JSON.stringify(decisionSchema) + goTick,
    "",
    "func main() {",
    "  yield.Main(func(ctx *yield.Context) (yield.Outcome, error) {",
    ...indent(body, 4),
    "  })",
    "}",
    "",
  ].join("\n")
}

function addOptionalRustSteps(body, pattern) {
  if (pattern.approvalId) {
    body.push(
      "",
      "let approval = ctx.ask_user(",
      "    " + quoted(pattern.approvalId) + ",",
      "    " + quoted(pattern.approvalQuestion) + ",",
      '    &[("continue", "Continue"), ("stop", "Stop")],',
      ");",
      'if approval != "continue" {',
      '    return Err(ctx.refused("the operator declined to continue"));',
      "}",
    )
  }
  if (pattern.actionId) {
    body.push(
      "",
      "let action = ctx.run_command(" +
        quoted(pattern.actionId) +
        ", " +
        quoted(pattern.actionCommand) +
        ", 600);",
      "ctx.require(",
      "    action.exit_code == 0,",
      "    " + quoted(pattern.actionClaim) + ",",
      '    Some(&json!({"exit_code": action.exit_code})),',
      ");",
    )
  }
  if (pattern.verifyId) {
    body.push(
      "",
      "let verify = ctx.run_command(" +
        quoted(pattern.verifyId) +
        ", " +
        quoted(pattern.verifyCommand) +
        ", 300);",
      "ctx.require(",
      "    verify.exit_code == 0,",
      "    " + quoted(pattern.verifyClaim) + ",",
      '    Some(&json!({"exit_code": verify.exit_code})),',
      ");",
    )
  }
}

function renderRust(pattern) {
  const body = [
    "let preflight = ctx.run_command(" +
      quoted(pattern.preflightId) +
      ", " +
      quoted(pattern.preflightCommand) +
      ", 300);",
    "ctx.require(",
    "    preflight.exit_code == 0,",
    "    " + quoted(pattern.preflightClaim) + ",",
    '    Some(&json!({"exit_code": preflight.exit_code})),',
    ");",
    "",
    "let decision = ctx.agent_task(",
    "    " + quoted(pattern.decisionId) + ",",
    "    " + quoted(pattern.instruction) + ",",
    '    Some(json!({"stdout": preflight.stdout, "stderr": preflight.stderr})),',
    "    Some(decision_schema()),",
    ");",
    "ctx.require(",
    '    decision["status"] == "pass" && decision["critical"] == 0,',
    "    " + quoted(pattern.decisionClaim) + ",",
    "    Some(&decision),",
    ");",
  ]
  addOptionalRustSteps(body, pattern)
  body.push("", ...resultLines(pattern, "rust"))
  return [
    "// " + pattern.title + ". Replace the illustrative commands with your project commands.",
    "use serde_json::{json, Value};",
    "use yieldskill::{define_skill, Context, SkillResult};",
    "",
    "fn decision_schema() -> Value {",
    "    json!(" + JSON.stringify(decisionSchema) + ")",
    "}",
    "",
    "fn program(ctx: &mut Context) -> SkillResult {",
    ...indent(body, 4),
    "}",
    "",
    "fn main() {",
    "    define_skill(program);",
    "}",
    "",
  ].join("\n")
}

function renderSkill(pattern) {
  return [
    "---",
    "name: " + pattern.slug,
    "description: " + pattern.summary,
    "---",
    "",
    "Run:",
    "",
    "    yskill run .",
    "",
    "Follow each returned operation exactly and resume the run with the result.",
    "The program owns order, approval, commands, and finish rules. The agent",
    "owns judgment inside each agent task. Replace the illustrative commands",
    "with the real commands from your repository before using this workflow.",
    "",
  ].join("\n")
}

function renderFixture(pattern) {
  const fixture = {
    [pattern.decisionId]: {
      status: "pass",
      critical: 0,
      summary: "Fixture result: " + pattern.summary,
    },
  }
  if (pattern.approvalId) fixture[pattern.approvalId] = { value: "continue" }
  return JSON.stringify(fixture, null, 2) + "\n"
}

function manifestFor(language, slug) {
  if (language === "typescript") return { run: ["node", "../src/" + slug + ".ts"] }
  if (language === "python") return { run: ["python3", "../src/" + slug + ".py"] }
  if (language === "go") return { run: ["go", "run", "../src/" + slug + "/main.go"] }
  return { run: ["cargo", "run", "--quiet", "--manifest-path", "../Cargo.toml", "--bin", slug] }
}

async function write(path, content) {
  await mkdir(dirname(path), { recursive: true })
  await writeFile(path, content)
}

await write(
  join(libraryDir, "catalog.json"),
  JSON.stringify(
    patterns.map((pattern) => ({
      slug: pattern.slug,
      title: pattern.title,
      summary: pattern.summary,
      languages,
    })),
    null,
    2,
  ) + "\n",
)

await write(
  join(libraryDir, "rust", "Cargo.toml"),
  [
    "[package]",
    'name = "yield-example-library"',
    'version = "0.1.0"',
    'edition = "2021"',
    "publish = false",
    "",
    "[dependencies]",
    'yieldskill = { path = "../../../sdk/rust" }',
    'serde_json = "1"',
    "",
  ].join("\n"),
)

const goSources = []
for (const pattern of patterns) {
  const sources = {
    typescript: renderTypeScript(pattern),
    python: renderPython(pattern),
    go: renderGo(pattern),
    rust: renderRust(pattern),
  }
  const extensions = { typescript: "ts", python: "py", go: "go", rust: "rs" }
  for (const language of languages) {
    const sourceDir =
      language === "rust"
        ? join(libraryDir, language, "src", "bin")
        : language === "go"
          ? join(libraryDir, language, "src", pattern.slug)
          : join(libraryDir, language, "src")
    const sourceName = language === "go" ? "main.go" : pattern.slug + "." + extensions[language]
    const sourcePath = join(sourceDir, sourceName)
    await write(sourcePath, sources[language])
    if (language === "go") goSources.push(sourcePath)
    const skillDir = join(libraryDir, language, pattern.slug)
    await write(join(skillDir, "SKILL.md"), renderSkill(pattern))
    await write(
      join(skillDir, "skill.json"),
      JSON.stringify(manifestFor(language, pattern.slug), null, 2) + "\n",
    )
    await write(join(skillDir, "fixtures", "responses.json"), renderFixture(pattern))
  }
}

await execFile("gofmt", ["-w", ...goSources])
await execFile("cargo", ["fmt", "--manifest-path", join(libraryDir, "rust", "Cargo.toml")])

console.log("generated " + patterns.length + " patterns in " + languages.length + " languages")
