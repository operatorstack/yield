import test from "node:test";
import assert from "node:assert/strict";
import { modulePath, validateModuleReceipt, verifyGoRelease } from "./go-release.mjs";

const sourceSha = "a".repeat(40);
const goPlatform = `${process.platform === "win32" ? "windows" : process.platform}/${process.arch === "x64" ? "amd64" : process.arch}`;

function receipt(version = "1.2.3") {
  return {
    Path: modulePath,
    Version: `v${version}`,
    Origin: { VCS: "git", URL: `https://${modulePath}`, Hash: sourceSha },
  };
}

test("binds the public Go module to its immutable tag source", () => {
  assert.equal(validateModuleReceipt(receipt(), { version: "1.2.3", sourceSha }).Version, "v1.2.3");
  assert.throws(
    () => validateModuleReceipt({ ...receipt(), Origin: { ...receipt().Origin, Hash: "b".repeat(40) } }, { version: "1.2.3", sourceSha }),
    /does not match the release tag/,
  );
});

test("verifies proxy discovery and a fresh command install", async () => {
  const calls = [];
  const result = await verifyGoRelease({
    version: "1.2.3",
    sourceSha,
    execImpl: async (file, args, options) => {
      calls.push({ file, args, options });
      if (args[0] === "list") return { stdout: JSON.stringify(receipt()) };
      if (file === "go") return { stdout: "" };
      return { stdout: `yskill 1.2.3 ${goPlatform}\n` };
    },
  });
  assert.equal(result.module, modulePath);
  assert.deepEqual(calls[0].args, ["list", "-m", "-json", `${modulePath}@v1.2.3`]);
  assert.deepEqual(calls[1].args, ["install", `${modulePath}/cmd/yskill@v1.2.3`]);
  assert.equal(calls[0].options.env.GOPROXY, "https://proxy.golang.org");
  assert.equal(calls[0].options.env.GOSUMDB, "sum.golang.org");
  assert.equal(calls[0].options.env.GOWORK, "off");
});

test("retries a proxy miss without accepting a partial release", async () => {
  let lists = 0;
  let delays = 0;
  await verifyGoRelease({
    version: "1.2.3",
    sourceSha,
    attempts: 2,
    delayMs: 1,
    delay: async () => { delays += 1; },
    execImpl: async (file, args) => {
      if (args[0] === "list") {
        lists += 1;
        if (lists === 1) throw new Error("module not found");
        return { stdout: JSON.stringify(receipt()) };
      }
      if (file === "go") return { stdout: "" };
      return { stdout: `yskill 1.2.3 ${goPlatform}\n` };
    },
  });
  assert.equal(lists, 2);
  assert.equal(delays, 1);
});
