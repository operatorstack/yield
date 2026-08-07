import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { assemble, isPackageVersion } from "./assemble.mjs";
import { binaryName, npmPackage, targets } from "./targets.mjs";

const homepage = "https://yield.operatorstack.systems/";

test("accepts stable and exact Yield canary versions", () => {
  assert.equal(isPackageVersion("1.2.3"), true);
  assert.equal(isPackageVersion("0.0.0-canary.20260807104031.b081bae38282"), true);
  assert.equal(isPackageVersion("1.2.3-beta.1"), false);
  assert.equal(isPackageVersion("0.0.0-canary.latest.b081bae38282"), false);
  assert.equal(isPackageVersion("v1.2.3"), false);
});

test("assembles one public npm package and six matching runtimes", async (t) => {
  const root = await mkdtemp(join(tmpdir(), "yield-assemble-"));
  t.after(() => rm(root, { recursive: true, force: true }));

  const binaries = join(root, "bin");
  const output = join(root, "packages");
  await mkdir(binaries);
  for (const target of targets) {
    await writeFile(join(binaries, binaryName(target)), `runtime:${target.id}`);
  }

  await assemble({ version: "1.2.3", binaries, output });
  const readJson = async (path) => JSON.parse(await readFile(path, "utf8"));
  const main = await readJson(join(output, "npm/yield/package.json"));

  assert.equal(main.name, "@operatorstack/yield");
  assert.equal(main.version, "1.2.3");
  assert.equal(main.homepage, homepage);
  assert.deepEqual(main.publishConfig, {
    access: "public",
    provenance: true,
    registry: "https://registry.npmjs.org/",
  });
  assert.deepEqual(
    main.optionalDependencies,
    Object.fromEntries(targets.map((target) => [npmPackage(target), "1.2.3"])),
  );
  const assembledReadme = await readFile(join(output, "npm/yield/README.md"), "utf8");
  assert.equal(assembledReadme, await readFile(join(import.meta.dirname, "../README.md"), "utf8"));
  assert.match(assembledReadme, /<h1 align="center">Yield<\/h1>/);
  assert.match(await readFile(join(output, "npm/yield/LICENSE"), "utf8"), /MIT License/);

  for (const target of targets) {
    const runtime = await readJson(join(output, `npm/${target.id}/package.json`));
    assert.equal(runtime.name, npmPackage(target));
    assert.equal(runtime.version, "1.2.3");
    assert.equal(runtime.homepage, homepage);
    assert.deepEqual(runtime.os, [target.nodeOs]);
    assert.deepEqual(runtime.cpu, [target.nodeCpu]);
    assert.equal(runtime.publishConfig.provenance, true);
    assert.match(await readFile(join(output, `npm/${target.id}/LICENSE`), "utf8"), /MIT License/);
  }
});
