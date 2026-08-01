import test from "node:test";
import assert from "node:assert/strict";
import { resolveRuntime, runtimePackage } from "./runtime.mjs";

const cases = [
  ["darwin", "x64", "@operatorstack/yield-darwin-amd64"],
  ["darwin", "arm64", "@operatorstack/yield-darwin-arm64"],
  ["linux", "x64", "@operatorstack/yield-linux-amd64"],
  ["linux", "arm64", "@operatorstack/yield-linux-arm64"],
  ["win32", "x64", "@operatorstack/yield-windows-amd64"],
  ["win32", "arm64", "@operatorstack/yield-windows-arm64"],
];

test("selects the exact runtime package for every supported target", () => {
  for (const [platform, arch, expected] of cases) {
    assert.equal(runtimePackage(platform, arch), expected);
    assert.equal(resolveRuntime({ platform, arch, resolve: (name) => `/packages/${name}` }), `/packages/${expected}`);
  }
});

test("rejects unsupported targets", () => {
  assert.throws(() => runtimePackage("freebsd", "x64"), /does not provide a runtime/);
});

test("does not fall back when the selected package is missing", () => {
  assert.throws(
    () => resolveRuntime({ platform: "linux", arch: "x64", resolve: () => { throw new Error("missing"); } }),
    /Reinstall @operatorstack\/yield/,
  );
});
