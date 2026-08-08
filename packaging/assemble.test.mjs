import test from "node:test"
import assert from "node:assert/strict"
import { execFileSync } from "node:child_process"
import { access, mkdtemp, mkdir, readFile, rm, stat, writeFile } from "node:fs/promises"
import { join } from "node:path"
import { tmpdir } from "node:os"
import { assemble, isPackageVersion } from "./assemble.mjs"
import { binaryName, npmPackage, rustPackage, targets } from "./targets.mjs"

const homepage = "https://yield.operatorstack.systems/"

test("accepts stable and exact Yield canary versions", () => {
  assert.equal(isPackageVersion("1.2.3"), true)
  assert.equal(isPackageVersion("0.0.0-canary.20260807104031.b081bae38282"), true)
  assert.equal(isPackageVersion("1.2.3-beta.1"), false)
  assert.equal(isPackageVersion("0.0.0-canary.latest.b081bae38282"), false)
  assert.equal(isPackageVersion("v1.2.3"), false)
})

test("assembles two public npm packages and six matching npm and Python runtimes", async (t) => {
  const root = await mkdtemp(join(tmpdir(), "yield-assemble-"))
  t.after(() => rm(root, { recursive: true, force: true }))

  const binaries = join(root, "bin")
  const output = join(root, "packages")
  await mkdir(binaries)
  for (const target of targets) {
    await writeFile(join(binaries, binaryName(target)), `runtime:${target.id}`)
  }

  await assemble({ version: "1.2.3", binaries, output })
  const readJson = async (path) => JSON.parse(await readFile(path, "utf8"))
  const main = await readJson(join(output, "npm/yield/package.json"))
  const initializer = await readJson(join(output, "npm/create-yield/package.json"))

  assert.equal(main.name, "@operatorstack/yield")
  assert.equal(main.version, "1.2.3")
  assert.equal(main.homepage, homepage)
  assert.deepEqual(main.publishConfig, {
    access: "public",
    provenance: true,
    registry: "https://registry.npmjs.org/",
  })
  assert.deepEqual(
    main.optionalDependencies,
    Object.fromEntries(targets.map((target) => [npmPackage(target), "1.2.3"])),
  )
  assert.equal(initializer.name, "@operatorstack/create-yield")
  assert.equal(initializer.version, "1.2.3")
  assert.equal(initializer.dependencies["@operatorstack/yield"], "1.2.3")
  assert.equal(initializer.publishConfig.provenance, true)
  assert.match(await readFile(join(output, "npm/create-yield/LICENSE"), "utf8"), /MIT License/)
  assert.match(
    await readFile(join(output, "npm/create-yield/bin/create-yield.mjs"), "utf8"),
    /bootstrap/,
  )
  const initializerPack = JSON.parse(
    execFileSync("npm", ["pack", "--dry-run", "--json"], {
      cwd: join(output, "npm/create-yield"),
      encoding: "utf8",
    }),
  )
  assert.equal(
    initializerPack[0].files.find((file) => file.path === "bin/create-yield.mjs").mode,
    0o755,
  )
  const assembledReadme = await readFile(join(output, "npm/yield/README.md"), "utf8")
  const repositoryReadme = await readFile(join(import.meta.dirname, "../README.md"), "utf8")
  assert.match(repositoryReadme, /pypi\.org\/project\/yieldskill/)
  assert.match(assembledReadme, /npmjs\.com\/package\/@operatorstack\/yield/)
  assert.doesNotMatch(assembledReadme, /pypi\.org|PyPI version|npm-exclude/)
  assert.equal(
    await readFile(join(output, "npm/yield/assets/yield-mark.svg"), "utf8"),
    await readFile(join(import.meta.dirname, "../assets/yield-mark.svg"), "utf8"),
  )
  assert.ok(main.files.includes("assets"))
  assert.match(assembledReadme, /<h1 align="center">Yield<\/h1>/)
  assert.match(await readFile(join(output, "npm/yield/LICENSE"), "utf8"), /MIT License/)
  await assert.rejects(access(join(output, "npm/yield/skills/release-yield")), { code: "ENOENT" })
  await assert.rejects(access(join(output, "npm/yield/.agents")), { code: "ENOENT" })
  await assert.rejects(access(join(output, "npm/yield/.cursor")), { code: "ENOENT" })
  await assert.rejects(access(join(output, "npm/yield/.claude")), { code: "ENOENT" })

  for (const target of targets) {
    const runtime = await readJson(join(output, `npm/${target.id}/package.json`))
    assert.equal(runtime.name, npmPackage(target))
    assert.equal(runtime.version, "1.2.3")
    assert.equal(runtime.homepage, homepage)
    assert.deepEqual(runtime.os, [target.nodeOs])
    assert.deepEqual(runtime.cpu, [target.nodeCpu])
    assert.deepEqual(runtime.bin, {
      "yskill-runtime": `./${target.goos === "windows" ? "yskill.exe" : "yskill"}`,
    })
    assert.equal(runtime.publishConfig.provenance, true)
    const packed = JSON.parse(
      execFileSync("npm", ["pack", "--dry-run", "--json"], {
        cwd: join(output, `npm/${target.id}`),
        encoding: "utf8",
      }),
    )
    assert.equal(
      packed[0].files.find(
        (file) => file.path === (target.goos === "windows" ? "yskill.exe" : "yskill"),
      ).mode,
      0o755,
    )
    assert.match(await readFile(join(output, `npm/${target.id}/LICENSE`), "utf8"), /MIT License/)

    const pythonRoot = join(output, `python/${target.id}`)
    assert.match(await readFile(join(pythonRoot, "pyproject.toml"), "utf8"), /version = "1\.2\.3"/)
    assert.match(await readFile(join(pythonRoot, "setup.py"), "utf8"), new RegExp(target.pythonTag))
    assert.match(await readFile(join(pythonRoot, "LICENSE"), "utf8"), /MIT License/)
    const pythonRuntime = target.goos === "windows" ? "yskill.exe" : "yskill"
    assert.equal(
      await readFile(join(pythonRoot, "yieldskill/_runtime", pythonRuntime), "utf8"),
      `runtime:${target.id}`,
    )

    const rustRoot = join(output, `rust/runtime/${target.id}`)
    const rustManifest = await readFile(join(rustRoot, "Cargo.toml"), "utf8")
    assert.match(rustManifest, new RegExp(`name = "${rustPackage(target)}"`))
    assert.match(rustManifest, /version = "1\.2\.3"/)
    assert.match(rustManifest, /readme = "README\.md"/)
    assert.doesNotMatch(rustManifest, /registry\s*=/)
    assert.match(
      await readFile(join(rustRoot, "README.md"), "utf8"),
      /installed automatically by `yieldskill`/,
    )
    assert.match(await readFile(join(rustRoot, "LICENSE"), "utf8"), /MIT License/)
    const rustRuntime = target.goos === "windows" ? "yskill.exe" : "yskill"
    assert.equal((await stat(join(rustRoot, "runtime", rustRuntime))).mode & 0o111, 0)
  }

  const rustMain = join(output, "rust/yieldskill")
  const rustMainManifest = await readFile(join(rustMain, "Cargo.toml"), "utf8")
  assert.match(rustMainManifest, /name = "yieldskill"/)
  assert.match(rustMainManifest, /version = "1\.2\.3"/)
  assert.doesNotMatch(rustMainManifest, /registry\s*=/)
  for (const target of targets) {
    assert.match(
      rustMainManifest,
      new RegExp(`${rustPackage(target)} = \\{ version = "=1\\.2\\.3" \\}`),
    )
  }
  const rustReadme = await readFile(join(rustMain, "README.md"), "utf8")
  assert.match(rustReadme, /crates\.io\/crates\/yieldskill/)
  assert.doesNotMatch(rustReadme, /npmjs\.com|pypi\.org/)
  assert.match(await readFile(join(rustMain, "LICENSE"), "utf8"), /MIT License/)
  await assert.rejects(access(join(output, "rust/.cargo/config.toml")), { code: "ENOENT" })
})
