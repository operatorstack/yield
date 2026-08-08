import { execFileSync, spawnSync } from "node:child_process"

const mode = process.argv[2]
if (mode !== "--check" && mode !== "--write") {
  throw new Error("usage: node scripts/format.mjs --check|--write")
}

const tracked = execFileSync("git", ["ls-files", "--cached", "--others", "--exclude-standard"], {
  encoding: "utf8",
})
  .split("\n")
  .filter(Boolean)

const filesWith = (...extensions) =>
  tracked.filter((path) => extensions.some((extension) => path.endsWith(extension)))

// These source bytes are bound to the committed 12-session agent receipt.
// Format them only when that evaluation is intentionally rerun.
const byteBoundAgentSource = (path) =>
  path.startsWith("evals/agent/") ||
  path.startsWith("internal/") ||
  path === "sdk/typescript/src/index.ts"

const generatedLibrarySource = (path) =>
  /examples\/library\/(go|python|rust|typescript)\//.test(path)

const prettierFiles = filesWith(
  ".cjs",
  ".js",
  ".json",
  ".md",
  ".mdx",
  ".mjs",
  ".ts",
  ".tsx",
  ".yaml",
  ".yml",
)
const pythonFiles = filesWith(".py", ".pyi").filter(
  (path) => !byteBoundAgentSource(path) && !generatedLibrarySource(path),
)
const goFiles = filesWith(".go").filter((path) => !generatedLibrarySource(path))
const rustFiles = filesWith(".rs").filter((path) => !generatedLibrarySource(path))
const shellFiles = filesWith(".bash", ".sh")
const tomlFiles = filesWith(".toml")

function run(label, command, args) {
  process.stdout.write(`${label}\n`)
  const result = spawnSync(command, args, {
    encoding: "utf8",
    maxBuffer: 64 * 1024 * 1024,
    stdio: ["inherit", "pipe", "pipe"],
  })
  if (result.stdout) process.stdout.write(result.stdout)
  if (result.stderr) process.stderr.write(result.stderr)
  if (result.error) {
    throw new Error(`${label} could not start: ${result.error.message}`)
  }
  if (result.status !== 0) {
    throw new Error(`${label} failed with exit code ${result.status}`)
  }
}

run("Prettier", "npm", [
  "exec",
  "--",
  "prettier",
  mode === "--check" ? "--check" : "--write",
  ...prettierFiles,
])
run("Ruff", "uvx", [
  "--from",
  "ruff==0.16.2",
  "ruff",
  "format",
  ...(mode === "--check" ? ["--check"] : []),
  ...pythonFiles,
])

if (mode === "--check") {
  const result = execFileSync("gofmt", ["-l", ...goFiles], { encoding: "utf8" })
  if (result.trim()) throw new Error(`gofmt found unformatted files:\n${result}`)
  process.stdout.write("gofmt\n")
} else {
  run("gofmt", "gofmt", ["-w", ...goFiles])
}

run("rustfmt", "rustfmt", [
  "--edition",
  "2021",
  ...(mode === "--check" ? ["--check"] : []),
  ...rustFiles,
])
run("shfmt", "go", [
  "run",
  "mvdan.cc/sh/v3/cmd/shfmt@v3.13.1",
  mode === "--check" ? "-d" : "-w",
  ...shellFiles,
])
run("Taplo", "npm", [
  "exec",
  "--",
  "taplo",
  "format",
  ...(mode === "--check" ? ["--check"] : []),
  ...tomlFiles,
])

process.stdout.write(`format ${mode === "--check" ? "check" : "write"} passed\n`)
