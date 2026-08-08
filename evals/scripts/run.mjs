import { createHash } from "node:crypto"
import { spawnSync } from "node:child_process"
import { mkdtemp, readdir, readFile, rm, stat, writeFile } from "node:fs/promises"
import { tmpdir } from "node:os"
import { dirname, join, relative, resolve } from "node:path"
import { fileURLToPath } from "node:url"

const evalRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..")
const yieldRoot = resolve(evalRoot, "..")
const libraryRoot = join(yieldRoot, "examples/library")
const languages = ["typescript", "python", "go", "rust"]
const runtimeCases = [
  [
    "resume-complete",
    "./internal/engine",
    "TestEndToEndRunResumeComplete",
    "a recorded response advances the run to completion",
  ],
  [
    "response-lock",
    "./internal/engine",
    "TestConcurrentIdenticalResumeCommitsOnce",
    "concurrent identical responses create one completion event",
  ],
  [
    "response-recovery",
    "./internal/engine",
    "TestRespondRecoveryRejectsDifferentCommittedContent",
    "an exact committed response recovers and different content is refused",
  ],
  [
    "ask-user-options",
    "./internal/conformance",
    "TestGuardRefusals",
    "every SDK rejects an answer outside the declared options",
  ],
  [
    "deterministic-replay",
    "./internal/engine",
    "TestReplayIsDeterministic",
    "the saved log returns to the same next step",
  ],
  [
    "replay-divergence",
    "./internal/engine",
    "TestReplayDivergenceFailsLoudly",
    "changed behavior stops replay instead of reusing the wrong result",
  ],
  [
    "requirement-block",
    "./internal/engine",
    "TestFailedRequirementBlocksRun",
    "a failed rule ends the run as blocked",
  ],
  [
    "source-change",
    "./internal/engine",
    "TestDigestMismatchRefusedThenMigrates",
    "changed source is refused until the user accepts the change",
  ],
]
const workflowRuntimeVersion = "0.2.0"
const excludedDirectories = new Set([
  ".git",
  ".yield",
  "node_modules",
  "runs",
  "raw",
  "artifacts",
  "target",
  "build",
  "dist",
  "__pycache__",
])
const digestRoots = [
  "cmd/yskill",
  "internal",
  "sdk",
  "examples/library",
  "evals/scripts",
  "evals/package.json",
]

function execute(command, args, cwd = yieldRoot) {
  const result = spawnSync(command, args, { cwd, encoding: "utf8", env: process.env })
  if (result.status !== 0) {
    const detail = [result.stdout, result.stderr].filter(Boolean).join("\n").trim()
    throw new Error(`${command} ${args.join(" ")} failed${detail ? `:\n${detail}` : ""}`)
  }
  return result.stdout.trim()
}

async function filesUnder(path) {
  const info = await stat(path)
  if (info.isFile()) return [path]
  const files = []
  for (const entry of await readdir(path, { withFileTypes: true })) {
    if (entry.isDirectory() && excludedDirectories.has(entry.name)) continue
    files.push(...(await filesUnder(join(path, entry.name))))
  }
  return files
}

async function sourceDigest() {
  const files = []
  for (const root of digestRoots) files.push(...(await filesUnder(join(yieldRoot, root))))
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

async function workflowCases(yskill) {
  const catalog = JSON.parse(await readFile(join(libraryRoot, "catalog.json"), "utf8"))
  const cases = []
  for (const language of languages) {
    const languageRoot = join(libraryRoot, language)
    for (const pattern of catalog) {
      const skill = join(languageRoot, pattern.slug)
      const output = execute(yskill, ["test", skill])
      if (!/reached completed$/.test(output))
        throw new Error(`${language}/${pattern.slug}: missing completed result`)
      cases.push({
        id: `${language}/${pattern.slug}`,
        language,
        pattern: pattern.slug,
        status: "passed",
      })
      await rm(join(skill, ".yield"), { recursive: true, force: true })
    }
  }
  return { catalog, cases }
}

function evaluateRuntime() {
  return runtimeCases.map(([id, packagePath, test, assertion]) => {
    execute("go", ["test", packagePath, "-run", `^${test}$`, "-count=1"])
    return { id, package: packagePath, test, assertion, status: "passed" }
  })
}

async function evaluate() {
  const temporary = await mkdtemp(join(tmpdir(), "yield-evals-"))
  try {
    const yskill = join(temporary, "yskill")
    execute("go", [
      "build",
      "-ldflags",
      `-X main.version=${workflowRuntimeVersion}`,
      "-o",
      yskill,
      "./cmd/yskill",
    ])
    const { catalog, cases } = await workflowCases(yskill)
    const invariants = evaluateRuntime()
    return {
      schema_version: 2,
      methodology_version: "1.1",
      generated_at: new Date().toISOString(),
      source_digest: await sourceDigest(),
      status: "passed",
      workflow_conformance: {
        passed: cases.length,
        total: cases.length,
        patterns: catalog.length,
        languages,
        cases,
      },
      runtime_invariants: {
        passed: invariants.length,
        total: invariants.length,
        cases: invariants,
      },
      claim_boundary: {
        summary: "Yield executes the tested workflows and passes the tested runtime checks.",
        compares_tools: false,
        tests_agent_judgment: false,
        exclusions: [
          "No claim that Yield is better than prompts or another tool.",
          "Fixed test data replaces agent and human judgment.",
          "Illustrative commands do not establish production safety for a real project.",
        ],
      },
    }
  } finally {
    await rm(temporary, { recursive: true, force: true })
  }
}

const result = await evaluate()
const latestPath = join(evalRoot, "results/latest.json")
if (process.argv.includes("--write")) {
  await writeFile(latestPath, JSON.stringify(result, null, 2) + "\n")
  console.log(`wrote ${relative(process.cwd(), latestPath)}`)
} else if (process.argv.includes("--check")) {
  const published = JSON.parse(await readFile(latestPath, "utf8"))
  for (const field of ["schema_version", "methodology_version", "source_digest", "status"]) {
    if (published[field] !== result[field]) throw new Error(`published ${field} is stale`)
  }
  for (const field of ["workflow_conformance", "runtime_invariants", "claim_boundary"]) {
    if (JSON.stringify(published[field]) !== JSON.stringify(result[field]))
      throw new Error(`published ${field} is stale`)
  }
  console.log(
    `passed ${result.workflow_conformance.passed}/${result.workflow_conformance.total} workflow tests`,
  )
  console.log(
    `passed ${result.runtime_invariants.passed}/${result.runtime_invariants.total} runtime checks`,
  )
} else {
  console.log(JSON.stringify(result, null, 2))
}
