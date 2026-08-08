package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
	if err := cmdBootstrap([]string{"--root", root, "--language", "typescript", "--agent", "codex", "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "skills")); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote skills directory: %v", err)
	}
}

func TestBootstrapCancellationDoesNotWrite(t *testing.T) {
	withBootstrapTestState(t)
	bootstrapInput = bytes.NewBufferString("no\n")
	root := t.TempDir()
	if err := cmdBootstrap([]string{"--root", root, "--language", "python", "--agent", "codex"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "skills")); !os.IsNotExist(err) {
		t.Fatalf("cancelled bootstrap wrote skills directory: %v", err)
	}
}

func TestBootstrapWritesBuilderProfileAndAdapter(t *testing.T) {
	withBootstrapTestState(t)
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
	want := []string{"select-mode", "collect-specification", "extract-flow", "write-workflow", "verify-generated", "repair-generated-", "register-generated", "verify-adapters"}
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

func TestBuilderFixturesReachCompletedAcrossLanguages(t *testing.T) {
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
			if err := cmdDoctor([]string{dir, "--root", root, "--test"}); err != nil {
				t.Fatalf("%s builder fixture did not complete: %v", language, err)
			}
		})
	}
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
