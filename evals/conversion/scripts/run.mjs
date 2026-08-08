import { createHash } from "node:crypto"
import { execFileSync, spawnSync } from "node:child_process"
import {
  chmod,
  cp,
  mkdir,
  mkdtemp,
  readFile,
  readdir,
  realpath,
  rm,
  writeFile,
} from "node:fs/promises"
import { homedir, tmpdir } from "node:os"
import { dirname, join, relative } from "node:path"
import { conversionRoot, evalRoot, sourceHash, yieldRoot } from "./surface.mjs"

const model = "gpt-5.6-sol"
const reasoning = "medium"
const runsRoot = join(evalRoot, "runs/conversion")
const resultPath = join(evalRoot, "results/latest-conversion.json")

function command(program, args, cwd = yieldRoot) {
  return execFileSync(program, args, { cwd, encoding: "utf8", env: process.env }).trim()
}

async function prepareAuthHome(parent) {
  const target = join(parent, "codex-home")
  await mkdir(target)
  if (!process.env.CODEX_API_KEY) {
    const source = join(process.env.CODEX_HOME ?? join(homedir(), ".codex"), "auth.json")
    await cp(source, join(target, "auth.json"))
    await chmod(join(target, "auth.json"), 0o600)
  }
  return target
}

function parseUsage(stdout) {
  let usage = {
    input_tokens: 0,
    cached_input_tokens: 0,
    output_tokens: 0,
    reasoning_output_tokens: 0,
  }
  for (const line of stdout.split("\n")) {
    if (!line.startsWith("{")) continue
    try {
      const event = JSON.parse(line)
      if (event.type === "turn.completed" && event.usage) usage = event.usage
    } catch {}
  }
  return usage
}

async function runCodex({ repo, authHome, prompt, evidenceDir, outputSchema, outputFile }) {
  const args = [
    "exec",
    "--ephemeral",
    "--ignore-user-config",
    "--ignore-rules",
    "--disable",
    "plugins",
    "--disable",
    "remote_plugin",
    "--disable",
    "apps",
    "--disable",
    "memories",
    "--disable",
    "goals",
    "--disable",
    "multi_agent",
    "--disable",
    "browser_use",
    "--disable",
    "computer_use",
    "--disable",
    "image_generation",
    "--disable",
    "skill_search",
    "--disable",
    "workspace_dependencies",
    "--json",
    "--sandbox",
    "danger-full-access",
    "-C",
    repo,
    "--model",
    model,
    "-c",
    `model_reasoning_effort=\"${reasoning}\"`,
  ]
  if (outputSchema) args.push("--output-schema", outputSchema)
  args.push("--output-last-message", outputFile, prompt)
  const execution = spawnSync("codex", args, {
    cwd: repo,
    encoding: "utf8",
    timeout: 12 * 60 * 1000,
    maxBuffer: 64 * 1024 * 1024,
    env: { ...process.env, CODEX_HOME: authHome },
  })
  await mkdir(evidenceDir, { recursive: true })
  await writeFile(join(evidenceDir, "transcript.jsonl"), execution.stdout ?? "")
  await writeFile(join(evidenceDir, "stderr.log"), execution.stderr ?? "")
  if (execution.status !== 0)
    throw new Error(`Codex exited ${execution.status}; see ${evidenceDir}`)
  return parseUsage(execution.stdout ?? "")
}

async function prepareRepository(session, yskill) {
  const unresolved = join(session, "repository")
  await mkdir(unresolved)
  const repo = await realpath(unresolved)
  await mkdir(join(repo, "skills/source-release"), { recursive: true })
  await cp(
    join(conversionRoot, "fixtures/source-skill/SKILL.md"),
    join(repo, "skills/source-release/SKILL.md"),
  )
  await writeFile(join(repo, "go.mod"), "module example.com/conversion-eval\n\ngo 1.26.5\n")
  command("git", ["init", "-b", "main"], repo)
  command("git", ["config", "user.email", "eval@operatorstack.systems"], repo)
  command("git", ["config", "user.name", "Yield Eval"], repo)
  command(
    yskill,
    ["bootstrap", "--root", repo, "--language", "go", "--agent", "codex", "--yes"],
    repo,
  )
  command("git", ["add", "."], repo)
  command("git", ["commit", "-m", "conversion evaluation fixture"], repo)
  return repo
}

async function readEvents(path) {
  return (await readFile(path, "utf8"))
    .split("\n")
    .filter(Boolean)
    .map((line) => JSON.parse(line))
}

async function builderEvidence(repo) {
  const directory = join(repo, "skills/yield-workflow-builder/.yield/runs")
  const logs = (await readdir(directory)).filter((name) => name.endsWith(".jsonl"))
  let selected
  for (const name of logs.sort()) {
    const path = join(directory, name)
    const events = await readEvents(path)
    const semantic = events.some(
      (event) =>
        event.type === "operation.completed" && event.data?.request_id === "project-semantics",
    )
    const complete = events.some((event) => event.type === "run.completed")
    if (semantic && (complete || !selected)) selected = { path, events }
  }
  if (!selected) throw new Error("builder produced no semantic-conversion run log")
  const { path, events } = selected
  const terminal = events.findLast(
    (event) =>
      event.type === "run.completed" ||
      event.type === "run.blocked" ||
      event.type === "run.refused",
  )
  if (terminal?.type !== "run.completed")
    throw new Error(`builder terminal was ${terminal?.type ?? "missing"}`)
  const projected = events.find(
    (event) =>
      event.type === "operation.completed" && event.data?.request_id === "project-semantics",
  )
  if (!projected?.data?.result) throw new Error("builder run log has no semantic projection")
  return { path, projection: projected.data.result }
}

function addUsage(left, right) {
  return Object.fromEntries(
    ["input_tokens", "cached_input_tokens", "output_tokens", "reasoning_output_tokens"].map(
      (key) => [key, (left[key] ?? 0) + (right[key] ?? 0)],
    ),
  )
}

async function main() {
  const session = await mkdtemp(join(tmpdir(), "yield-conversion-eval-"))
  const stamp = new Date().toISOString().replaceAll(":", "-")
  const evidenceRoot = join(runsRoot, stamp)
  const yskill = join(session, "yskill")
  command("go", ["build", "-ldflags", "-X main.version=0.1.38", "-o", yskill, "./cmd/yskill"])
  const authHome = await prepareAuthHome(session)
  try {
    const repo = await prepareRepository(session, yskill)
    const candidateOutput = join(repo, ".yield/candidate-session.txt")
    await mkdir(join(repo, ".yield"), { recursive: true })
    const candidateUsage = await runCodex({
      repo,
      authHome,
      evidenceDir: join(evidenceRoot, "candidate"),
      outputFile: candidateOutput,
      prompt:
        "Read .agents/skills/yield-workflow-builder/SKILL.md and use the real builder. Convert skills/source-release into a new Go skill workflow at skills/converted-release. Drive every Yield operation to a terminal result. Do not imitate the workflow or bypass yskill. Use the current request as the specification.",
    })
    const evidence = await builderEvidence(repo)
    await cp(evidence.path, join(evidenceRoot, "candidate/builder-run.jsonl"))
    await mkdir(join(repo, ".eval/negative-control"), { recursive: true })
    await cp(
      join(conversionRoot, "fixtures/negative-control"),
      join(repo, ".eval/negative-control"),
      { recursive: true },
    )
    await cp(
      join(conversionRoot, "fixtures/fault-probes.json"),
      join(repo, ".eval/fault-probes.json"),
    )
    await writeFile(
      join(repo, ".eval/projection.json"),
      JSON.stringify(evidence.projection, null, 2) + "\n",
    )
    const judgeOutput = join(repo, ".eval/judge.json")
    const judgeUsage = await runCodex({
      repo,
      authHome,
      evidenceDir: join(evidenceRoot, "judge"),
      outputSchema: join(conversionRoot, "judge-schema.json"),
      outputFile: judgeOutput,
      prompt:
        "Act as an independent semantic-disposition judge. Read skills/source-release/SKILL.md, .eval/projection.json, every file in skills/converted-release, every file in .eval/negative-control, and .eval/fault-probes.json. Accept the generated candidate only if every source clause has exactly one valid disposition, each required destination exists and remains reachable by a coding agent, control is enforced in code, useful guidance remains model-facing, both reaches both, and exclusions have no destination plus a reason. Reject the static negative control because it deliberately drops guidance. Mark each defect probe true only when you recognize that it must be rejected. Return only schema-valid JSON.",
    })
    const judge = JSON.parse(await readFile(judgeOutput, "utf8"))
    const clauses = evidence.projection.clauses ?? []
    const counts = Object.fromEntries(
      ["control", "guidance", "both", "excluded"].map((kind) => [
        kind,
        clauses.filter((clause) => clause.disposition === kind).length,
      ]),
    )
    const findings = judge.clause_findings ?? []
    const passed =
      judge.candidate?.verdict === "accept" &&
      judge.negative_control?.verdict === "reject" &&
      clauses.length === 4 &&
      Object.values(counts).every((count) => count === 1) &&
      findings.length === 4 &&
      findings.every((finding) => finding.preserved === true && finding.reachable === true) &&
      Object.values(judge.defect_detection ?? {}).every((value) => value === true)
    const source = await readFile(join(conversionRoot, "fixtures/source-skill/SKILL.md"))
    const receipt = {
      schema_version: 1,
      methodology_version: "semantic-disposition-v1",
      generated_at: new Date().toISOString(),
      source_hash: await sourceHash(),
      fixture_source_hash: createHash("sha256").update(source).digest("hex"),
      status: passed ? "passed" : "failed",
      model: {
        product: "Codex CLI",
        cli_version: command("codex", ["--version"]),
        name: model,
        reasoning,
      },
      sessions: 2,
      token_usage: addUsage(candidateUsage, judgeUsage),
      clause_counts: { total: clauses.length, ...counts },
      candidate_verdict: judge.candidate?.verdict,
      negative_control_verdict: judge.negative_control?.verdict,
      defect_detection: judge.defect_detection,
      claim_boundary:
        "Advisory evidence for this four-clause fixture. The contract is stable; model projections can differ.",
    }
    if (process.argv.includes("--write"))
      await writeFile(resultPath, JSON.stringify(receipt, null, 2) + "\n")
    console.log(
      JSON.stringify(
        {
          status: receipt.status,
          clauses: receipt.clause_counts,
          evidence: relative(evalRoot, evidenceRoot),
        },
        null,
        2,
      ),
    )
    if (!passed) process.exitCode = 1
  } finally {
    await rm(session, { recursive: true, force: true })
  }
}

await main()
