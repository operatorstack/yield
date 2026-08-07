import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, readdir, rm, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { compareRelease, expectedWheelNames, fetchPyPIRelease, inspectLocalRelease, prepareUpload } from "./pypi-release.mjs";

async function fixture(t, version = "1.2.3") {
  const root = await mkdtemp(join(tmpdir(), "yield-pypi-"));
  t.after(() => rm(root, { recursive: true, force: true }));
  const dist = join(root, "dist");
  const upload = join(root, "upload");
  const { mkdir } = await import("node:fs/promises");
  await mkdir(dist);
  for (const name of expectedWheelNames(version)) await writeFile(join(dist, name), `wheel:${name}`);
  return { dist, upload, version };
}

test("requires the complete six-wheel release unit", async (t) => {
  const { dist, version } = await fixture(t);
  assert.equal((await inspectLocalRelease(dist, version)).length, 6);
  await rm(join(dist, expectedWheelNames(version)[0]));
  await assert.rejects(inspectLocalRelease(dist, version), /wheel set mismatch/);
});

test("accepts matching remote files and returns only missing wheels", async (t) => {
  const { dist, version } = await fixture(t);
  const local = await inspectLocalRelease(dist, version);
  assert.deepEqual(compareRelease(local, local.slice(0, 2)), local.slice(2));
  assert.throws(() => compareRelease(local, [{ filename: local[0].filename, sha256: "wrong" }]), /hash mismatch/);
  assert.throws(() => compareRelease(local, [{ filename: "yieldskill-1.2.3.tar.gz", sha256: "x" }]), /unexpected remote file/);
});

test("prepares an idempotent upload directory", async (t) => {
  const { dist, upload, version } = await fixture(t);
  const local = await inspectLocalRelease(dist, version);
  const partial = await prepareUpload({ dist, upload, version, remote: local.slice(0, 4) });
  assert.equal(partial.missing.length, 2);
  assert.deepEqual((await readdir(upload)).sort(), partial.missing.map(({ filename }) => filename).sort());
  const complete = await prepareUpload({ dist, upload, version, remote: local });
  assert.equal(complete.missing.length, 0);
  assert.deepEqual(await readdir(upload), []);
});

test("treats a missing PyPI project as an empty release", async () => {
  const files = await fetchPyPIRelease("1.2.3", async () => ({ status: 404, ok: false }));
  assert.deepEqual(files, []);
});

test("reads PyPI SHA-256 receipts", async () => {
  const files = await fetchPyPIRelease("1.2.3", async () => ({
    status: 200,
    ok: true,
    json: async () => ({ urls: [{ filename: "a.whl", digests: { sha256: "abc" } }] }),
  }));
  assert.deepEqual(files, [{ filename: "a.whl", sha256: "abc" }]);
});
