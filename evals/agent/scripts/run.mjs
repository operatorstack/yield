import { createHash } from "node:crypto"
import { execFileSync, spawnSync } from "node:child_process"
import { chmod, cp, mkdir, mkdtemp, readFile, readdir, rm, stat, writeFile } from "node:fs/promises"
import { homedir, tmpdir } from "node:os"
import { dirname, join, relative, resolve } from "node:path"
import { fileURLToPath } from "node:url"

const agentRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..")
const evalRoot = resolve(agentRoot, "..")
const yieldRoot = resolve(evalRoot, "..")
const fixturesRoot = join(agentRoot, "fixtures")
const runsRoot = join(evalRoot, "runs/agent")
const selectedCase = valueAfter("--case")
const selectedArm = valueAfter("--arm")
const repeat = Number(valueAfter("--repeat") ?? "1")
const model = process.env.EVAL_AGENT_MODEL ?? "gpt-5.6-terra"
const reasoning = process.env.EVAL_AGENT_REASONING ?? "medium"
const arms = selectedArm ? [selectedArm] : ["long", "yield"]

if (!Number.isInteger(repeat) || repeat < 1) throw new Error("--repeat must be a positive integer")
if (arms.some((arm) => !["long", "yield"].includes(arm))) throw new Error("--arm must be long or yield")

function valueAfter(flag) {
  const index = process.argv.indexOf(flag)
  return index === -1 ? undefined : process.argv[index + 1]
}

function command(command, args, cwd = yieldRoot) {
  return execFileSync(command, args, { cwd, encoding: "utf8", env: process.env }).trim()
}

async function sourceHash() {
  const roots = [
    [evalRoot, "agent/cases.json"],
    [evalRoot, "agent/fixtures"],
    [evalRoot, "agent/scripts"],
    [yieldRoot, "cmd/yskill"],
    [yieldRoot, "internal"],
    [yieldRoot, "sdk/typescript/src/index.ts"],
  ]
  const files = []
  for (const [base, root] of roots) files.push(...await filesUnder(join(base, root)))
  files.sort()
  const hash = createHash("sha256")
  for (const path of files) {
    hash.update(relative(yieldRoot, path))
    hash.update("\0")
    hash.update(await readFile(path))
    hash.update("\0")
  }
  return hash.digest("hex")
}

async function filesUnder(path) {
  if ((await stat(path)).isFile()) return [path]
  const entries = await readdir(path, { withFileTypes: true })
  const files = []
  for (const entry of entries) {
    const child = join(path, entry.name)
    if (entry.isDirectory()) files.push(...await filesUnder(child))
    else files.push(child)
  }
  return files
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

async function prepareRepo(parent, arm, testCase, yskill, repeatIndex) {
  const repo = join(parent, `${testCase.id}-${arm}-${repeatIndex}`)
  await mkdir(repo)
  await cp(join(fixturesRoot, "shared"), repo, { recursive: true })
  await cp(join(fixturesRoot, arm), repo, { recursive: true })
  await mkdir(join(repo, ".eval"), { recursive: true })
  await writeFile(join(repo, ".eval/case.json"), JSON.stringify({ commands: testCase.commands, answers: testCase.answers }, null, 2) + "\n")
  await writeFile(join(repo, ".eval/release.json"), JSON.stringify(testCase.review, null, 2) + "\n")
  await writeFile(join(repo, ".eval/output-schema.json"), JSON.stringify({
    type: "object",
    required: ["status", "summary"],
    properties: {
      status: { enum: ["completed", "blocked", "refused", "failed"] },
      summary: { type: "string" },
    },
    additionalProperties: false,
  }, null, 2) + "\n")
  if (arm === "yield") {
    await cp(join(yieldRoot, "sdk/typescript/src/index.ts"), join(repo, "skills/release/yield-sdk.ts"))
    await cp(yskill, join(repo, "bin/yskill"))
    await chmod(join(repo, "bin/yskill"), 0o755)
  }
  command("git", ["init", "-b", "main"], repo)
  command("git", ["config", "user.email", "eval@operatorstack.systems"], repo)
  command("git", ["config", "user.name", "Yield Eval"], repo)
  command("git", ["add", "."], repo)
  command("git", ["commit", "-m", "evaluation fixture"], repo)
  return repo
}

function parseCodexUsage(stdout) {
  let usage = { input_tokens: 0, cached_input_tokens: 0, output_tokens: 0, reasoning_output_tokens: 0 }
  for (const line of stdout.split("\n")) {
    if (!line.startsWith("{")) continue
    try {
      const event = JSON.parse(line)
      if (event.type === "turn.completed" && event.usage) usage = event.usage
    } catch {}
  }
  return usage
}

async function runCodex(repo, authHome, evidenceDir) {
  const prompt = [
    "Read SKILL.md and execute that workflow exactly.",
    "This evaluates workflow control, not your domain knowledge.",
    "Do not edit SKILL.md, workflow code, bin scripts, or files under .eval.",
    "Do not read .eval/case.json. Read .eval/release.json only when the workflow asks.",
    "Do not infer hidden expectations. Report the terminal status you actually reach.",
  ].join(" ")
  const started = Date.now()
  const execution = spawnSync("codex", [
    "exec", "--ephemeral", "--ignore-user-config", "--ignore-rules",
    "--disable", "plugins", "--disable", "remote_plugin", "--disable", "apps",
    "--disable", "memories", "--disable", "goals", "--disable", "multi_agent",
    "--disable", "browser_use", "--disable", "computer_use", "--disable", "image_generation",
    "--disable", "skill_search", "--disable", "workspace_dependencies",
    "--json", "--sandbox", "workspace-write", "-C", repo,
    "--model", model, "-c", `model_reasoning_effort=\"${reasoning}\"`,
    "--output-schema", join(repo, ".eval/output-schema.json"),
    "--output-last-message", join(repo, ".eval/codex-last.json"),
    prompt,
  ], {
    cwd: repo,
    encoding: "utf8",
    timeout: 10 * 60 * 1000,
    maxBuffer: 64 * 1024 * 1024,
    env: { ...process.env, CODEX_HOME: authHome },
  })
  await mkdir(evidenceDir, { recursive: true })
  await writeFile(join(evidenceDir, "transcript.jsonl"), execution.stdout ?? "")
  await writeFile(join(evidenceDir, "stderr.log"), execution.stderr ?? "")
  if (execution.status !== 0) throw new Error(`Codex exited ${execution.status}; see ${evidenceDir}`)
  return { duration_ms: Date.now() - started, usage: parseCodexUsage(execution.stdout ?? "") }
}

async function readJSONL(path) {
  const text = await readFile(path, "utf8")
  return text.split("\n").filter(Boolean).map((line) => JSON.parse(line))
}

async function scoreLong(repo) {
  const events = await readJSONL(join(repo, ".eval/observed.jsonl"))
  const steps = events.filter((event) => ["run_command", "agent_task", "ask_user"].includes(event.kind)).map((event) => event.id)
  const terminals = events.filter((event) => event.kind === "terminal")
  if (terminals.length !== 1) throw new Error(`long arm wrote ${terminals.length} terminal events`)
  return {
    steps,
    terminal: terminals[0].id,
    requirements: events.filter((event) => event.kind === "requirement").map((event) => Boolean(event.result?.passed)),
    command_evidence: events.filter((event) => event.kind === "run_command"),
    accepted_agent_tasks: events.filter((event) => event.kind === "agent_task").length,
    accepted_user_answers: events.filter((event) => event.kind === "ask_user").length,
  }
}

async function scoreYield(repo) {
  const runsDir = join(repo, "skills/release/.yield/runs")
  const logs = (await readdir(runsDir)).filter((name) => name.endsWith(".jsonl"))
  if (logs.length !== 1) throw new Error(`Yield arm produced ${logs.length} run logs`)
  const events = await readJSONL(join(runsDir, logs[0]))
  const requests = events.filter((event) => event.type === "operation.requested").map((event) => event.data.request)
  const completed = new Set(events.filter((event) => event.type === "operation.completed").map((event) => event.data.request_id))
  const terminalEvent = events.findLast((event) => ["run.completed", "run.blocked", "run.refused"].includes(event.type))
  if (!terminalEvent) throw new Error("Yield arm did not reach a terminal event")
  const terminal = terminalEvent.type.slice(4)
  const rejected = events.filter((event) => event.type === "response.rejected")
  const unanswered = requests.filter((request) => !completed.has(request.id))
  if (unanswered.length) throw new Error(`Yield log has unanswered requests: ${unanswered.map((request) => request.id).join(", ")}`)
  const observed = await readJSONL(join(repo, ".eval/observed.jsonl"))
  return {
    steps: requests.map((request) => request.id),
    terminal,
    requirements: events.filter((event) => event.type.startsWith("requirement.")).map((event) => event.type === "requirement.passed"),
    command_evidence: observed.filter((event) => event.kind === "run_command"),
    accepted_agent_tasks: requests.filter((request) => request.kind === "agent_task" && completed.has(request.id)).length,
    accepted_user_answers: requests.filter((request) => request.kind === "ask_user" && completed.has(request.id)).length,
    response_rejections: rejected.map((event) => event.data),
    event_counts: Object.fromEntries([...new Set(events.map((event) => event.type))].sort().map((type) => [type, events.filter((event) => event.type === type).length])),
  }
}

function same(a, b) { return JSON.stringify(a) === JSON.stringify(b) }

function scoreAgainstExpected(observed, expected) {
  const failures = []
  if (!same(observed.steps, expected.steps)) failures.push(`steps ${JSON.stringify(observed.steps)} != ${JSON.stringify(expected.steps)}`)
  if (observed.terminal !== expected.terminal) failures.push(`terminal ${observed.terminal} != ${expected.terminal}`)
  const commandSteps = observed.steps.filter((step) => ["test-package", "publish-package", "verify-package"].includes(step))
  if (!same(observed.command_evidence.map((event) => event.id), commandSteps)) failures.push("actual command evidence does not match the workflow trace")
  if (observed.response_rejections?.length) failures.push(`Yield rejected ${observed.response_rejections.length} response(s)`)
  return failures
}

async function main() {
  const allCases = JSON.parse(await readFile(join(agentRoot, "cases.json"), "utf8"))
  const cases = selectedCase ? allCases.filter((testCase) => testCase.id === selectedCase) : allCases
  if (!cases.length) throw new Error(`unknown case: ${selectedCase}`)
  const session = await mkdtemp(join(tmpdir(), "yield-agent-eval-"))
  const runStamp = new Date().toISOString().replaceAll(":", "-")
  const evidenceRoot = join(runsRoot, runStamp)
  const yskill = join(session, "yskill")
  command("go", ["build", "-o", yskill, "./cmd/yskill"])
  const authHome = await prepareAuthHome(session)
  const runs = []
  try {
    for (let index = 0; index < repeat; index++) {
      for (const testCase of cases) {
        for (const arm of arms) {
          process.stdout.write(`running ${testCase.id} / ${arm} / ${index + 1}\n`)
          const repo = await prepareRepo(session, arm, testCase, yskill, index + 1)
          const evidenceDir = join(evidenceRoot, testCase.id, arm, String(index + 1))
          const execution = await runCodex(repo, authHome, evidenceDir)
          const observed = arm === "long" ? await scoreLong(repo) : await scoreYield(repo)
          const failures = scoreAgainstExpected(observed, testCase.expected)
          await cp(join(repo, ".eval/observed.jsonl"), join(evidenceDir, "observed.jsonl"))
          if (arm === "yield") await cp(join(repo, "skills/release/.yield/runs"), join(evidenceDir, "yield-runs"), { recursive: true })
          runs.push({ case: testCase.id, arm, repeat: index + 1, passed: failures.length === 0, failures, ...execution, ...observed })
          process.stdout.write(`${failures.length ? "failed" : "passed"} ${testCase.id} / ${arm}\n`)
        }
      }
    }
  } finally {
    await rm(session, { recursive: true, force: true })
  }

  const comparisons = []
  for (let index = 1; index <= repeat; index++) {
    for (const testCase of cases) {
      const long = runs.find((run) => run.case === testCase.id && run.arm === "long" && run.repeat === index)
      const yieldRun = runs.find((run) => run.case === testCase.id && run.arm === "yield" && run.repeat === index)
      if (!long || !yieldRun) continue
      const equivalent = long.passed && yieldRun.passed && same(long.steps, yieldRun.steps) &&
        long.terminal === yieldRun.terminal && same(long.requirements, yieldRun.requirements) &&
        long.accepted_agent_tasks === yieldRun.accepted_agent_tasks &&
        long.accepted_user_answers === yieldRun.accepted_user_answers
      comparisons.push({
        case: testCase.id,
        repeat: index,
        equivalent,
        long_steps: long.steps,
        yield_steps: yieldRun.steps,
        long_terminal: long.terminal,
        yield_terminal: yieldRun.terminal,
        long_requirements: long.requirements,
        yield_requirements: yieldRun.requirements,
        accepted_agent_tasks: { long: long.accepted_agent_tasks, yield: yieldRun.accepted_agent_tasks },
        accepted_user_answers: { long: long.accepted_user_answers, yield: yieldRun.accepted_user_answers },
      })
    }
  }
  const result = {
    schema_version: 1,
    methodology_version: "agent-equivalence-v1",
    generated_at: new Date().toISOString(),
    source_hash: await sourceHash(),
    status: runs.every((run) => run.passed) && comparisons.every((comparison) => comparison.equivalent) ? "passed" : "failed",
    agent: { product: "Codex CLI", cli_version: command("codex", ["--version"]), model, reasoning },
    coverage: { cases: cases.length, arms, repeats: repeat, agent_runs: runs.length },
    runs,
    equivalence: { passed: comparisons.filter((comparison) => comparison.equivalent).length, total: comparisons.length, comparisons },
    claim_boundary: {
      summary: "The same Codex model followed the owned long skill and the thin-skill-plus-Yield workflow with the same tested step order and final status.",
      exclusions: [
        "This does not measure general coding-agent quality.",
        "This does not claim Yield is better than a long skill.",
        "This covers the declared cases, not every possible workflow.",
      ],
    },
  }
  if (process.argv.includes("--write")) await writeFile(join(evalRoot, "results/latest-agent.json"), JSON.stringify(result, null, 2) + "\n")
  process.stdout.write(JSON.stringify({ status: result.status, coverage: result.coverage, equivalence: result.equivalence, evidence: relative(evalRoot, evidenceRoot) }, null, 2) + "\n")
  if (result.status !== "passed") process.exitCode = 1
}

await main()
