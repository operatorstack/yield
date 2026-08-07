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

test("README presents the workflow as four ordered steps", async () => {
  const readme = await text("README.md");
  const headings = [
    "### 1. Install Yield",
    "### 2. Create the workflow",
    "### 3. Test the workflow",
    "### 4. Register and use the skill",
  ];

  let previous = -1;
  for (const heading of headings) {
    const current = readme.indexOf(heading);
    assert.ok(current > previous, `${heading} is missing or out of order`);
    previous = current;
  }

  assert.match(readme, /npm exec -- yskill doctor skills\/release --test/);
  assert.match(readme, /npm exec -- yskill register skills\/release/);
  assert.match(readme, /Registration is the discovery step\./);
});

test("README adapter paths match every verified agent", async () => {
  const [readme, registryText] = await Promise.all([
    text("README.md"),
    text("cmd/yskill/registry/agents.json"),
  ]);
  const registry = JSON.parse(registryText);

  for (const agent of registry.agents.filter((entry) => entry.tier === "verified")) {
    const adapter = `${agent.project_dir}/release/SKILL.md`;
    assert.match(readme, new RegExp(adapter.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
  }

  assert.match(readme, /--agent cursor,codex,claude-code/);
  assert.match(readme, /If all three are selected/);
});

test("README uses the edge-cropped Yield mark", async () => {
  const [readme, mark] = await Promise.all([
    text("README.md"),
    text("assets/yield-mark.svg"),
  ]);
  assert.match(
    readme,
    /<img src="assets\/yield-mark\.svg" width="96" height="96" alt="Yield" \/>/,
  );
  assert.doesNotMatch(readme, /apple-touch-icon\.png/);
  assert.match(mark, /viewBox="0 0 60 60"/);
  assert.match(mark, /<rect x="1" y="1" width="58" height="58"/);
});
