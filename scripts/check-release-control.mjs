#!/usr/bin/env node
import { readdir, readFile, stat } from "node:fs/promises"
import { resolve } from "node:path"
import process from "node:process"
import { pathToFileURL } from "node:url"
import { parse } from "yaml"

const SHA_REF = /^[^\s@]+@[0-9a-f]{40}$/

function expect(condition, message) {
  if (!condition) throw new Error(message)
}

function usesIn(value, found = []) {
  if (Array.isArray(value)) {
    for (const item of value) usesIn(item, found)
  } else if (value && typeof value === "object") {
    for (const [key, item] of Object.entries(value)) {
      if (key === "uses" && typeof item === "string") found.push(item)
      else usesIn(item, found)
    }
  }
  return found
}

async function exists(path) {
  return stat(path).then(
    () => true,
    () => false,
  )
}

export async function checkReleaseControl(root = resolve(import.meta.dirname, "..")) {
  const workflowDir = resolve(root, ".github/workflows")
  const names = (await readdir(workflowDir)).filter(
    (name) => name.endsWith(".yml") || name.endsWith(".yaml"),
  )
  const workflows = {}
  const raw = {}
  for (const name of names) {
    raw[name] = await readFile(resolve(workflowDir, name), "utf8")
    workflows[name] = parse(raw[name])
    for (const action of usesIn(workflows[name])) {
      expect(
        action.startsWith("./") || SHA_REF.test(action),
        `${name}: action is not pinned to a full commit SHA: ${action}`,
      )
    }
  }

  expect(
    !(await exists(resolve(root, "UPSTREAM.json"))),
    "UPSTREAM.json must be removed after graduation",
  )
  expect(
    !names.includes("sync-upstream.yml"),
    "projection sync workflow must be removed after graduation",
  )

  const verify = workflows["verify.yml"]
  const validationJobs = [
    "format",
    "go",
    "release",
    "selfhost",
    "typescript",
    "python",
    "rust",
    "conformance",
    "examples",
  ]
  expect(verify, "verify.yml is required")
  expect(verify.on?.pull_request !== undefined, "verification must run on every pull request")
  expect(
    validationJobs.every((name) => verify.jobs?.[name]),
    "verification must expose every SDK and package boundary",
  )
  expect(
    JSON.stringify([...(verify.jobs?.validate?.needs ?? [])].sort()) ===
      JSON.stringify([...validationJobs].sort()),
    "the final validation gate must depend on every visible validation job",
  )
  expect(
    verify.jobs?.validate?.name === "Release authority and full validation",
    "the protected validation context must remain stable",
  )

  const release = workflows["release.yml"]
  expect(release, "release.yml is required")
  expect(
    JSON.stringify(Object.keys(release.on ?? {}).sort()) === JSON.stringify(["workflow_dispatch"]),
    "stable release must be dispatch-only",
  )
  expect(release.permissions?.contents === "read", "release planning must be read-only")
  expect(
    release.jobs?.release?.permissions?.contents === "write" &&
      release.jobs?.release?.permissions?.actions === "write",
    "the controller needs tag and publisher-dispatch authority",
  )
  expect(
    release.jobs?.release?.environment === "release-control",
    "release authorization must use the protected release-control environment",
  )
  expect(
    raw["release.yml"].includes("repos/$GITHUB_REPOSITORY/git/refs"),
    "release controller must create tags with its scoped GitHub token",
  )
  expect(
    !raw["release.yml"].includes('git push origin "refs/tags/$TAG"'),
    "release controller must not push tags without explicit authentication",
  )
  expect(raw["release.yml"].includes("--draft"), "release controller must create a draft release")
  expect(
    !raw["release.yml"].includes("--draft=false"),
    "release controller must not finalize its own release",
  )

  const publisher = workflows["npm-publish.yml"]
  expect(
    publisher,
    "npm-publish.yml is required because both registry trust policies bind to this workflow identity",
  )
  expect(
    publisher.permissions?.contents === "read",
    "package publisher must default to read-only source access",
  )
  expect(publisher.on?.push?.branches?.includes("main"), "npm canary must follow public main")
  expect(
    publisher.on?.workflow_dispatch?.inputs?.version?.required === true,
    "stable packages must require an exact version dispatch",
  )
  expect(
    publisher.jobs?.npm?.permissions?.contents === "read" &&
      publisher.jobs?.npm?.permissions?.["id-token"] === "write",
    "npm publisher must use read-only source plus OIDC",
  )
  expect(
    publisher.jobs?.pypi?.permissions?.contents === "read" &&
      publisher.jobs?.pypi?.permissions?.["id-token"] === "write",
    "PyPI publisher must use read-only source plus OIDC",
  )
  expect(
    publisher.jobs?.crates?.permissions?.contents === "read" &&
      publisher.jobs?.crates?.permissions?.["id-token"] === "write",
    "crates.io publisher must use read-only source plus OIDC",
  )
  expect(
    publisher.jobs?.pypi?.environment === "pypi-production",
    "stable PyPI publishing must use the protected pypi-production environment",
  )
  expect(
    publisher.jobs?.crates?.environment === "crates-production",
    "stable crates.io publishing must use the protected crates-production environment",
  )
  expect(
    publisher.jobs?.["selfhost-canary"]?.needs?.includes("npm"),
    "canary self-hosting must follow successful npm publication",
  )
  expect(
    raw["npm-publish.yml"].includes('"@operatorstack/yield@${VERSION}"'),
    "canary self-hosting must install the exact resolved version",
  )
  expect(
    !raw["npm-publish.yml"].includes("@operatorstack/yield@canary"),
    "canary self-hosting must not execute a floating dist-tag",
  )
  const pythonWheelStep = publisher.jobs?.build?.steps?.find(
    (step) => step.name === "Build Python wheels",
  )
  expect(
    pythonWheelStep?.if === "needs.resolve.outputs.channel == 'stable'",
    "PyPI wheels must be built only for stable PEP 440 versions",
  )
  expect(
    raw["npm-publish.yml"].indexOf("Publish platform runtimes") <
      raw["npm-publish.yml"].indexOf("Publish SDK and CLI"),
    "runtime packages must publish before the SDK package",
  )
  expect(
    raw["npm-publish.yml"].includes("@operatorstack/create-yield@${VERSION}"),
    "npm trusted publishing must include the initializer package",
  )
  expect(
    raw["npm-publish.yml"].indexOf("Publish SDK and CLI") <
      raw["npm-publish.yml"].indexOf("Publish npm initializer"),
    "the SDK package must publish before the initializer",
  )
  expect(
    raw["npm-publish.yml"].indexOf("Publish npm initializer") <
      raw["npm-publish.yml"].indexOf("Verify complete npm release unit"),
    "the initializer must publish before release-unit verification",
  )
  expect(
    raw["npm-publish.yml"].includes('npm create "@operatorstack/yield@${VERSION}"'),
    "the exact published initializer must pass a dry-run smoke test",
  )
  expect(
    raw["npm-publish.yml"].includes("pypa/gh-action-pypi-publish@"),
    "PyPI publishing must use the trusted-publishing action",
  )
  expect(
    raw["npm-publish.yml"].includes("rust-lang/crates-io-auth-action@"),
    "crates.io publishing must use the trusted-publishing action",
  )
  expect(
    raw["npm-publish.yml"].includes("chmod 0644 dist/packages/rust/runtime/*/runtime/*"),
    "Rust archives must normalize embedded runtime modes before artifact transport",
  )
  expect(
    raw["npm-publish.yml"].includes("SOURCE_DATE_EPOCH"),
    "Python wheels must bind timestamps to the immutable source revision",
  )
  expect(
    raw["npm-publish.yml"].includes(
      "name: crates-${{ needs.resolve.outputs.version }}-${{ needs.resolve.outputs.source_sha }}",
    ),
    "the crates publisher must upload an exact post-dependency receipt",
  )
  expect(
    raw["npm-publish.yml"].indexOf("rust/runtime/*") <
      raw["npm-publish.yml"].indexOf("rust/yieldskill"),
    "Rust runtime crates must publish before the SDK crate",
  )
  expect(
    !raw["npm-publish.yml"].includes("skip-existing"),
    "PyPI retries must verify hashes instead of blindly skipping existing files",
  )

  const finalizer = workflows["release-finalize.yml"]
  expect(
    finalizer?.permissions?.actions === "read" && finalizer.permissions?.contents === "read",
    "finalizer preflight must be read-only",
  )
  expect(
    finalizer.jobs?.finalize?.permissions?.actions === "read" &&
      finalizer.jobs?.finalize?.permissions?.contents === "write",
    "receipt-complete finalization needs artifact read access and contents:write",
  )
  expect(
    finalizer.jobs?.finalize?.needs === "resolve",
    "finalization must follow read-only tag resolution",
  )
  expect(
    raw["release-finalize.yml"].includes("github.event.workflow_run.event == 'workflow_dispatch'"),
    "automatic finalization must ignore canary publisher runs",
  )
  expect(
    raw["release-finalize.yml"].includes('select(.event == "workflow_dispatch")'),
    "finalization must select only stable publisher receipts",
  )
  expect(
    raw["release-finalize.yml"].includes("--draft=false"),
    "only the receipt finalizer may publish the GitHub release",
  )
  expect(
    raw["release-finalize.yml"].includes("npm-publish.yml"),
    "finalization must bind the combined publisher receipt",
  )
  expect(
    raw["release-finalize.yml"].includes("pypi-release.mjs verify"),
    "finalization must verify the PyPI wheel hashes",
  )
  expect(
    raw["release-finalize.yml"].includes("crates-release.mjs verify"),
    "finalization must verify the crates.io package hashes",
  )
  expect(
    raw["release-finalize.yml"].includes("go-release.mjs"),
    "finalization must verify the public Go module and command install",
  )
  expect(
    raw["release-finalize.yml"].includes('--source-sha "$SOURCE_SHA"'),
    "Go finalization must bind the module to the release source",
  )
  expect(
    raw["release-finalize.yml"].includes('--name "crates-${version}-${SOURCE_SHA}"'),
    "finalization must consume the publisher-produced crates receipt",
  )
  expect(
    raw["release.yml"].includes("gh workflow run npm-publish.yml"),
    "the release controller must dispatch the trusted-publishing event after tagging",
  )
  expect(
    !Object.values(raw).some((text) => text.includes("CRATES_BOOTSTRAP_TOKEN")),
    "crates.io publishing must not use a bootstrap token",
  )
  for (const [name, text] of Object.entries(raw)) {
    expect(
      !/NPM_TOKEN|NODE_AUTH_TOKEN|PYPI_TOKEN|secrets\.(npm|pypi)|password:/i.test(text),
      `${name}: long-lived registry credentials are forbidden`,
    )
  }

  return {
    workflows: names.length,
    externalActionsPinned: names
      .flatMap((name) => usesIn(workflows[name]))
      .filter((ref) => !ref.startsWith("./")).length,
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  checkReleaseControl()
    .then((result) =>
      console.log(
        `release-control: ${result.workflows} workflows and ${result.externalActionsPinned} pinned action references verified`,
      ),
    )
    .catch((error) => {
      console.error(`release-control: ${error.message}`)
      process.exit(1)
    })
}
