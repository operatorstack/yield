import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { resolve } from "node:path";

const root = resolve(import.meta.dirname, "..");

async function text(path) {
  return readFile(resolve(root, path), "utf8");
}

test("README release example matches the tested TypeScript source", async () => {
  const [readme, source] = await Promise.all([
    text("README.md"),
    text("examples/release-checklist/main.ts"),
  ]);

  const readmeMatch = readme.match(
    /<!-- release-example:start -->\s*```typescript\n([\s\S]*?)\n```\s*<!-- release-example:end -->/,
  );
  assert.ok(readmeMatch, "README release example markers are missing");

  const sourceMatch = source.match(
    /\/\/ README_EXAMPLE_START\n([\s\S]*?)\n\/\/ README_EXAMPLE_END/,
  );
  assert.ok(sourceMatch, "TypeScript release example markers are missing");

  const readmeProgram = readmeMatch[1]
    .replace(/^import \{ defineSkill \} from "@operatorstack\/yield";\n+/, "")
    .trim();
  assert.equal(readmeProgram, sourceMatch[1].trim());
});

test("README agent claims match the pinned registry", async () => {
  const [readme, registryText] = await Promise.all([
    text("README.md"),
    text("cmd/yskill/registry/agents.json"),
  ]);
  const registry = JSON.parse(registryText);
  const verified = registry.agents.filter((agent) => agent.tier === "verified");
  const registryBacked = registry.agents.filter((agent) => agent.tier === "registry");
  const normalized = readme.replace(/\s+/g, " ");

  assert.deepEqual(
    verified.map((agent) => agent.id).sort(),
    ["claude-code", "codex", "cursor"],
  );
  assert.match(normalized, /Verified with Cursor, Codex, and Claude Code\./);
  assert.match(
    normalized,
    new RegExp(`Registry-backed project paths are available for ${registryBacked.length} more coding agents\\.`),
  );
  assert.doesNotMatch(readme, /Agent Plugins and Yield/);
});

test("README uses the compact Yield mark", async () => {
  const readme = await text("README.md");
  assert.match(readme, /https:\/\/yield\.operatorstack\.systems\/favicon\.svg/);
  assert.doesNotMatch(readme, /apple-touch-icon\.png/);
});
