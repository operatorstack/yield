package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

func renderBootstrapSkill(language string, profile bootstrapProfile) (map[string]string, string, error) {
	version := profile.YieldVersion
	agents := strings.Join(profile.Agents, ",")
	launcher := map[string]string{
		"typescript": "npm --prefix 'skills/yield-workflow-builder' exec -- yskill",
		"python":     fmt.Sprintf("uvx --from 'yieldskill==%s' yskill", shellQuoteValue(version)),
		"go":         ".yield/bin/yskill",
		"rust":       ".yield/bin/yskill",
	}[language]
	config, err := json.MarshalIndent(map[string]any{
		"version": 1, "yield_version": version, "language": language,
		"launcher": launcher, "agents": profile.Agents,
	}, "", "  ")
	if err != nil {
		return nil, "", err
	}
	files := map[string]string{
		"SKILL.md": fmt.Sprintf(`---
name: yield-workflow-builder
description: Learn Yield, create or convert skill workflows, check and repair them, upgrade dependencies, and register coding-agent adapters.
---

<!-- generated-by: yskill-bootstrap; version: %s -->

Run from the repository root:

    %s run 'skills/yield-workflow-builder'

Follow each returned operation exactly. Answer each operation directly:

    %s respond <run-id> --value <answer> --skill 'skills/yield-workflow-builder'

Use --result-json for structured agent work. When an operation asks you to
write or repair files, edit the repository before returning the JSON result.
Do not skip an operation or invent its response.

The helper teaches the relevant Yield primitive and previews exact files and
commands before any mutation. A declined plan ends without applying it. Learn
and check modes do not write files; check runs static doctor without fixtures.
The helper cannot upgrade itself during an active run. Exit and run
yskill helper install to refresh it.

For convert mode, the builder first applies a semantic-disposition projection:

    C = clauses(S)
    Pi(S) = {(c, d, T, r) | c in C}

Each source clause has exactly one disposition: control, guidance, both, or
excluded. Control must reach code. Guidance must remain model-facing in the
canonical SKILL.md or a relevant agent_task. Both must reach both places.
Excluded clauses have no destination and need a reason. Every destination must
remain reachable by the coding agent. The projection stays in the Yield run
log. It is not a generated destination file.
`, version, launcher, launcher),
		"builder.json":            string(config) + "\n",
		"fixtures/responses.json": bootstrapFixtureResponses,
		"fixtures/test.json":      bootstrapFixtureConfig,
	}
	dependency := ""
	switch language {
	case "typescript":
		files["main.ts"] = bootstrapTypeScript
		files["package.json"] = fmt.Sprintf("{\n  \"private\": true,\n  \"type\": \"module\",\n  \"dependencies\": { \"@operatorstack/yield\": \"%s\" }\n}\n", version)
		files["skill.json"] = fmt.Sprintf("{\"version\":1,\"yield_version\":%q,\"language\":\"typescript\",\"run\":[\"node\",\"main.ts\"]}\n", version)
		dependency = "npm install --ignore-scripts --no-audit --no-fund (inside skills/yield-workflow-builder)"
	case "python":
		files["main.py"] = bootstrapPython
		files["requirements.txt"] = fmt.Sprintf("yieldskill==%s\n", version)
		files["skill.json"] = fmt.Sprintf("{\"version\":1,\"yield_version\":%q,\"language\":\"python\",\"run\":[\"python\",\"main.py\"]}\n", version)
	case "go":
		files["main.go"] = bootstrapGo
		files["go.mod"] = fmt.Sprintf("module yield-workflow-builder\n\ngo 1.26.5\n\nrequire github.com/operatorstack/yield v%s\n", version)
		files["skill.json"] = fmt.Sprintf("{\"version\":1,\"yield_version\":%q,\"language\":\"go\",\"run\":[\"go\",\"run\",\"-mod=readonly\",\".\"]}\n", version)
		dependency = "go mod tidy (inside skills/yield-workflow-builder)"
	case "rust":
		files[".gitignore"] = rustSkillGitignore
		files["src/main.rs"] = bootstrapRust
		files["Cargo.toml"] = fmt.Sprintf("[package]\nname = \"yield-workflow-builder\"\nversion = \"0.1.0\"\nedition = \"2021\"\n\n[dependencies]\nyieldskill = { version = \"=%s\" }\nserde_json = \"1\"\n", version)
		files["skill.json"] = fmt.Sprintf("{\"version\":1,\"yield_version\":%q,\"language\":\"rust\",\"run\":[\"cargo\",\"run\",\"--quiet\"]}\n", version)
	default:
		return nil, "", fmt.Errorf("unsupported language %q", language)
	}
	for path, content := range files {
		files[path] = strings.ReplaceAll(content, "{{AGENTS}}", agents)
	}
	return files, dependency, nil
}

const bootstrapFixtureResponses = `{
  "select-mode": {"value": "create"},
  "collect-specification": {
    "name": "yield-workflow-builder-fixture",
    "description": "Create a harmless fixture workflow.",
    "language": "go",
    "destination": "skills/yield-workflow-builder-fixture",
    "source_path": "",
    "target_path": "",
    "requested_version": ""
  },
  "extract-flow": {
    "summary": "Ask for confirmation and complete.",
    "steps": [{"id":"confirm","kind":"ask_user","description":"Ask for confirmation."}]
  },
  "teach-and-plan": {
    "summary": "Create one harmless fixture workflow.",
    "primitives": ["AgentTask for bounded judgment; Require for verified completion."],
    "files": ["skills/yield-workflow-builder-fixture/SKILL.md","skills/yield-workflow-builder-fixture/main.go"],
    "commands": ["yskill doctor skills/yield-workflow-builder-fixture --test"]
  },
  "approve-change": {"value": "apply"},
  "write-workflow": {"files":["skills/yield-workflow-builder-fixture/SKILL.md","skills/yield-workflow-builder-fixture/main.go"]}
}
`

const bootstrapFixtureConfig = `{
  "version": 1,
  "setup": [],
  "after_response": {},
  "teardown": []
}
`

const bootstrapTypeScript = `import { existsSync, readFileSync, realpathSync } from "node:fs";
import { dirname, isAbsolute, normalize, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";
import { defineSkill } from "@operatorstack/yield";

type Mode = "learn"|"create"|"convert"|"check"|"repair"|"upgrade"|"register";
type Specification = { name:string; description:string; language:"typescript"|"python"|"go"|"rust"; destination:string; source_path:string; target_path:string; requested_version:string };
type Files = { files:string[] };
type Projection = { ready:boolean; unresolved:string[]; clauses:unknown[] };
type Plan = { summary:string; primitives:string[]; files:string[]; commands:string[] };
const helperPath = "skills/yield-workflow-builder";
const specSchema = {type:"object",required:["name","description","language","destination","source_path","target_path","requested_version"],additionalProperties:false,properties:{name:{type:"string"},description:{type:"string"},language:{enum:["typescript","python","go","rust"]},destination:{type:"string"},source_path:{type:"string"},target_path:{type:"string"},requested_version:{type:"string"}}};
const projectionSchema = {type:"object",required:["clauses","ready","unresolved"],additionalProperties:false,properties:{clauses:{type:"array",minItems:1,items:{type:"object",required:["source_clause","disposition","destinations","reason"],additionalProperties:false,properties:{source_clause:{type:"string",minLength:1},disposition:{enum:["control","guidance","both","excluded"]},destinations:{type:"array",items:{type:"object",required:["kind","target"],additionalProperties:false,properties:{kind:{enum:["code","skill","agent_task"]},target:{type:"string",minLength:1}}}},reason:{type:"string"}}}},ready:{type:"boolean"},unresolved:{type:"array",items:{type:"string",minLength:1}}}};
const flowSchema = {type:"object",required:["summary","steps"],additionalProperties:false,properties:{summary:{type:"string",minLength:1},steps:{type:"array",minItems:1,items:{type:"object",required:["id","kind","description"],additionalProperties:false,properties:{id:{type:"string",minLength:1},kind:{enum:["ask_user","agent_task","run_command","branch","require"]},description:{type:"string",minLength:1}}}}}};
const filesSchema = {type:"object",required:["files"],additionalProperties:false,properties:{files:{type:"array",minItems:1,items:{type:"string",minLength:1}}}};
const planSchema = {type:"object",required:["summary","primitives","files","commands"],additionalProperties:false,properties:{summary:{type:"string",minLength:1},primitives:{type:"array",minItems:1,items:{type:"string",minLength:1}},files:{type:"array",minItems:1,items:{type:"string",minLength:1}},commands:{type:"array",minItems:1,items:{type:"string",minLength:1}}}};
const guideSchema = {type:"object",required:["summary","primitives","manual_steps","docs"],additionalProperties:false,properties:{summary:{type:"string",minLength:1},primitives:{type:"array",minItems:1,items:{type:"string",minLength:1}},manual_steps:{type:"array",minItems:1,items:{type:"string",minLength:1}},docs:{type:"array",minItems:1,items:{type:"string",minLength:1}}}};
const config = JSON.parse(readFileSync(new URL("./builder.json", import.meta.url), "utf8")) as {launcher:string;agents:string[];yield_version:string};
const root = realpathSync(fileURLToPath(new URL("../../", import.meta.url)));
const quote = (v:string) => "'" + v.replaceAll("'", "'\\''") + "'";
const inside = (p:string) => p === root || p.startsWith(root+sep);
const safe = (p:string, destination=false) => !!p && !isAbsolute(p) && !normalize(p).split(sep).includes("..") && normalize(p).startsWith("skills"+sep) && inside(realpathSync(destination ? dirname(resolve(root,p)) : resolve(root,p)));
const filesStayInside = (files:string[], target:string) => files.every((file) => normalize(file) === normalize(target) || normalize(file).startsWith(normalize(target)+sep));

defineSkill((ctx) => {
  const mode = ctx.askUser("select-mode", "What do you want to do with Yield?", [
    {value:"learn",label:"Learn"},{value:"create",label:"Create"},{value:"convert",label:"Convert"},{value:"check",label:"Check"},
    {value:"repair",label:"Repair"},{value:"upgrade",label:"Upgrade"},{value:"register",label:"Register"},
  ]) as Mode;
  if (mode === "learn") {
    const guide = ctx.agentTask("teach-yield", "Teach the smallest relevant Yield concept for the user's request. Explain what remains in SKILL.md, what moves into code, fixtures, doctor, and registration. Give manual steps before mentioning this helper. Do not edit files or run commands.", {mode}, guideSchema);
    return {mode,changed:false,guide};
  }
  const spec = ctx.agentTask<Specification>("collect-specification", "Use the user's current request. For create or convert, return a safe kebab-case name, description, target language, new destination under skills/, and source_path for convert. For check, repair, upgrade, or register, return target_path for one existing workflow under skills/. Set requested_version only when the user explicitly requests one. Use empty strings for fields that do not apply. Do not write files.", {mode}, specSchema);
  const creates = mode === "create" || mode === "convert";
  const target = creates ? spec.destination : spec.target_path;
  if (creates) {
    if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(spec.name) || !spec.description || !safe(target,true) || target === helperPath) ctx.blocked("the destination must be a new named path under skills/");
    const available = ctx.runCommand("check-destination", "cd ../.. && test ! -e "+quote(target), 30);
    if (available.exit_code !== 0) ctx.blocked("the destination must be a new path under skills/");
  } else if (!safe(target)) {
    ctx.blocked("the target workflow must be an existing path under skills/");
  }
  if (mode === "upgrade" && target === helperPath) ctx.blocked("the helper cannot upgrade itself during an active run; exit and run yskill helper install");
  if (mode === "upgrade" && spec.requested_version && spec.requested_version !== config.yield_version) ctx.blocked("unsupported Yield version change; exit and run yskill helper install for the requested version");
  const fixture = target === "skills/yield-workflow-builder-fixture";
  if (mode === "check") {
    const checked = ctx.runCommand("check-workflow", fixture?"printf fixture-check-ok":"cd ../.. && "+config.launcher+" doctor "+quote(target)+" --root .", 120);
    return {mode,target,changed:false,healthy:checked.exit_code===0,stdout:checked.stdout,stderr:checked.stderr};
  }
  let source = "";
  let projection: Projection|null = null;
  if (mode === "convert") {
    if (!safe(spec.source_path)) ctx.blocked("the source skill must be inside the repository");
    const sourceFile = resolve(root,spec.source_path,"SKILL.md");
    if (!existsSync(sourceFile)) ctx.blocked("the source SKILL.md does not exist");
    source = readFileSync(sourceFile,"utf8");
    projection = ctx.agentTask<Projection>("project-semantics", "Map every source clause exactly once using Pi(S)={(c,d,T,r)|c in clauses(S)}. YAML frontmatter is metadata, not a clause. Each top-level bullet is exactly one clause, including a compound sentence. For prose without bullets, treat each paragraph as one clause. Use disposition control, guidance, both, or excluded. Control needs a reachable code destination. Guidance needs a reachable skill or agent_task destination. Both needs both kinds. Excluded needs no destination and a non-empty reason. Name concrete destinations that the writer can create. Use the target language's canonical entrypoint for code: main.ts, main.py, main.go, or src/main.rs. Report uncertainty in unresolved and set ready false. Do not write files.", {source,spec}, projectionSchema);
    if (!projection.ready || projection.unresolved.length > 0) ctx.blocked("the semantic projection has unresolved source clauses");
  }
  let flow:unknown = null;
  let inspection:unknown = null;
  if (creates) flow = ctx.agentTask("extract-flow", "Extract or design the minimal workflow. Keep model judgment in agent_task. Put order, branches, commands, approvals, evidence, and finish rules in code.", {mode,spec,source,projection}, flowSchema);
  else inspection = ctx.runCommand("inspect-workflow", fixture?"printf fixture-inspect-ok":"cd ../.. && "+config.launcher+" doctor "+quote(target)+" --root .", 120);
  const plan = ctx.agentTask<Plan>("teach-and-plan", "Teach the relevant Yield primitives, then return the exact files and commands proposed for this operation. For register, explain generated adapters. For repair, use the doctor evidence. For upgrade, target the installed Yield version and include dependency and lockfile changes. Do not edit files or run commands.", {mode,spec,target,flow,inspection,yield_version:config.yield_version}, planSchema);
  const approval = ctx.askUser("approve-change", plan.summary+" Apply this plan?", [{value:"apply",label:"Apply"},{value:"stop",label:"Stop"}]);
  if (approval !== "apply") ctx.refused("the developer declined the proposed Yield changes");
  const agentFlag = config.agents.join(",");
  const verify = fixture ? "printf fixture-ok" : "cd ../.. && "+config.launcher+" doctor "+quote(target)+" --root . --test";
  if (mode === "register") {
    const checked = ctx.runCommand("verify-before-register",verify,600);
    ctx.require(checked.exit_code===0,"the workflow passes its fixture run",{exit_code:checked.exit_code});
    const registered = ctx.runCommand("register-workflow",fixture?"printf fixture-register-ok":"cd ../.. && "+config.launcher+" register "+quote(target)+" --root . --agent "+quote(agentFlag),120);
    ctx.require(registered.exit_code===0,"the workflow is registered",{exit_code:registered.exit_code});
    const adapters = ctx.runCommand("verify-adapters",fixture?"printf fixture-adapters-ok":"cd ../.. && "+config.launcher+" doctor "+quote(target)+" --root . --agent "+quote(agentFlag)+" --test",600);
    ctx.require(adapters.exit_code===0,"the generated adapters pass verification",{exit_code:adapters.exit_code});
    return {mode,target,changed:true,verified:true};
  }
  let written = ctx.agentTask<Files>(creates?"write-workflow":mode+"-workflow", creates?"Create the complete Yield workflow at the destination. Follow every semantic disposition. Write the program, SKILL.md, exact-version dependencies, skill.json, and self-contained fixtures. Keep useful model-facing guidance in SKILL.md; remove only duplicated sequencing. Do not edit outside the destination.":"Apply only the approved operation to the target workflow. For upgrade, update its manifest and language dependency to the supplied exact Yield version. Do not edit outside the target. Return every changed file.", {mode,spec,target,source,projection,flow,plan,yield_version:config.yield_version}, filesSchema);
  if (!filesStayInside(written.files,target)) ctx.blocked("the reported changes escape the approved workflow directory");
  let checked = ctx.runCommand("verify-workflow",verify,600);
  for (let attempt=1; checked.exit_code!==0 && attempt<=2; attempt++) {
    written = ctx.agentTask<Files>("repair-workflow-"+attempt,"Verification failed. Repair only the approved workflow directory and return every changed file.",{mode,target,stdout:checked.stdout,stderr:checked.stderr},filesSchema);
    if (!filesStayInside(written.files,target)) ctx.blocked("the reported repair escapes the approved workflow directory");
    checked = ctx.runCommand("verify-workflow-retry-"+attempt,verify,600);
  }
  if (checked.exit_code!==0) ctx.blocked("the workflow still fails after two repair attempts");
  ctx.require(checked.exit_code===0,"the workflow passes its fixture run",{exit_code:checked.exit_code});
  const registered = ctx.runCommand("register-workflow",fixture?"printf fixture-register-ok":"cd ../.. && "+config.launcher+" register "+quote(target)+" --root . --agent "+quote(agentFlag),120);
  ctx.require(registered.exit_code===0,"the workflow is registered",{exit_code:registered.exit_code});
  const adapters = ctx.runCommand("verify-adapters",fixture?"printf fixture-adapters-ok":"cd ../.. && "+config.launcher+" doctor "+quote(target)+" --root . --agent "+quote(agentFlag)+" --test",600);
  ctx.require(adapters.exit_code===0,"the generated adapters pass verification",{exit_code:adapters.exit_code});
  return {mode,target,language:spec.language,files:written.files,changed:true,verified:true};
});
`

const bootstrapPython = `from pathlib import Path
import json
from yieldskill import define_skill

ROOT = Path(__file__).resolve().parents[2].resolve()
CONFIG = json.loads((Path(__file__).parent / "builder.json").read_text())
HELPER_PATH = "skills/yield-workflow-builder"
SPEC_SCHEMA = {"type":"object","required":["name","description","language","destination","source_path","target_path","requested_version"],"additionalProperties":False,"properties":{"name":{"type":"string"},"description":{"type":"string"},"language":{"enum":["typescript","python","go","rust"]},"destination":{"type":"string"},"source_path":{"type":"string"},"target_path":{"type":"string"},"requested_version":{"type":"string"}}}
PROJECTION_SCHEMA = {"type":"object","required":["clauses","ready","unresolved"],"additionalProperties":False,"properties":{"clauses":{"type":"array","minItems":1,"items":{"type":"object","required":["source_clause","disposition","destinations","reason"],"additionalProperties":False,"properties":{"source_clause":{"type":"string","minLength":1},"disposition":{"enum":["control","guidance","both","excluded"]},"destinations":{"type":"array","items":{"type":"object","required":["kind","target"],"additionalProperties":False,"properties":{"kind":{"enum":["code","skill","agent_task"]},"target":{"type":"string","minLength":1}}}},"reason":{"type":"string"}}}},"ready":{"type":"boolean"},"unresolved":{"type":"array","items":{"type":"string","minLength":1}}}}
FLOW_SCHEMA = {"type":"object","required":["summary","steps"],"additionalProperties":False,"properties":{"summary":{"type":"string","minLength":1},"steps":{"type":"array","minItems":1,"items":{"type":"object","required":["id","kind","description"],"additionalProperties":False,"properties":{"id":{"type":"string","minLength":1},"kind":{"enum":["ask_user","agent_task","run_command","branch","require"]},"description":{"type":"string","minLength":1}}}}}}
FILES_SCHEMA = {"type":"object","required":["files"],"additionalProperties":False,"properties":{"files":{"type":"array","minItems":1,"items":{"type":"string","minLength":1}}}}
PLAN_SCHEMA = {"type":"object","required":["summary","primitives","files","commands"],"additionalProperties":False,"properties":{"summary":{"type":"string","minLength":1},"primitives":{"type":"array","minItems":1,"items":{"type":"string","minLength":1}},"files":{"type":"array","minItems":1,"items":{"type":"string","minLength":1}},"commands":{"type":"array","minItems":1,"items":{"type":"string","minLength":1}}}}
GUIDE_SCHEMA = {"type":"object","required":["summary","primitives","manual_steps","docs"],"additionalProperties":False,"properties":{"summary":{"type":"string","minLength":1},"primitives":{"type":"array","minItems":1,"items":{"type":"string","minLength":1}},"manual_steps":{"type":"array","minItems":1,"items":{"type":"string","minLength":1}},"docs":{"type":"array","minItems":1,"items":{"type":"string","minLength":1}}}}

def safe(path, destination=False):
    candidate = Path(path)
    if not path or candidate.is_absolute() or ".." in candidate.parts or len(candidate.parts) <= 1 or candidate.parts[0] != "skills": return False
    resolved = ((ROOT / candidate).parent if destination else (ROOT / candidate)).resolve()
    return resolved == ROOT or ROOT in resolved.parents

def quote(value): return "'" + value.replace("'", "'\\''") + "'"
def files_stay_inside(files, target):
    base = Path(target)
    return all(Path(file) == base or base in Path(file).parents for file in files)
def receipt(result): return {"exit_code":result.exit_code,"stdout":result.stdout,"stderr":result.stderr}

def program(ctx):
    options = [{"value":value,"label":value.title()} for value in ["learn","create","convert","check","repair","upgrade","register"]]
    mode = ctx.ask_user("select-mode", "What do you want to do with Yield?", options=options)
    if mode == "learn":
        guide = ctx.agent_task("teach-yield", "Teach the smallest relevant Yield concept for the user's request. Explain what remains in SKILL.md, what moves into code, fixtures, doctor, and registration. Give manual steps before mentioning this helper. Do not edit files or run commands.", context={"mode":mode}, schema=GUIDE_SCHEMA)
        return {"mode":mode,"changed":False,"guide":guide}
    spec = ctx.agent_task("collect-specification", "Use the user's current request. For create or convert, return a safe kebab-case name, description, target language, new destination under skills/, and source_path for convert. For check, repair, upgrade, or register, return target_path for one existing workflow under skills/. Set requested_version only when the user explicitly requests one. Use empty strings for fields that do not apply. Do not write files.", context={"mode":mode}, schema=SPEC_SCHEMA)
    creates = mode in ["create","convert"]
    target = spec["destination"] if creates else spec["target_path"]
    if creates:
        valid_name = spec["name"] and all(part.isalnum() and part.lower() == part for part in spec["name"].split("-"))
        if not valid_name or not spec["description"] or not safe(target, True) or target == HELPER_PATH: ctx.blocked("the destination must be a new named path under skills/")
        available = ctx.run_command("check-destination", "cd ../.. && test ! -e " + quote(target), timeout_seconds=30)
        if available.exit_code != 0: ctx.blocked("the destination must be a new path under skills/")
    elif not safe(target): ctx.blocked("the target workflow must be an existing path under skills/")
    if mode == "upgrade" and target == HELPER_PATH: ctx.blocked("the helper cannot upgrade itself during an active run; exit and run yskill helper install")
    if mode == "upgrade" and spec["requested_version"] and spec["requested_version"] != CONFIG["yield_version"]: ctx.blocked("unsupported Yield version change; exit and run yskill helper install for the requested version")
    fixture = target == "skills/yield-workflow-builder-fixture"
    if mode == "check":
        checked = ctx.run_command("check-workflow", "printf fixture-check-ok" if fixture else "cd ../.. && " + CONFIG["launcher"] + " doctor " + quote(target) + " --root .", timeout_seconds=120)
        return {"mode":mode,"target":target,"changed":False,"healthy":checked.exit_code == 0,"stdout":checked.stdout,"stderr":checked.stderr}
    source = ""
    projection = None
    if mode == "convert":
        if not safe(spec["source_path"]): ctx.blocked("the source skill must be inside the repository")
        source_file = ROOT / spec["source_path"] / "SKILL.md"
        if not source_file.is_file(): ctx.blocked("the source SKILL.md does not exist")
        source = source_file.read_text()
        projection = ctx.agent_task("project-semantics", "Map every source clause exactly once using Pi(S)={(c,d,T,r)|c in clauses(S)}. YAML frontmatter is metadata, not a clause. Each top-level bullet is exactly one clause, including a compound sentence. For prose without bullets, treat each paragraph as one clause. Use disposition control, guidance, both, or excluded. Control needs a reachable code destination. Guidance needs a reachable skill or agent_task destination. Both needs both kinds. Excluded needs no destination and a non-empty reason. Name concrete destinations that the writer can create. Use the target language's canonical entrypoint for code: main.ts, main.py, main.go, or src/main.rs. Report uncertainty in unresolved and set ready false. Do not write files.", context={"source":source,"spec":spec}, schema=PROJECTION_SCHEMA)
        if not projection["ready"] or projection["unresolved"]: ctx.blocked("the semantic projection has unresolved source clauses")
    flow = ctx.agent_task("extract-flow", "Extract or design the minimal workflow. Keep model judgment in agent_task. Put order, branches, commands, approvals, evidence, and finish rules in code.", context={"mode":mode,"spec":spec,"source":source,"projection":projection}, schema=FLOW_SCHEMA) if creates else None
    inspection_result = None if creates else ctx.run_command("inspect-workflow", "printf fixture-inspect-ok" if fixture else "cd ../.. && " + CONFIG["launcher"] + " doctor " + quote(target) + " --root .", timeout_seconds=120)
    inspection = None if inspection_result is None else receipt(inspection_result)
    plan = ctx.agent_task("teach-and-plan", "Teach the relevant Yield primitives, then return the exact files and commands proposed. For register, explain adapters. For repair, use doctor evidence. For upgrade, target the installed Yield version and include dependency and lockfile changes. Do not edit files or run commands.", context={"mode":mode,"spec":spec,"target":target,"flow":flow,"inspection":inspection,"yield_version":CONFIG["yield_version"]}, schema=PLAN_SCHEMA)
    approval = ctx.ask_user("approve-change", plan["summary"] + " Apply this plan?", options=[{"value":"apply","label":"Apply"},{"value":"stop","label":"Stop"}])
    if approval != "apply": ctx.refused("the developer declined the proposed Yield changes")
    agents = ",".join(CONFIG["agents"])
    verify = "printf fixture-ok" if fixture else "cd ../.. && " + CONFIG["launcher"] + " doctor " + quote(target) + " --root . --test"
    register = "printf fixture-register-ok" if fixture else "cd ../.. && " + CONFIG["launcher"] + " register " + quote(target) + " --root . --agent " + quote(agents)
    verify_adapters = "printf fixture-adapters-ok" if fixture else "cd ../.. && " + CONFIG["launcher"] + " doctor " + quote(target) + " --root . --agent " + quote(agents) + " --test"
    if mode == "register":
        checked = ctx.run_command("verify-before-register", verify, timeout_seconds=600); ctx.require(checked.exit_code == 0, "the workflow passes its fixture run", {"exit_code":checked.exit_code})
        registered = ctx.run_command("register-workflow", register, timeout_seconds=120); ctx.require(registered.exit_code == 0, "the workflow is registered", {"exit_code":registered.exit_code})
        adapters = ctx.run_command("verify-adapters", verify_adapters, timeout_seconds=600); ctx.require(adapters.exit_code == 0, "the generated adapters pass verification", {"exit_code":adapters.exit_code})
        return {"mode":mode,"target":target,"changed":True,"verified":True}
    prompt = "Create the complete Yield workflow at the destination. Follow every semantic disposition. Write the program, SKILL.md, exact-version dependencies, skill.json, and self-contained fixtures. Keep useful model-facing guidance in SKILL.md; remove only duplicated sequencing. Do not edit outside the destination." if creates else "Apply only the approved operation to the target. For upgrade, update its manifest and language dependency to the supplied exact Yield version. Do not edit outside the target. Return every changed file."
    written = ctx.agent_task("write-workflow" if creates else mode + "-workflow", prompt, context={"mode":mode,"spec":spec,"target":target,"source":source,"projection":projection,"flow":flow,"plan":plan,"yield_version":CONFIG["yield_version"]}, schema=FILES_SCHEMA)
    if not files_stay_inside(written["files"], target): ctx.blocked("the reported changes escape the approved workflow directory")
    checked = ctx.run_command("verify-workflow", verify, timeout_seconds=600)
    for attempt in range(1, 3):
        if checked.exit_code == 0: break
        written = ctx.agent_task(f"repair-workflow-{attempt}", "Verification failed. Repair only the approved workflow directory and return every changed file.", context={"mode":mode,"target":target,"stdout":checked.stdout,"stderr":checked.stderr}, schema=FILES_SCHEMA)
        if not files_stay_inside(written["files"], target): ctx.blocked("the reported repair escapes the approved workflow directory")
        checked = ctx.run_command(f"verify-workflow-retry-{attempt}", verify, timeout_seconds=600)
    if checked.exit_code != 0: ctx.blocked("the workflow still fails after two repair attempts")
    ctx.require(True, "the workflow passes its fixture run", {"exit_code":checked.exit_code})
    registered = ctx.run_command("register-workflow", register, timeout_seconds=120); ctx.require(registered.exit_code == 0, "the workflow is registered", {"exit_code":registered.exit_code})
    adapters = ctx.run_command("verify-adapters", verify_adapters, timeout_seconds=600); ctx.require(adapters.exit_code == 0, "the generated adapters pass verification", {"exit_code":adapters.exit_code})
    return {"mode":mode,"target":target,"language":spec["language"],"files":written["files"],"changed":True,"verified":True}

define_skill(program)
`

const bootstrapGo = `package main

import (
  "encoding/json"
  "fmt"
  "os"
  "path/filepath"
  "regexp"
  "strings"
  "github.com/operatorstack/yield/sdk/yield"
)
type specification struct { Name string ` + "`json:\"name\"`" + `; Description string ` + "`json:\"description\"`" + `; Language string ` + "`json:\"language\"`" + `; Destination string ` + "`json:\"destination\"`" + `; SourcePath string ` + "`json:\"source_path\"`" + `; TargetPath string ` + "`json:\"target_path\"`" + `; RequestedVersion string ` + "`json:\"requested_version\"`" + ` }
type fileResult struct { Files []string ` + "`json:\"files\"`" + ` }
type projectionResult struct { Ready bool ` + "`json:\"ready\"`" + `; Unresolved []string ` + "`json:\"unresolved\"`" + ` }
type planResult struct { Summary string ` + "`json:\"summary\"`" + ` }
type config struct { Launcher string ` + "`json:\"launcher\"`" + `; Agents []string ` + "`json:\"agents\"`" + `; YieldVersion string ` + "`json:\"yield_version\"`" + ` }
const helperPath = "skills/yield-workflow-builder"
const specSchema = ` + "`" + `{"type":"object","required":["name","description","language","destination","source_path","target_path","requested_version"],"additionalProperties":false,"properties":{"name":{"type":"string"},"description":{"type":"string"},"language":{"enum":["typescript","python","go","rust"]},"destination":{"type":"string"},"source_path":{"type":"string"},"target_path":{"type":"string"},"requested_version":{"type":"string"}}}` + "`" + `
const projectionSchema = ` + "`" + `{"type":"object","required":["clauses","ready","unresolved"],"additionalProperties":false,"properties":{"clauses":{"type":"array","minItems":1,"items":{"type":"object","required":["source_clause","disposition","destinations","reason"],"additionalProperties":false,"properties":{"source_clause":{"type":"string","minLength":1},"disposition":{"enum":["control","guidance","both","excluded"]},"destinations":{"type":"array","items":{"type":"object","required":["kind","target"],"additionalProperties":false,"properties":{"kind":{"enum":["code","skill","agent_task"]},"target":{"type":"string","minLength":1}}}},"reason":{"type":"string"}}}},"ready":{"type":"boolean"},"unresolved":{"type":"array","items":{"type":"string","minLength":1}}}}` + "`" + `
const flowSchema = ` + "`" + `{"type":"object","required":["summary","steps"],"additionalProperties":false,"properties":{"summary":{"type":"string","minLength":1},"steps":{"type":"array","minItems":1,"items":{"type":"object","required":["id","kind","description"],"additionalProperties":false,"properties":{"id":{"type":"string","minLength":1},"kind":{"enum":["ask_user","agent_task","run_command","branch","require"]},"description":{"type":"string","minLength":1}}}}}}` + "`" + `
const filesSchema = ` + "`" + `{"type":"object","required":["files"],"additionalProperties":false,"properties":{"files":{"type":"array","minItems":1,"items":{"type":"string","minLength":1}}}}` + "`" + `
const planSchema = ` + "`" + `{"type":"object","required":["summary","primitives","files","commands"],"additionalProperties":false,"properties":{"summary":{"type":"string","minLength":1},"primitives":{"type":"array","minItems":1,"items":{"type":"string","minLength":1}},"files":{"type":"array","minItems":1,"items":{"type":"string","minLength":1}},"commands":{"type":"array","minItems":1,"items":{"type":"string","minLength":1}}}}` + "`" + `
const guideSchema = ` + "`" + `{"type":"object","required":["summary","primitives","manual_steps","docs"],"additionalProperties":false,"properties":{"summary":{"type":"string","minLength":1},"primitives":{"type":"array","minItems":1,"items":{"type":"string","minLength":1}},"manual_steps":{"type":"array","minItems":1,"items":{"type":"string","minLength":1}},"docs":{"type":"array","minItems":1,"items":{"type":"string","minLength":1}}}}` + "`" + `
func quote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
func safe(root,value string,destination bool) bool { if value==""||filepath.IsAbs(value) { return false }; clean:=filepath.Clean(value); parts:=strings.Split(filepath.ToSlash(clean), "/"); if len(parts)<=1||parts[0]!="skills"||strings.Contains(filepath.ToSlash(clean), "../"){return false};probe:=filepath.Join(root,clean);if destination{probe=filepath.Dir(probe)};canonical,err:=filepath.EvalSymlinks(probe);return err==nil&&(canonical==root||strings.HasPrefix(canonical+string(filepath.Separator),root+string(filepath.Separator))) }
func filesStayInside(files []string,target string) bool { base:=filepath.Clean(target);for _,file:=range files{clean:=filepath.Clean(file);if clean!=base&&!strings.HasPrefix(clean,base+string(filepath.Separator)){return false}};return true }
func main() { yield.Main(func(ctx *yield.Context) (yield.Outcome,error) {
  rawConfig,err:=os.ReadFile("builder.json"); if err!=nil{return yield.Outcome{},err}; var cfg config; if err=json.Unmarshal(rawConfig,&cfg);err!=nil{return yield.Outcome{},err}
  mode:=ctx.AskUser("select-mode","What do you want to do with Yield?",yield.Option{Value:"learn",Label:"Learn"},yield.Option{Value:"create",Label:"Create"},yield.Option{Value:"convert",Label:"Convert"},yield.Option{Value:"check",Label:"Check"},yield.Option{Value:"repair",Label:"Repair"},yield.Option{Value:"upgrade",Label:"Upgrade"},yield.Option{Value:"register",Label:"Register"})
  if mode=="learn" { guide:=ctx.AgentTask("teach-yield","Teach the smallest relevant Yield concept for the user's request. Explain what remains in SKILL.md, what moves into code, fixtures, doctor, and registration. Give manual steps before mentioning this helper. Do not edit files or run commands.",map[string]any{"mode":mode},json.RawMessage(guideSchema));return ctx.Complete(map[string]any{"mode":mode,"changed":false,"guide":json.RawMessage(guide)}) }
  raw:=ctx.AgentTask("collect-specification","Use the user's current request. For create or convert, return a safe kebab-case name, description, target language, new destination under skills/, and source_path for convert. For check, repair, upgrade, or register, return target_path for one existing workflow under skills/. Set requested_version only when the user explicitly requests one. Use empty strings for fields that do not apply. Do not write files.",map[string]any{"mode":mode},json.RawMessage(specSchema)); var spec specification; if err=json.Unmarshal(raw,&spec);err!=nil{return yield.Outcome{},err}
  root,err:=filepath.Abs(filepath.Join("..","..")); if err!=nil{return yield.Outcome{},err};root,err=filepath.EvalSymlinks(root);if err!=nil{return yield.Outcome{},err}
  creates:=mode=="create"||mode=="convert";target:=spec.TargetPath;if creates{target=spec.Destination;if !regexp.MustCompile(` + "`" + `^[a-z0-9]+(?:-[a-z0-9]+)*$` + "`" + `).MatchString(spec.Name)||spec.Description==""||!safe(root,target,true)||target==helperPath{return yield.Outcome{},ctx.Blocked("the destination must be a new named path under skills/")};available:=ctx.RunCommand("check-destination","cd ../.. && test ! -e "+quote(target),30);if available.ExitCode!=0{return yield.Outcome{},ctx.Blocked("the destination must be a new path under skills/")}}else if !safe(root,target,false){return yield.Outcome{},ctx.Blocked("the target workflow must be an existing path under skills/")}
  if mode=="upgrade"&&target==helperPath{return yield.Outcome{},ctx.Blocked("the helper cannot upgrade itself during an active run; exit and run yskill helper install")}
  if mode=="upgrade"&&spec.RequestedVersion!=""&&spec.RequestedVersion!=cfg.YieldVersion{return yield.Outcome{},ctx.Blocked("unsupported Yield version change; exit and run yskill helper install for the requested version")};fixture:=target=="skills/yield-workflow-builder-fixture"
  if mode=="check" { command:="cd ../.. && "+cfg.Launcher+" doctor "+quote(target)+" --root .";if fixture{command="printf fixture-check-ok"};checked:=ctx.RunCommand("check-workflow",command,120);return ctx.Complete(map[string]any{"mode":mode,"target":target,"changed":false,"healthy":checked.ExitCode==0,"stdout":checked.Stdout,"stderr":checked.Stderr}) }
  source:="";var projection json.RawMessage;if mode=="convert"{if !safe(root,spec.SourcePath,false){return yield.Outcome{},ctx.Blocked("the source skill must be inside the repository")};b,readErr:=os.ReadFile(filepath.Join(root,spec.SourcePath,"SKILL.md"));if readErr!=nil{return yield.Outcome{},ctx.Blocked("the source SKILL.md does not exist")};source=string(b);projection=ctx.AgentTask("project-semantics","Map every source clause exactly once using Pi(S)={(c,d,T,r)|c in clauses(S)}. YAML frontmatter is metadata, not a clause. Each top-level bullet is exactly one clause, including a compound sentence. For prose without bullets, treat each paragraph as one clause. Use disposition control, guidance, both, or excluded. Control needs a reachable code destination. Guidance needs a reachable skill or agent_task destination. Both needs both kinds. Excluded needs no destination and a non-empty reason. Name concrete destinations that the writer can create. Use the target language's canonical entrypoint for code: main.ts, main.py, main.go, or src/main.rs. Report uncertainty in unresolved and set ready false. Do not write files.",map[string]any{"source":source,"spec":spec},json.RawMessage(projectionSchema));var projected projectionResult;if err=json.Unmarshal(projection,&projected);err!=nil{return yield.Outcome{},err};if !projected.Ready||len(projected.Unresolved)>0{return yield.Outcome{},ctx.Blocked("the semantic projection has unresolved source clauses")}}
  var flow json.RawMessage;var inspection any;if creates{flow=ctx.AgentTask("extract-flow","Extract or design the minimal workflow. Keep model judgment in agent_task. Put order, branches, commands, approvals, evidence, and finish rules in code.",map[string]any{"mode":mode,"spec":spec,"source":source,"projection":projection},json.RawMessage(flowSchema))}else{command:="cd ../.. && "+cfg.Launcher+" doctor "+quote(target)+" --root .";if fixture{command="printf fixture-inspect-ok"};observed:=ctx.RunCommand("inspect-workflow",command,120);inspection=map[string]any{"exit_code":observed.ExitCode,"stdout":observed.Stdout,"stderr":observed.Stderr}}
  planRaw:=ctx.AgentTask("teach-and-plan","Teach the relevant Yield primitives, then return the exact files and commands proposed. For register, explain adapters. For repair, use doctor evidence. For upgrade, target the installed Yield version and include dependency and lockfile changes. Do not edit files or run commands.",map[string]any{"mode":mode,"spec":spec,"target":target,"flow":flow,"inspection":inspection,"yield_version":cfg.YieldVersion},json.RawMessage(planSchema));var plan planResult;if err=json.Unmarshal(planRaw,&plan);err!=nil{return yield.Outcome{},err}
  if ctx.AskUser("approve-change",plan.Summary+" Apply this plan?",yield.Option{Value:"apply",Label:"Apply"},yield.Option{Value:"stop",Label:"Stop"})!="apply"{return yield.Outcome{},ctx.Refused("the developer declined the proposed Yield changes")}
  agents:=strings.Join(cfg.Agents,",");verify:="printf fixture-ok";register:="printf fixture-register-ok";verifyAdapters:="printf fixture-adapters-ok";if !fixture{verify="cd ../.. && "+cfg.Launcher+" doctor "+quote(target)+" --root . --test";register="cd ../.. && "+cfg.Launcher+" register "+quote(target)+" --root . --agent "+quote(agents);verifyAdapters="cd ../.. && "+cfg.Launcher+" doctor "+quote(target)+" --root . --agent "+quote(agents)+" --test"}
  if mode=="register"{checked:=ctx.RunCommand("verify-before-register",verify,600);ctx.Require(checked.ExitCode==0,"the workflow passes its fixture run",map[string]any{"exit_code":checked.ExitCode});registered:=ctx.RunCommand("register-workflow",register,120);ctx.Require(registered.ExitCode==0,"the workflow is registered",map[string]any{"exit_code":registered.ExitCode});adapters:=ctx.RunCommand("verify-adapters",verifyAdapters,600);ctx.Require(adapters.ExitCode==0,"the generated adapters pass verification",map[string]any{"exit_code":adapters.ExitCode});return ctx.Complete(map[string]any{"mode":mode,"target":target,"changed":true,"verified":true})}
  task:="write-workflow";prompt:="Create the complete Yield workflow at the destination. Follow every semantic disposition. Write the program, SKILL.md, exact-version dependencies, skill.json, and self-contained fixtures. Keep useful model-facing guidance in SKILL.md; remove only duplicated sequencing. Do not edit outside the destination.";if !creates{task=mode+"-workflow";prompt="Apply only the approved operation to the target. For upgrade, update its manifest and language dependency to the supplied exact Yield version. Do not edit outside the target. Return every changed file."}
  writtenRaw:=ctx.AgentTask(task,prompt,map[string]any{"mode":mode,"spec":spec,"target":target,"source":source,"projection":projection,"flow":flow,"plan":json.RawMessage(planRaw),"yield_version":cfg.YieldVersion},json.RawMessage(filesSchema));var written fileResult;if err=json.Unmarshal(writtenRaw,&written);err!=nil{return yield.Outcome{},err};if !filesStayInside(written.Files,target){return yield.Outcome{},ctx.Blocked("the reported changes escape the approved workflow directory")}
  checked:=ctx.RunCommand("verify-workflow",verify,600);for attempt:=1;checked.ExitCode!=0&&attempt<=2;attempt++{repair:=ctx.AgentTask(fmt.Sprintf("repair-workflow-%d",attempt),"Verification failed. Repair only the approved workflow directory and return every changed file.",map[string]any{"mode":mode,"target":target,"stdout":checked.Stdout,"stderr":checked.Stderr},json.RawMessage(filesSchema));if err=json.Unmarshal(repair,&written);err!=nil{return yield.Outcome{},err};if !filesStayInside(written.Files,target){return yield.Outcome{},ctx.Blocked("the reported repair escapes the approved workflow directory")};checked=ctx.RunCommand(fmt.Sprintf("verify-workflow-retry-%d",attempt),verify,600)}
  if checked.ExitCode!=0{return yield.Outcome{},ctx.Blocked("the workflow still fails after two repair attempts")};ctx.Require(true,"the workflow passes its fixture run",map[string]any{"exit_code":checked.ExitCode});registered:=ctx.RunCommand("register-workflow",register,120);ctx.Require(registered.ExitCode==0,"the workflow is registered",map[string]any{"exit_code":registered.ExitCode});adapters:=ctx.RunCommand("verify-adapters",verifyAdapters,600);ctx.Require(adapters.ExitCode==0,"the generated adapters pass verification",map[string]any{"exit_code":adapters.ExitCode})
  return ctx.Complete(map[string]any{"mode":mode,"target":target,"language":spec.Language,"files":written.Files,"changed":true,"verified":true})
}) }
`

const bootstrapRust = `use serde_json::{json, Value};
use std::{fs, path::{Path, PathBuf}};
use yieldskill::{define_skill, Context, SkillResult};
const HELPER_PATH:&str="skills/yield-workflow-builder";
const SPEC_SCHEMA:&str=r#"{"type":"object","required":["name","description","language","destination","source_path","target_path","requested_version"],"additionalProperties":false,"properties":{"name":{"type":"string"},"description":{"type":"string"},"language":{"enum":["typescript","python","go","rust"]},"destination":{"type":"string"},"source_path":{"type":"string"},"target_path":{"type":"string"},"requested_version":{"type":"string"}}}"#;
const PROJECTION_SCHEMA:&str=r#"{"type":"object","required":["clauses","ready","unresolved"],"additionalProperties":false,"properties":{"clauses":{"type":"array","minItems":1,"items":{"type":"object","required":["source_clause","disposition","destinations","reason"],"additionalProperties":false,"properties":{"source_clause":{"type":"string","minLength":1},"disposition":{"enum":["control","guidance","both","excluded"]},"destinations":{"type":"array","items":{"type":"object","required":["kind","target"],"additionalProperties":false,"properties":{"kind":{"enum":["code","skill","agent_task"]},"target":{"type":"string","minLength":1}}}},"reason":{"type":"string"}}}},"ready":{"type":"boolean"},"unresolved":{"type":"array","items":{"type":"string","minLength":1}}}}"#;
const FLOW_SCHEMA:&str=r#"{"type":"object","required":["summary","steps"],"additionalProperties":false,"properties":{"summary":{"type":"string","minLength":1},"steps":{"type":"array","minItems":1,"items":{"type":"object","required":["id","kind","description"],"additionalProperties":false,"properties":{"id":{"type":"string","minLength":1},"kind":{"enum":["ask_user","agent_task","run_command","branch","require"]},"description":{"type":"string","minLength":1}}}}}}"#;
const FILES_SCHEMA:&str=r#"{"type":"object","required":["files"],"additionalProperties":false,"properties":{"files":{"type":"array","minItems":1,"items":{"type":"string","minLength":1}}}}"#;
const PLAN_SCHEMA:&str=r#"{"type":"object","required":["summary","primitives","files","commands"],"additionalProperties":false,"properties":{"summary":{"type":"string","minLength":1},"primitives":{"type":"array","minItems":1,"items":{"type":"string","minLength":1}},"files":{"type":"array","minItems":1,"items":{"type":"string","minLength":1}},"commands":{"type":"array","minItems":1,"items":{"type":"string","minLength":1}}}}"#;
const GUIDE_SCHEMA:&str=r#"{"type":"object","required":["summary","primitives","manual_steps","docs"],"additionalProperties":false,"properties":{"summary":{"type":"string","minLength":1},"primitives":{"type":"array","minItems":1,"items":{"type":"string","minLength":1}},"manual_steps":{"type":"array","minItems":1,"items":{"type":"string","minLength":1}},"docs":{"type":"array","minItems":1,"items":{"type":"string","minLength":1}}}}"#;
fn safe(root:&Path,value:&str,destination:bool)->bool{let p=Path::new(value);if value.is_empty()||p.is_absolute()||p.components().any(|c|matches!(c,std::path::Component::ParentDir))||p.components().next().map(|c|c.as_os_str()!="skills").unwrap_or(true)||p.components().count()<=1{return false}let joined=root.join(p);let probe=if destination{joined.parent().unwrap_or(root)}else{joined.as_path()};probe.canonicalize().map(|resolved|resolved.starts_with(root)).unwrap_or(false)}
fn valid_name(value:&str)->bool{!value.is_empty()&&value.split('-').all(|part|!part.is_empty()&&part.bytes().all(|b|b.is_ascii_lowercase()||b.is_ascii_digit()))}
fn quote(value:&str)->String{format!("'{}'",value.replace('\'',"'\\''"))}
fn files_stay_inside(value:&Value,target:&str)->bool{value["files"].as_array().map(|files|files.iter().all(|file|file.as_str().map(|path|{let p=Path::new(path);p==Path::new(target)||p.starts_with(Path::new(target))}).unwrap_or(false))).unwrap_or(false)}
fn program(ctx:&mut Context)->SkillResult{
 let cfg:Value=serde_json::from_slice(&fs::read("builder.json").expect("builder.json must be readable")).expect("builder.json must be valid JSON");
 let mode=ctx.ask_user("select-mode","What do you want to do with Yield?",&[("learn","Learn"),("create","Create"),("convert","Convert"),("check","Check"),("repair","Repair"),("upgrade","Upgrade"),("register","Register")]);
 if mode=="learn"{let guide=ctx.agent_task("teach-yield","Teach the smallest relevant Yield concept for the user's request. Explain what remains in SKILL.md, what moves into code, fixtures, doctor, and registration. Give manual steps before mentioning this helper. Do not edit files or run commands.",Some(json!({"mode":mode})),Some(serde_json::from_str(GUIDE_SCHEMA).unwrap()));return Ok(json!({"mode":mode,"changed":false,"guide":guide}))}
 let spec=ctx.agent_task("collect-specification","Use the user's current request. For create or convert, return a safe kebab-case name, description, target language, new destination under skills/, and source_path for convert. For check, repair, upgrade, or register, return target_path for one existing workflow under skills/. Set requested_version only when the user explicitly requests one. Use empty strings for fields that do not apply. Do not write files.",Some(json!({"mode":mode})),Some(serde_json::from_str(SPEC_SCHEMA).unwrap()));
 let root=PathBuf::from("../..").canonicalize().expect("repository root must be readable");let creates=mode=="create"||mode=="convert";let target=if creates{spec["destination"].as_str().unwrap_or("")}else{spec["target_path"].as_str().unwrap_or("")};
 if creates{if !valid_name(spec["name"].as_str().unwrap_or(""))||spec["description"].as_str().unwrap_or("").is_empty()||!safe(&root,target,true)||target==HELPER_PATH{return Err(ctx.blocked("the destination must be a new named path under skills/"))}let available=ctx.run_command("check-destination",&format!("cd ../.. && test ! -e {}",quote(target)),30);if available.exit_code!=0{return Err(ctx.blocked("the destination must be a new path under skills/"))}}else if !safe(&root,target,false){return Err(ctx.blocked("the target workflow must be an existing path under skills/"))}
 if mode=="upgrade"&&target==HELPER_PATH{return Err(ctx.blocked("the helper cannot upgrade itself during an active run; exit and run yskill helper install"))}
 if mode=="upgrade"&&!spec["requested_version"].as_str().unwrap_or("").is_empty()&&spec["requested_version"]!=cfg["yield_version"]{return Err(ctx.blocked("unsupported Yield version change; exit and run yskill helper install for the requested version"))}let fixture=target=="skills/yield-workflow-builder-fixture";
 let launcher=cfg["launcher"].as_str().unwrap();if mode=="check"{let command=if fixture{"printf fixture-check-ok".to_string()}else{format!("cd ../.. && {} doctor {} --root .",launcher,quote(target))};let checked=ctx.run_command("check-workflow",&command,120);return Ok(json!({"mode":mode,"target":target,"changed":false,"healthy":checked.exit_code==0,"stdout":checked.stdout,"stderr":checked.stderr}))}
 let mut source=String::new();let mut projection=Value::Null;if mode=="convert"{let source_path=spec["source_path"].as_str().unwrap_or("");if !safe(&root,source_path,false){return Err(ctx.blocked("the source skill must be inside the repository"))};source=fs::read_to_string(root.join(source_path).join("SKILL.md")).map_err(|_|ctx.blocked("the source SKILL.md does not exist"))?;projection=ctx.agent_task("project-semantics","Map every source clause exactly once using Pi(S)={(c,d,T,r)|c in clauses(S)}. YAML frontmatter is metadata, not a clause. Each top-level bullet is exactly one clause, including a compound sentence. For prose without bullets, treat each paragraph as one clause. Use disposition control, guidance, both, or excluded. Control needs a reachable code destination. Guidance needs a reachable skill or agent_task destination. Both needs both kinds. Excluded needs no destination and a non-empty reason. Name concrete destinations that the writer can create. Use the target language's canonical entrypoint for code: main.ts, main.py, main.go, or src/main.rs. Report uncertainty in unresolved and set ready false. Do not write files.",Some(json!({"source":source,"spec":spec})),Some(serde_json::from_str(PROJECTION_SCHEMA).unwrap()));if !projection["ready"].as_bool().unwrap_or(false)||projection["unresolved"].as_array().map(|v|!v.is_empty()).unwrap_or(true){return Err(ctx.blocked("the semantic projection has unresolved source clauses"))}}
 let flow=if creates{ctx.agent_task("extract-flow","Extract or design the minimal workflow. Keep model judgment in agent_task. Put order, branches, commands, approvals, evidence, and finish rules in code.",Some(json!({"mode":mode,"spec":spec,"source":source,"projection":projection})),Some(serde_json::from_str(FLOW_SCHEMA).unwrap()))}else{Value::Null};
 let inspection=if creates{Value::Null}else{let command=if fixture{"printf fixture-inspect-ok".to_string()}else{format!("cd ../.. && {} doctor {} --root .",launcher,quote(target))};let observed=ctx.run_command("inspect-workflow",&command,120);json!({"exit_code":observed.exit_code,"stdout":observed.stdout,"stderr":observed.stderr})};
 let plan=ctx.agent_task("teach-and-plan","Teach the relevant Yield primitives, then return the exact files and commands proposed. For register, explain adapters. For repair, use doctor evidence. For upgrade, target the installed Yield version and include dependency and lockfile changes. Do not edit files or run commands.",Some(json!({"mode":mode,"spec":spec,"target":target,"flow":flow,"inspection":inspection,"yield_version":cfg["yield_version"]})),Some(serde_json::from_str(PLAN_SCHEMA).unwrap()));
 let approval=ctx.ask_user("approve-change",&format!("{} Apply this plan?",plan["summary"].as_str().unwrap_or("Apply the proposed Yield changes.")),&[("apply","Apply"),("stop","Stop")]);if approval!="apply"{return Err(ctx.refused("the developer declined the proposed Yield changes"))}
 let agents=cfg["agents"].as_array().unwrap().iter().filter_map(|v|v.as_str()).collect::<Vec<_>>().join(",");let verify=if fixture{"printf fixture-ok".to_string()}else{format!("cd ../.. && {} doctor {} --root . --test",launcher,quote(target))};let register=if fixture{"printf fixture-register-ok".to_string()}else{format!("cd ../.. && {} register {} --root . --agent {}",launcher,quote(target),quote(&agents))};let verify_adapters=if fixture{"printf fixture-adapters-ok".to_string()}else{format!("cd ../.. && {} doctor {} --root . --agent {} --test",launcher,quote(target),quote(&agents))};
 if mode=="register"{let checked=ctx.run_command("verify-before-register",&verify,600);ctx.require(checked.exit_code==0,"the workflow passes its fixture run",Some(&json!({"exit_code":checked.exit_code})));let registered=ctx.run_command("register-workflow",&register,120);ctx.require(registered.exit_code==0,"the workflow is registered",Some(&json!({"exit_code":registered.exit_code})));let adapters=ctx.run_command("verify-adapters",&verify_adapters,600);ctx.require(adapters.exit_code==0,"the generated adapters pass verification",Some(&json!({"exit_code":adapters.exit_code})));return Ok(json!({"mode":mode,"target":target,"changed":true,"verified":true}))}
 let task=if creates{"write-workflow"}else if mode=="repair"{"repair-workflow"}else{"upgrade-workflow"};let prompt=if creates{"Create the complete Yield workflow at the destination. Follow every semantic disposition. Write the program, SKILL.md, exact-version dependencies, skill.json, and self-contained fixtures. Keep useful model-facing guidance in SKILL.md; remove only duplicated sequencing. Do not edit outside the destination."}else{"Apply only the approved operation to the target. For upgrade, update its manifest and language dependency to the supplied exact Yield version. Do not edit outside the target. Return every changed file."};let mut written=ctx.agent_task(task,prompt,Some(json!({"mode":mode,"spec":spec,"target":target,"source":source,"projection":projection,"flow":flow,"plan":plan,"yield_version":cfg["yield_version"]})),Some(serde_json::from_str(FILES_SCHEMA).unwrap()));if !files_stay_inside(&written,target){return Err(ctx.blocked("the reported changes escape the approved workflow directory"))}
 let mut checked=ctx.run_command("verify-workflow",&verify,600);for attempt in 1..=2{if checked.exit_code==0{break}written=ctx.agent_task(&format!("repair-workflow-{attempt}"),"Verification failed. Repair only the approved workflow directory and return every changed file.",Some(json!({"mode":mode,"target":target,"stdout":checked.stdout,"stderr":checked.stderr})),Some(serde_json::from_str(FILES_SCHEMA).unwrap()));if !files_stay_inside(&written,target){return Err(ctx.blocked("the reported repair escapes the approved workflow directory"))}checked=ctx.run_command(&format!("verify-workflow-retry-{attempt}"),&verify,600)}
 if checked.exit_code!=0{return Err(ctx.blocked("the workflow still fails after two repair attempts"))}ctx.require(true,"the workflow passes its fixture run",Some(&json!({"exit_code":checked.exit_code})));let registered=ctx.run_command("register-workflow",&register,120);ctx.require(registered.exit_code==0,"the workflow is registered",Some(&json!({"exit_code":registered.exit_code})));let adapters=ctx.run_command("verify-adapters",&verify_adapters,600);ctx.require(adapters.exit_code==0,"the generated adapters pass verification",Some(&json!({"exit_code":adapters.exit_code})));
 Ok(json!({"mode":mode,"target":target,"language":spec["language"],"files":written["files"],"changed":true,"verified":true}))
}
fn main(){define_skill(program);}
`
