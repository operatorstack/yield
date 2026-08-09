package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/operatorstack/yield/internal/protocol"
)

func withBootstrapTestState(t *testing.T) {
	t.Helper()
	oldVersion := version
	oldInput := bootstrapInput
	oldCommand := bootstrapCommand
	oldDoctor := bootstrapDoctor
	version = "1.2.3"
	bootstrapInput = strings.NewReader("yes\n")
	bootstrapCommand = func(string, string, ...string) error { return nil }
	bootstrapDoctor = func(string, string, []string) error { return nil }
	t.Cleanup(func() {
		version = oldVersion
		bootstrapInput = oldInput
		bootstrapCommand = oldCommand
		bootstrapDoctor = oldDoctor
	})
}

func TestBootstrapDryRunDoesNotWrite(t *testing.T) {
	withBootstrapTestState(t)
	root := t.TempDir()
	var runErr error
	output := captureStdout(t, func() {
		runErr = cmdBootstrap([]string{"--root", root, "--language", "typescript", "--agent", "codex", "--dry-run"})
	})
	if runErr != nil {
		t.Fatal(runErr)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"helper install plan: language=typescript root=" + resolvedRoot,
		filepath.Join(resolvedRoot, "skills", bootstrapSkillName),
		"npm 'install' '--ignore-scripts' '--no-audit' '--no-fund'",
		"yskill 'register'",
		"'--agent' 'codex'",
		"helper: dry run complete; no files changed",
	} {
		if !strings.Contains(output, required) {
			t.Errorf("dry-run output is missing %q:\n%s", required, output)
		}
	}
	if strings.Count(output, "yskill 'doctor'") != 2 || strings.Index(output, "yskill 'register'") > strings.LastIndex(output, "yskill 'doctor'") {
		t.Errorf("dry-run operations do not match execution order:\n%s", output)
	}
	if strings.Contains(output, "Apply this bootstrap plan") {
		t.Errorf("dry-run uses obsolete bootstrap wording:\n%s", output)
	}
	if _, err := os.Stat(filepath.Join(root, "skills")); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote skills directory: %v", err)
	}
}

func TestHelperInstallUsesBootstrapContract(t *testing.T) {
	withBootstrapTestState(t)
	root := t.TempDir()
	if err := cmdHelper([]string{"install", "--root", root, "--language", "typescript", "--agent", "codex", "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "skills")); !os.IsNotExist(err) {
		t.Fatalf("helper dry run wrote skills directory: %v", err)
	}
	if err := cmdHelper(nil); err == nil || !strings.Contains(err.Error(), "requires a subcommand") {
		t.Fatalf("missing helper subcommand returned %v", err)
	}
}

func TestBootstrapCancellationDoesNotWrite(t *testing.T) {
	withBootstrapTestState(t)
	bootstrapInput = bytes.NewBufferString("no\n")
	root := t.TempDir()
	var runErr error
	output := captureStdout(t, func() {
		runErr = cmdBootstrap([]string{"--root", root, "--language", "python", "--agent", "codex"})
	})
	if runErr != nil {
		t.Fatal(runErr)
	}
	if !strings.Contains(output, "Apply this helper install plan? [y/N]") || strings.Contains(output, "Apply this bootstrap plan") {
		t.Fatalf("cancellation prompt is not helper-specific:\n%s", output)
	}
	if !strings.Contains(output, "helper: cancelled; no files changed") {
		t.Fatalf("cancellation result is missing:\n%s", output)
	}
	if _, err := os.Stat(filepath.Join(root, "skills")); !os.IsNotExist(err) {
		t.Fatalf("cancelled bootstrap wrote skills directory: %v", err)
	}
}

func TestBootstrapWritesBuilderProfileAndAdapter(t *testing.T) {
	withBootstrapTestState(t)
	var doctorAgents [][]string
	bootstrapDoctor = func(_ string, _ string, agents []string) error {
		doctorAgents = append(doctorAgents, append([]string(nil), agents...))
		return nil
	}
	root := t.TempDir()
	if err := cmdBootstrap([]string{"--root", root, "--language", "python", "--agent", "codex", "--yes"}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"skills/yield-workflow-builder/SKILL.md",
		"skills/yield-workflow-builder/main.py",
		".yield/bootstrap.json",
		".agents/skills/yield-workflow-builder/SKILL.md",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
			t.Fatalf("missing %s: %v", path, err)
		}
	}
	adapter, _ := os.ReadFile(filepath.Join(root, ".agents/skills/yield-workflow-builder/SKILL.md"))
	if !strings.Contains(string(adapter), "uvx --from 'yieldskill==1.2.3' yskill") {
		t.Fatalf("adapter does not use the pinned uvx launcher:\n%s", adapter)
	}
	if err := cmdBootstrap([]string{"--root", root, "--language", "python", "--agent", "codex", "--yes"}); err != nil {
		t.Fatalf("idempotent bootstrap failed: %v", err)
	}
	if len(doctorAgents) != 4 || len(doctorAgents[0]) != 0 || len(doctorAgents[1]) != 1 || doctorAgents[1][0] != "codex" {
		t.Fatalf("bootstrap must verify the workflow before registration and adapters after it: %#v", doctorAgents)
	}
}

func TestBootstrapRefusesForeignSkillAndAdapter(t *testing.T) {
	withBootstrapTestState(t)
	for _, collision := range []string{
		"skills/yield-workflow-builder/SKILL.md",
		".agents/skills/yield-workflow-builder/SKILL.md",
	} {
		root := t.TempDir()
		path := filepath.Join(root, filepath.FromSlash(collision))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("user owned\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		err := cmdBootstrap([]string{"--root", root, "--language", "typescript", "--agent", "codex", "--yes"})
		if err == nil || !strings.Contains(err.Error(), "refusing to overwrite user-owned") {
			t.Fatalf("collision %s returned %v", collision, err)
		}
	}
}

func TestBootstrapRefusesAdapterSymlinkEscape(t *testing.T) {
	withBootstrapTestState(t)
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".agents")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	err := cmdBootstrap([]string{"--root", root, "--language", "typescript", "--agent", "codex", "--yes"})
	if err == nil || !strings.Contains(err.Error(), "outside repository") {
		t.Fatalf("symlink escape returned %v", err)
	}
}

func TestBuilderTemplatesExposeEquivalentOperations(t *testing.T) {
	profile := bootstrapProfile{YieldVersion: "1.2.3", Agents: []string{"codex"}}
	want := []string{"learn", "create", "convert", "check", "repair", "upgrade", "register", "select-mode", "teach-yield", "collect-specification", "source skill directory under skills/, not the SKILL.md file", "check-destination", "project-semantics", "extract-flow", "teach-and-plan", "approve-change", "write-workflow", "verify-workflow", "repair-workflow-", "register-workflow", "verify-adapters", "yskill helper install"}
	projectionContract := []string{"source_clause", "disposition", "destinations", "reason", "control", "guidance", "both", "excluded", "ready", "unresolved"}
	repairLimit := map[string]string{"typescript": "attempt<=2", "python": "range(1, 3)", "go": "attempt<=2", "rust": "1..=2"}
	for _, language := range []string{"typescript", "python", "go", "rust"} {
		files, _, err := renderBootstrapSkill(language, profile)
		if err != nil {
			t.Fatal(err)
		}
		var program string
		for _, path := range []string{"main.ts", "main.py", "main.go", "src/main.rs"} {
			if files[path] != "" {
				program = files[path]
			}
		}
		for _, operation := range want {
			if !strings.Contains(program, operation) {
				t.Errorf("%s builder is missing operation %s", language, operation)
			}
		}
		for _, field := range projectionContract {
			if !strings.Contains(program, field) {
				t.Errorf("%s builder projection is missing %s", language, field)
			}
		}
		if !strings.Contains(program, repairLimit[language]) || strings.Contains(program, "repair-workflow-3") {
			t.Errorf("%s builder does not enforce the two-attempt repair limit", language)
		}
		skill := files["SKILL.md"]
		parts := strings.SplitN(skill, "---", 3)
		if len(parts) != 3 || strings.Count(parts[1], "\n") != 3 || !strings.Contains(parts[1], "\nname: yield-workflow-builder\n") || !strings.Contains(parts[1], "\ndescription: ") {
			t.Errorf("%s generated SKILL.md frontmatter is invalid: %q", language, parts[1])
		}
		if strings.Count(skill, "\n") >= 500 {
			t.Errorf("%s generated SKILL.md exceeds the concise skill limit", language)
		}
		for _, downstream := range []string{"source,projection", `"source":source,"projection":projection`} {
			if strings.Contains(program, downstream) {
				goto hasProjectionContext
			}
		}
		t.Errorf("%s builder does not pass source and projection downstream", language)
	hasProjectionContext:
	}
}

func TestBootstrapRustTemplateAddsAndPreservesSkillGitignore(t *testing.T) {
	profile := bootstrapProfile{YieldVersion: "1.2.3", Agents: []string{"codex"}}
	files, _, err := renderBootstrapSkill("rust", profile)
	if err != nil {
		t.Fatal(err)
	}
	if got := files[".gitignore"]; got != rustSkillGitignore {
		t.Fatalf("Rust bootstrap .gitignore = %q, want %q", got, rustSkillGitignore)
	}
	for _, language := range []string{"typescript", "python", "go"} {
		files, _, err := renderBootstrapSkill(language, profile)
		if err != nil {
			t.Fatal(err)
		}
		if _, found := files[".gitignore"]; found {
			t.Fatalf("%s bootstrap template created Rust-specific .gitignore", language)
		}
	}

	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", bootstrapSkillName)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const existing = "user-owned-rule/\n"
	if err := os.WriteFile(filepath.Join(skillDir, ".gitignore"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := bootstrapPlan{
		Root:     root,
		Language: "python",
		SkillDir: skillDir,
		Profile:  bootstrapProfile{Version: 1, YieldVersion: "1.2.3", Language: "python"},
		Files:    map[string]string{".gitignore": rustSkillGitignore},
	}
	if err := applyBootstrapPlan(plan); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, filepath.Join(skillDir, ".gitignore")); got != existing {
		t.Fatalf("existing bootstrap .gitignore changed: %q", got)
	}
}

func TestBuilderCreateFixtureDoesNotRequireProjection(t *testing.T) {
	if strings.Contains(bootstrapFixtureResponses, `"project-semantics"`) {
		t.Fatal("create mode fixture must remain unchanged by conversion projection")
	}
}

func TestBootstrapDetectsOneLanguageAndRejectsAmbiguity(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pyproject.toml"), []byte("[project]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := detectBootstrapLanguage(root); err != nil || got != "python" {
		t.Fatalf("detected %q, %v", got, err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := detectBootstrapLanguage(root); err == nil || !strings.Contains(err.Error(), "multiple project languages") {
		t.Fatalf("ambiguous detection returned %v", err)
	}
}

func TestBuilderTemplatesCompile(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	profile := bootstrapProfile{YieldVersion: "0.0.0", Agents: []string{"codex"}}
	tests := []struct {
		language string
		command  string
		args     []string
	}{
		{language: "typescript", command: "node", args: []string{"--check", "main.ts"}},
		{language: "python", command: "python3", args: []string{"-m", "py_compile", "main.py"}},
		{language: "go", command: "go", args: []string{"test", "./..."}},
		{language: "rust", command: "cargo", args: []string{"check", "--quiet"}},
	}
	for _, test := range tests {
		t.Run(test.language, func(t *testing.T) {
			if _, err := exec.LookPath(test.command); err != nil {
				t.Skipf("%s is unavailable", test.command)
			}
			dir := t.TempDir()
			files, _, err := renderBootstrapSkill(test.language, profile)
			if err != nil {
				t.Fatal(err)
			}
			if test.language == "go" {
				files["go.mod"] += "\nreplace github.com/operatorstack/yield => " + filepath.ToSlash(repoRoot) + "\n"
			}
			if test.language == "rust" {
				files["Cargo.toml"] = strings.Replace(files["Cargo.toml"], `yieldskill = { version = "=0.0.0" }`, `yieldskill = { path = "`+filepath.ToSlash(filepath.Join(repoRoot, "sdk", "rust"))+`" }`, 1)
			}
			for path, content := range files {
				full := filepath.Join(dir, filepath.FromSlash(path))
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if test.language == "go" {
				command := exec.Command("go", "mod", "tidy")
				command.Dir = dir
				if output, err := command.CombinedOutput(); err != nil {
					t.Fatalf("prepare go template: %v\n%s", err, output)
				}
			}
			command := exec.Command(test.command, test.args...)
			command.Dir = dir
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("%s template does not compile: %v\n%s", test.language, err, output)
			}
		})
	}
}

func TestBuilderModeFixturesAcrossLanguages(t *testing.T) {
	oldVersion := version
	version = "0.1.0"
	t.Cleanup(func() { version = oldVersion })
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	profile := bootstrapProfile{YieldVersion: "0.1.0", Agents: []string{"codex"}}
	for _, language := range []string{"typescript", "python", "go", "rust"} {
		t.Run(language, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "skills", bootstrapSkillName)
			files, _, err := renderBootstrapSkill(language, profile)
			if err != nil {
				t.Fatal(err)
			}
			if language == "go" {
				files["go.mod"] += "\nreplace github.com/operatorstack/yield => " + filepath.ToSlash(repoRoot) + "\n"
			}
			if language == "rust" {
				files["Cargo.toml"] = strings.Replace(files["Cargo.toml"], `yieldskill = { version = "=0.1.0" }`, `yieldskill = { version = "=0.1.0", path = "`+filepath.ToSlash(filepath.Join(repoRoot, "sdk", "rust"))+`" }`, 1)
			}
			for path, content := range files {
				full := filepath.Join(dir, filepath.FromSlash(path))
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			switch language {
			case "typescript":
				runTestCommand(t, filepath.Join(repoRoot, "sdk", "typescript"), "npm", "run", "build")
				module := filepath.Join(dir, "node_modules", "@operatorstack", "yield")
				if err := os.MkdirAll(filepath.Dir(module), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(repoRoot, "sdk", "typescript"), module); err != nil {
					t.Fatal(err)
				}
			case "python":
				old := os.Getenv("PYTHONPATH")
				t.Setenv("PYTHONPATH", filepath.Join(repoRoot, "sdk", "python")+string(os.PathListSeparator)+old)
				t.Setenv("PYTHONDONTWRITEBYTECODE", "1")
				python3, err := exec.LookPath("python3")
				if err != nil {
					t.Skip("python3 is unavailable")
				}
				bin := filepath.Join(root, ".yield", "test-bin")
				if err := os.MkdirAll(bin, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(python3, filepath.Join(bin, "python")); err != nil {
					t.Fatal(err)
				}
				t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			case "go":
				runTestCommand(t, dir, "go", "mod", "tidy")
			}
			if language == "go" || language == "rust" {
				runtimePath := localRuntimePath(root)
				if err := os.MkdirAll(filepath.Dir(runtimePath), 0o755); err != nil {
					t.Fatal(err)
				}
				runTestCommand(t, repoRoot, "go", "build", "-ldflags", "-X main.version=0.1.0", "-o", runtimePath, "./cmd/yskill")
			}
			source := filepath.Join(root, "skills", "source-fixture")
			if err := os.MkdirAll(source, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("---\nname: source-fixture\ndescription: Test then ask before publishing.\n---\n\nRun tests. Ask before publishing.\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			for _, mode := range []string{"learn", "create", "convert"} {
				writeBuilderResponses(t, dir, builderModeResponses(t, mode, "apply"))
				if err := cmdDoctor([]string{dir, "--root", root, "--test"}); err != nil {
					t.Fatalf("%s %s fixture did not complete: %v", language, mode, err)
				}
			}
			target := filepath.Join(root, "skills", "yield-workflow-builder-fixture")
			if err := os.MkdirAll(target, 0o755); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(target, "SKILL.md")
			if err := os.WriteFile(marker, []byte("fixture-owned\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			for _, mode := range []string{"check", "repair", "upgrade", "register"} {
				writeBuilderResponses(t, dir, builderModeResponses(t, mode, "apply"))
				if err := cmdDoctor([]string{dir, "--root", root, "--test"}); err != nil {
					t.Fatalf("%s %s fixture did not complete: %v", language, mode, err)
				}
			}
			declinedResponses := builderModeResponses(t, "repair", "stop")
			question := builderApprovalQuestion(t, dir, declinedResponses)
			for _, expected := range []string{
				"Apply the fixture plan.",
				"Primitives:\n- Require binds completion to evidence.",
				"Files:\n- skills/yield-workflow-builder-fixture/SKILL.md",
				"Commands:\n- yskill doctor --test",
				"Apply this plan?",
			} {
				if !strings.Contains(question, expected) {
					t.Errorf("%s approval question is missing %q:\n%s", language, expected, question)
				}
			}
			writeBuilderResponses(t, dir, declinedResponses)
			if err := cmdDoctor([]string{dir, "--root", root, "--test"}); err == nil || !strings.Contains(err.Error(), "terminal status refused") {
				t.Fatalf("%s declined mutation returned %v", language, err)
			}
			if got := readTestFile(t, marker); got != "fixture-owned\n" {
				t.Fatalf("%s declined mutation changed the target: %q", language, got)
			}
			unsupported := strings.Replace(builderModeResponses(t, "upgrade", "apply"), `"requested_version": "0.1.0"`, `"requested_version": "9.9.9"`, 1)
			writeBuilderResponses(t, dir, unsupported)
			if err := cmdDoctor([]string{dir, "--root", root, "--test"}); err == nil || !strings.Contains(err.Error(), "unsupported Yield version change") {
				t.Fatalf("%s unsupported upgrade returned %v", language, err)
			}
			selfUpgrade := strings.Replace(builderModeResponses(t, "upgrade", "apply"), `"target_path": "skills/yield-workflow-builder-fixture"`, `"target_path": "skills/yield-workflow-builder"`, 1)
			writeBuilderResponses(t, dir, selfUpgrade)
			if err := cmdDoctor([]string{dir, "--root", root, "--test"}); err == nil || !strings.Contains(err.Error(), "cannot upgrade itself") {
				t.Fatalf("%s self-upgrade returned %v", language, err)
			}
			escape := strings.Replace(builderModeResponses(t, "check", "apply"), `"target_path": "skills/yield-workflow-builder-fixture"`, `"target_path": "../outside"`, 1)
			writeBuilderResponses(t, dir, escape)
			if err := cmdDoctor([]string{dir, "--root", root, "--test"}); err == nil || !strings.Contains(err.Error(), "existing path under skills") {
				t.Fatalf("%s path escape returned %v", language, err)
			}
			writeBuilderResponses(t, dir, builderModeResponses(t, "create", "apply"))
			if err := cmdDoctor([]string{dir, "--root", root, "--test"}); err == nil || !strings.Contains(err.Error(), "new path under skills") {
				t.Fatalf("%s existing destination returned %v", language, err)
			}
		})
	}
}

func writeBuilderResponses(t *testing.T, dir, responses string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "fixtures", "responses.json"), []byte(responses), 0o644); err != nil {
		t.Fatal(err)
	}
}

func builderModeResponses(t *testing.T, mode, approval string) string {
	t.Helper()
	responses := map[string]any{"select-mode": map[string]string{"value": mode}}
	if mode == "learn" {
		responses["teach-yield"] = map[string]any{
			"summary": "Keep control flow in code.", "primitives": []string{"runCommand"},
			"manual_steps": []string{"init", "write fixtures", "doctor --test", "register"}, "docs": []string{"docs/quickstart.md"},
		}
		return marshalBuilderResponses(t, responses)
	}
	spec := map[string]any{
		"name": "", "description": "", "language": "go", "destination": "", "source_path": "",
		"target_path": "skills/yield-workflow-builder-fixture", "requested_version": "",
	}
	if mode == "create" || mode == "convert" {
		spec["name"] = "yield-workflow-builder-fixture"
		spec["description"] = "Create a harmless fixture workflow."
		spec["destination"] = "skills/yield-workflow-builder-fixture"
		if mode == "convert" {
			spec["source_path"] = "skills/source-fixture"
			responses["project-semantics"] = map[string]any{
				"clauses": []any{map[string]any{"source_clause": "Run tests.", "disposition": "control", "destinations": []any{map[string]string{"kind": "code", "target": "main"}}, "reason": "Executable gate."}},
				"ready":   true, "unresolved": []string{},
			}
		}
		responses["extract-flow"] = map[string]any{
			"summary": "Check then complete.", "steps": []any{map[string]string{"id": "check", "kind": "run_command", "description": "Run a check."}},
		}
	}
	if mode == "upgrade" {
		spec["requested_version"] = "0.1.0"
	}
	responses["collect-specification"] = spec
	if mode == "check" {
		return marshalBuilderResponses(t, responses)
	}
	responses["teach-and-plan"] = map[string]any{
		"summary": "Apply the fixture plan.", "primitives": []string{"Require binds completion to evidence."},
		"files": []string{"skills/yield-workflow-builder-fixture/SKILL.md"}, "commands": []string{"yskill doctor --test"},
	}
	responses["approve-change"] = map[string]string{"value": approval}
	if approval == "apply" && mode != "register" {
		task := mode + "-workflow"
		if mode == "create" || mode == "convert" {
			task = "write-workflow"
		}
		responses[task] = map[string]any{"files": []string{"skills/yield-workflow-builder-fixture/SKILL.md"}}
	}
	return marshalBuilderResponses(t, responses)
}

func marshalBuilderResponses(t *testing.T, responses map[string]any) string {
	t.Helper()
	b, err := json.MarshalIndent(responses, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(b) + "\n"
}

func builderApprovalQuestion(t *testing.T, dir, responses string) string {
	t.Helper()
	var script map[string]json.RawMessage
	if err := json.Unmarshal([]byte(responses), &script); err != nil {
		t.Fatal(err)
	}
	e, err := newEngineWithRunsDir(dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p, err := e.StartRun(nil)
	if err != nil {
		t.Fatal(err)
	}
	for p.Terminal == nil {
		if p.Envelope.Request.ID == "approve-change" {
			var payload protocol.AskUserPayload
			if err := json.Unmarshal(p.Envelope.Request.Payload, &payload); err != nil {
				t.Fatal(err)
			}
			return payload.Question
		}
		result, ok := script[p.Envelope.Request.ID]
		if !ok {
			t.Fatalf("no scripted response before approval for %q", p.Envelope.Request.ID)
		}
		response, err := json.Marshal(protocol.ResponseEnvelope{
			RunID: p.RunID, Sequence: p.Envelope.Sequence, RequestID: p.Envelope.Request.ID,
			Status: "completed", Result: result,
		})
		if err != nil {
			t.Fatal(err)
		}
		p, err = e.Resume(p.RunID, response, false)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("workflow completed before requesting mutation approval")
	return ""
}

func runTestCommand(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("%s is unavailable", name)
	}
	command := exec.Command(name, args...)
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, output)
	}
}
