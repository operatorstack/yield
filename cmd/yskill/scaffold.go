package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var releaseVersionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$`)
var tidyGoModule = func(dir string) error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("prepare Go scaffold dependencies: %w", err)
	}
	return nil
}

func defaultLanguage() string {
	if language := strings.TrimSpace(os.Getenv("YIELD_LANGUAGE")); language != "" {
		return language
	}
	return "go"
}

func packageVersion() string {
	v := runtimeVersion()
	if releaseVersionPattern.MatchString(v) {
		return v
	}
	return "0.0.0"
}

func scaffoldSkill(dir, language, sdkPath, description string) error {
	language = strings.ToLower(strings.TrimSpace(language))
	if language != "typescript" && language != "python" && language != "go" && language != "rust" {
		return fmt.Errorf("unsupported language %q; choose typescript, python, go, or rust", language)
	}
	if sdkPath != "" && language != "go" {
		return fmt.Errorf("--sdk is only valid with --language go")
	}
	name := filepath.Base(filepath.Clean(dir))
	if !portableSkillName.MatchString(name) || len(name) > 64 {
		return fmt.Errorf("skill directory name must match [a-z0-9]+(-[a-z0-9]+)* and be at most 64 characters")
	}
	skillPath := filepath.Join(dir, "SKILL.md")
	if _, err := os.Stat(skillPath); os.IsNotExist(err) && strings.TrimSpace(description) == "" {
		return fmt.Errorf("--description is required for a new skill; describe what it does and when an agent should use it")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	writeIfAbsent := func(rel, content string) error {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("init: %s exists, preserved\n", rel)
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, []byte(content), 0o644)
	}

	launcher := map[string]string{
		"typescript": "npm exec -- yskill",
		"python":     "python -m yieldskill",
		"go":         "yskill",
		"rust":       "yskill",
	}[language]
	files := scaffoldFiles(name, language, sdkPath)
	files["SKILL.md"] = fmt.Sprintf(skillMD, name, yamlString(strings.TrimSpace(description)), launcher, launcher)
	files["fixtures/responses.json"] = "{\n  \"confirm-start\": {\"value\": \"yes\"}\n}\n"
	files["fixtures/test.json"] = "{\n  \"version\": 1,\n  \"setup\": [],\n  \"after_response\": {},\n  \"teardown\": []\n}\n"
	keys := make([]string, 0, len(files))
	for key := range files {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, rel := range keys {
		if err := writeIfAbsent(rel, files[rel]); err != nil {
			return err
		}
	}
	if language == "go" {
		if err := tidyGoModule(dir); err != nil {
			return err
		}
	}
	if _, err := readSkillMetadata(dir); err != nil {
		return fmt.Errorf("validate SKILL.md: %w", err)
	}
	fmt.Printf("init: %s skill %q scaffolded in %s\n", language, name, dir)
	fmt.Println("next: replace the starter program and fixtures with the described workflow")
	fmt.Printf("test: %s doctor %s --test\n", launcher, shellQuote(dir))
	fmt.Printf("then: %s register %s\n", launcher, shellQuote(dir))
	return nil
}

func scaffoldFiles(name, language, sdkPath string) map[string]string {
	v := packageVersion()
	switch language {
	case "typescript":
		return map[string]string{
			"main.ts": mainTypeScript,
			"package.json": fmt.Sprintf(`{
  "private": true,
  "type": "module",
  "dependencies": { "@operatorstack/yield": "%s" }
}
`, v),
			"skill.json": "{\"version\":1,\"language\":\"typescript\",\"run\":[\"node\",\"main.ts\"]}\n",
		}
	case "python":
		return map[string]string{
			"main.py":          mainPython,
			"requirements.txt": fmt.Sprintf("--index-url https://get.operatorstack.systems/pip/simple/\nyieldskill==%s\n", v),
			"skill.json":       "{\"version\":1,\"language\":\"python\",\"run\":[\"python\",\"main.py\"]}\n",
		}
	case "rust":
		return map[string]string{
			".cargo/config.toml": "[registries.operatorstack]\nindex = \"sparse+https://get.operatorstack.systems/cargo/index/\"\n",
			"Cargo.toml":         fmt.Sprintf("[package]\nname = %q\nversion = \"0.1.0\"\nedition = \"2021\"\n\n[dependencies]\nyieldskill = { version = \"=%s\", registry = \"operatorstack\" }\nserde_json = \"1\"\n", name, v),
			"src/main.rs":        mainRust,
			"skill.json":         "{\"version\":1,\"language\":\"rust\",\"run\":[\"cargo\",\"run\",\"--quiet\"]}\n",
		}
	default:
		gomod := fmt.Sprintf("module %s\n\ngo 1.26.5\n\nrequire github.com/operatorstack/yield v%s\n", name, v)
		if sdkPath != "" {
			gomod += fmt.Sprintf("\nreplace github.com/operatorstack/yield => %s\n", sdkPath)
		}
		return map[string]string{
			"main.go":    mainGo,
			"go.mod":     gomod,
			"skill.json": "{\"version\":1,\"language\":\"go\",\"run\":[\"go\",\"run\",\"-mod=readonly\",\".\"]}\n",
		}
	}
}

const skillMD = `---
name: %s
description: %s
---

Run:

    %s run .

Follow each returned operation exactly. Answer it directly:

	%s respond <run-id> --value <answer> --skill .

Do not skip an operation or invent a response.
`

const mainGo = `package main

import "github.com/operatorstack/yield/sdk/yield"

func main() {
	yield.Main(func(ctx *yield.Context) (yield.Outcome, error) {
		answer := ctx.AskUser("confirm-start", "Ready to start?")
		if answer != "yes" {
			return yield.Outcome{}, ctx.Refused("user declined to start")
		}
		return yield.Outcome{}, ctx.Blocked("replace the starter workflow and fixture before testing")
	})
}
`

const mainTypeScript = `import { defineSkill } from "@operatorstack/yield"

defineSkill((ctx) => {
  const answer = ctx.askUser("confirm-start", "Ready to start?", [
    { value: "yes" }, { value: "no" }
  ])
  if (answer !== "yes") ctx.refused("user declined to start")
  ctx.blocked("replace the starter workflow and fixture before testing")
})
`

const mainPython = `from yieldskill import define_skill

def program(ctx):
    answer = ctx.ask_user("confirm-start", "Ready to start?", options=[{"value": "yes"}, {"value": "no"}])
    if answer != "yes":
        ctx.refused("user declined to start")
    ctx.blocked("replace the starter workflow and fixture before testing")

define_skill(program)
`

const mainRust = `use serde_json::json;
use yieldskill::{define_skill, Context, SkillResult};

fn program(ctx: &mut Context) -> SkillResult {
    let answer = ctx.ask_user("confirm-start", "Ready to start?", &[("yes", "Yes"), ("no", "No")]);
    if answer != "yes" {
        return Err(ctx.refused("user declined to start"));
    }
    Err(ctx.blocked("replace the starter workflow and fixture before testing"))
}

fn main() { define_skill(program); }
`
