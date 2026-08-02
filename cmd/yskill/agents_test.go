package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRegistryIsBroadValidAndVerifiedPathsArePinned(t *testing.T) {
	registry, err := loadAgentRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if len(registry.Agents) < 70 {
		t.Fatalf("agent registry has %d entries, want broad registry", len(registry.Agents))
	}
	want := map[string]string{
		"cursor":      ".cursor/skills",
		"codex":       ".agents/skills",
		"claude-code": ".claude/skills",
	}
	for _, agent := range registry.Agents {
		if path, ok := want[agent.ID]; ok {
			if agent.ProjectDir != path || agent.Tier != "verified" {
				t.Fatalf("%s = path %q tier %q", agent.ID, agent.ProjectDir, agent.Tier)
			}
			delete(want, agent.ID)
		}
	}
	if len(want) != 0 {
		t.Fatalf("verified agents missing: %v", want)
	}
}

func TestRegisterWritesThinAdaptersAndDeduplicatesSharedDestination(t *testing.T) {
	repo := t.TempDir()
	writeTestFile(t, filepath.Join(repo, ".git"), "gitdir: fixture\n")
	skill := createTypeScriptSkill(t, repo, "review")
	registrations, err := registerSkill(skill, repo, []string{"codex", "amp", "cursor", "claude-code"})
	if err != nil {
		t.Fatal(err)
	}
	if len(registrations) != 3 {
		t.Fatalf("registrations = %d, want three unique destinations: %+v", len(registrations), registrations)
	}
	for _, rel := range []string{
		".agents/skills/review/SKILL.md",
		".cursor/skills/review/SKILL.md",
		".claude/skills/review/SKILL.md",
	} {
		adapter := readTestFile(t, filepath.Join(repo, filepath.FromSlash(rel)))
		if !strings.Contains(adapter, "source: skills/review;") || !strings.Contains(adapter, "npm exec -- yskill run 'skills/review'") {
			t.Fatalf("adapter %s does not point to canonical workflow:\n%s", rel, adapter)
		}
		if strings.Contains(adapter, "defineSkill") || strings.Contains(adapter, "fixtures") {
			t.Fatalf("adapter %s duplicated workflow content", rel)
		}
	}
}

func TestEveryRegistryAgentGeneratesAContainedAdapter(t *testing.T) {
	repo := t.TempDir()
	writeTestFile(t, filepath.Join(repo, ".git"), "gitdir: fixture\n")
	writeTestFile(t, filepath.Join(repo, "package.json"), `{"dependencies":{"@operatorstack/yield":"0.1.17"}}`)
	skill := filepath.Join(repo, "workflows", "review")
	writeTestFile(t, filepath.Join(skill, "SKILL.md"), "---\nname: review\ndescription: Review the branch when the user wants code checked before shipping.\n---\n")
	writeTestFile(t, filepath.Join(skill, "skill.json"), `{"version":1,"language":"typescript","run":["node","main.ts"]}`)
	writeTestFile(t, filepath.Join(skill, "main.ts"), "export {}\n")
	registry, err := loadAgentRegistry()
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(registry.Agents))
	for _, agent := range registry.Agents {
		ids = append(ids, agent.ID)
	}
	registrations, err := registerSkill(skill, repo, ids)
	if err != nil {
		t.Fatal(err)
	}
	if len(registrations) == 0 || len(registrations) >= len(ids) {
		t.Fatalf("unique registrations = %d for %d agents; expected shared destinations to deduplicate", len(registrations), len(ids))
	}
	for _, registration := range registrations {
		path := filepath.Join(repo, filepath.FromSlash(registration.Path))
		if !within(repo, path) {
			t.Fatalf("adapter escaped repository: %s", path)
		}
		if _, err := readSkillMetadata(filepath.Dir(path)); err != nil {
			t.Fatalf("adapter %s is not a portable skill: %v", registration.Path, err)
		}
	}
}

func TestRegisterUpdatesOwnedAdapterAndRefusesForeignCollision(t *testing.T) {
	repo := t.TempDir()
	writeTestFile(t, filepath.Join(repo, ".git"), "gitdir: fixture\n")
	skill := createTypeScriptSkill(t, repo, "review")
	if _, err := registerSkill(skill, repo, []string{"codex"}); err != nil {
		t.Fatal(err)
	}
	canonical := filepath.Join(skill, "SKILL.md")
	updated := strings.Replace(readTestFile(t, canonical), "Review the branch", "Review changed code", 1)
	writeTestFile(t, canonical, updated)
	if _, err := registerSkill(skill, repo, []string{"codex"}); err != nil {
		t.Fatalf("update generated adapter: %v", err)
	}
	adapterPath := filepath.Join(repo, ".agents", "skills", "review", "SKILL.md")
	writeTestFile(t, adapterPath, "user owned\n")
	if _, err := registerSkill(skill, repo, []string{"codex"}); err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("foreign collision error = %v", err)
	}
}

func TestRegisterRejectsOutsideRepositoryAndAgentDirectoryCanonical(t *testing.T) {
	repo := t.TempDir()
	writeTestFile(t, filepath.Join(repo, ".git"), "gitdir: fixture\n")
	outside := createTypeScriptSkill(t, t.TempDir(), "outside")
	if _, err := registerSkill(outside, repo, []string{"codex"}); err == nil || !strings.Contains(err.Error(), "inside repository root") {
		t.Fatalf("outside repository error = %v", err)
	}
	insideAgent := createTypeScriptSkill(t, filepath.Join(repo, ".cursor"), "inside-agent")
	if _, err := registerSkill(insideAgent, repo, []string{"cursor"}); err == nil || !strings.Contains(err.Error(), "must not live inside agent discovery") {
		t.Fatalf("agent directory error = %v", err)
	}
}

func TestRegisterRejectsSymlinkEscapes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unprivileged Windows test environments cannot reliably create symlinks")
	}
	repo := t.TempDir()
	writeTestFile(t, filepath.Join(repo, ".git"), "gitdir: fixture\n")
	outside := t.TempDir()
	skill := createTypeScriptSkill(t, outside, "review")
	if err := os.Symlink(skill, filepath.Join(repo, "review-link")); err != nil {
		t.Fatal(err)
	}
	if _, err := registerSkill(filepath.Join(repo, "review-link"), repo, []string{"codex"}); err == nil || !strings.Contains(err.Error(), "inside repository root") {
		t.Fatalf("source symlink escape error = %v", err)
	}

	canonical := createTypeScriptSkill(t, repo, "safe-review")
	if err := os.Symlink(outside, filepath.Join(repo, ".agents")); err != nil {
		t.Fatal(err)
	}
	if _, err := registerSkill(canonical, repo, []string{"codex"}); err == nil || !strings.Contains(err.Error(), "resolves outside repository") {
		t.Fatalf("destination symlink escape error = %v", err)
	}
}

func TestAutoDetectionUsesProjectAgentDirectory(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".cursor"), 0o755); err != nil {
		t.Fatal(err)
	}
	registry, err := loadAgentRegistry()
	if err != nil {
		t.Fatal(err)
	}
	selected, err := selectAgents(registry, []string{"auto"}, repo)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, agent := range selected {
		found = found || agent.ID == "cursor"
	}
	if !found {
		t.Fatal("project-local .cursor directory was not auto-detected")
	}
}

func TestSkillMetadataValidation(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bad-name")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, "SKILL.md"), "---\nname: other\ndescription: TODO\n---\n")
	if _, err := readSkillMetadata(dir); err == nil {
		t.Fatal("invalid metadata was accepted")
	}
}

func TestDoctorDetectsCurrentAndStaleAdapters(t *testing.T) {
	repo := t.TempDir()
	writeTestFile(t, filepath.Join(repo, ".git"), "gitdir: fixture\n")
	skill := createTypeScriptSkill(t, repo, "review")
	if _, err := registerSkill(skill, repo, []string{"codex"}); err != nil {
		t.Fatal(err)
	}
	if err := cmdDoctor([]string{skill, "--root", repo, "--agent", "codex"}); err != nil {
		t.Fatalf("doctor current adapter: %v", err)
	}
	writeTestFile(t, filepath.Join(skill, "main.ts"), "export const changed = true\n")
	if err := cmdDoctor([]string{skill, "--root", repo, "--agent", "codex"}); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("stale adapter error = %v", err)
	}
}

func TestDoctorWithoutAgentChecksWorkflowOnly(t *testing.T) {
	repo := t.TempDir()
	writeTestFile(t, filepath.Join(repo, ".git"), "gitdir: fixture\n")
	skill := createTypeScriptSkill(t, repo, "review")
	if err := cmdDoctor([]string{skill, "--root", repo}); err != nil {
		t.Fatalf("workflow-only doctor required an adapter: %v", err)
	}
}

func TestDoctorWorkflowOnlyDoesNotRequireRepository(t *testing.T) {
	root := t.TempDir()
	skill := createTypeScriptSkill(t, root, "review")
	if err := cmdDoctor([]string{skill}); err != nil {
		t.Fatalf("workflow-only doctor required a repository: %v", err)
	}
}

func TestRegisterAllPreflightsAndWritesEveryWorkflow(t *testing.T) {
	repo := t.TempDir()
	writeTestFile(t, filepath.Join(repo, ".git"), "gitdir: fixture\n")
	writeTestFile(t, filepath.Join(repo, "package.json"), `{"dependencies":{"@operatorstack/yield":"0.1.19"}}`)
	for _, name := range []string{"review", "release"} {
		skill := filepath.Join(repo, "skills", name)
		writeTestFile(t, filepath.Join(skill, "SKILL.md"), "---\nname: "+name+"\ndescription: Run "+name+" when the matching project workflow is requested.\n---\n")
		writeTestFile(t, filepath.Join(skill, "skill.json"), `{"version":1,"language":"typescript","run":["node","main.ts"]}`)
		writeTestFile(t, filepath.Join(skill, "main.ts"), "export {}\n")
	}
	if err := cmdRegisterAll([]string{filepath.Join(repo, "skills"), "--root", repo, "--agent", "codex", "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".agents", "skills", "review", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("dry-run wrote an adapter")
	}
	if err := cmdRegisterAll([]string{filepath.Join(repo, "skills"), "--root", repo, "--agent", "codex"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"review", "release"} {
		if _, err := os.Stat(filepath.Join(repo, ".agents", "skills", name, "SKILL.md")); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRegisterAllPruneRemovesOnlyOwnedAdapterFile(t *testing.T) {
	repo := t.TempDir()
	writeTestFile(t, filepath.Join(repo, ".git"), "gitdir: fixture\n")
	writeTestFile(t, filepath.Join(repo, "package.json"), `{"dependencies":{"@operatorstack/yield":"0.1.19"}}`)
	for _, name := range []string{"review", "release"} {
		skill := filepath.Join(repo, "skills", name)
		writeTestFile(t, filepath.Join(skill, "SKILL.md"), "---\nname: "+name+"\ndescription: Run "+name+" when the matching project workflow is requested.\n---\n")
		writeTestFile(t, filepath.Join(skill, "skill.json"), `{"version":1,"language":"typescript","run":["node","main.ts"]}`)
		writeTestFile(t, filepath.Join(skill, "main.ts"), "export {}\n")
	}
	if err := cmdRegisterAll([]string{filepath.Join(repo, "skills"), "--root", repo, "--agent", "codex"}); err != nil {
		t.Fatal(err)
	}
	releaseAdapter := filepath.Join(repo, ".agents", "skills", "release")
	writeTestFile(t, filepath.Join(releaseAdapter, "notes.txt"), "keep me\n")
	if err := os.RemoveAll(filepath.Join(repo, "skills", "release")); err != nil {
		t.Fatal(err)
	}
	if err := cmdRegisterAll([]string{filepath.Join(repo, "skills"), "--root", repo, "--agent", "codex", "--prune"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(releaseAdapter, "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("obsolete generated adapter remains: %v", err)
	}
	if got := readTestFile(t, filepath.Join(releaseAdapter, "notes.txt")); got != "keep me\n" {
		t.Fatalf("prune changed sibling file: %q", got)
	}
}

func TestRegisterAllRefusesCrossDirectoryNameCollisionBeforeWrites(t *testing.T) {
	repo := t.TempDir()
	writeTestFile(t, filepath.Join(repo, ".git"), "gitdir: fixture\n")
	writeTestFile(t, filepath.Join(repo, "package.json"), `{"dependencies":{"@operatorstack/yield":"0.1.19"}}`)
	makeSkill := func(parent string) string {
		skill := filepath.Join(repo, parent, "review")
		writeTestFile(t, filepath.Join(skill, "SKILL.md"), "---\nname: review\ndescription: Review the project when a user asks for a check.\n---\n")
		writeTestFile(t, filepath.Join(skill, "skill.json"), `{"version":1,"language":"typescript","run":["node","main.ts"]}`)
		writeTestFile(t, filepath.Join(skill, "main.ts"), "export {}\n")
		return skill
	}
	first := makeSkill("typescript-skills")
	second := makeSkill("python-skills")
	if _, err := registerSkill(first, repo, []string{"codex"}); err != nil {
		t.Fatal(err)
	}
	err := cmdRegisterAll([]string{filepath.Dir(second), "--root", repo, "--agent", "codex"})
	if err == nil || !strings.Contains(err.Error(), "name collision") || !strings.Contains(err.Error(), "typescript-skills/review") || !strings.Contains(err.Error(), "python-skills/review") {
		t.Fatalf("cross-directory collision = %v", err)
	}
}

func TestShellQuoteEscapesSingleQuote(t *testing.T) {
	if got := shellQuote("skills/team's-review"); got != `'skills/team'"'"'s-review'` {
		t.Fatalf("shellQuote = %q", got)
	}
}

func TestPythonLauncherUsesRepositoryVirtualEnvironment(t *testing.T) {
	repo := t.TempDir()
	python := filepath.Join(repo, ".venv", "bin", "python")
	t.Setenv("YIELD_PYTHON", python)
	got, err := launcherFor("python", filepath.Join(repo, "skills", "review"), repo)
	if err != nil {
		t.Fatal(err)
	}
	want := shellQuote(filepath.ToSlash(filepath.Join(".venv", "bin", "python"))) + " -m yieldskill"
	if got != want {
		t.Fatalf("python launcher = %q, want %q", got, want)
	}
}

func createTypeScriptSkill(t *testing.T, repo, name string) string {
	t.Helper()
	writeTestFile(t, filepath.Join(repo, "package.json"), `{"dependencies":{"@operatorstack/yield":"0.1.17"}}`)
	skill := filepath.Join(repo, "skills", name)
	writeTestFile(t, filepath.Join(skill, "SKILL.md"), "---\nname: "+name+"\ndescription: Review the branch when the user wants code checked before shipping.\n---\n")
	writeTestFile(t, filepath.Join(skill, "skill.json"), `{"version":1,"language":"typescript","run":["node","main.ts"]}`)
	writeTestFile(t, filepath.Join(skill, "main.ts"), "export {}\n")
	return skill
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
