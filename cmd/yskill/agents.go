package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/operatorstack/yield/internal/protocol"
)

//go:embed registry/agents.json
var agentRegistryJSON []byte

const generatedAdapterPrefix = "<!-- generated-by: yskill; source: "

type agentRegistry struct {
	Source struct {
		Repository string `json:"repository"`
		Commit     string `json:"commit"`
		License    string `json:"license"`
		Path       string `json:"path"`
	} `json:"source"`
	Agents []agentConfig `json:"agents"`
}

type agentConfig struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	ProjectDir  string `json:"project_dir"`
	Tier        string `json:"tier"`
}

type agentListFlag []string

func (v *agentListFlag) String() string { return strings.Join(*v, ",") }
func (v *agentListFlag) Set(value string) error {
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			*v = append(*v, item)
		}
	}
	return nil
}

func loadAgentRegistry() (agentRegistry, error) {
	var registry agentRegistry
	if err := json.Unmarshal(agentRegistryJSON, &registry); err != nil {
		return registry, fmt.Errorf("embedded agent registry does not decode: %w", err)
	}
	if registry.Source.Repository == "" || len(registry.Source.Commit) != 40 || registry.Source.License == "" || registry.Source.Path == "" {
		return registry, fmt.Errorf("embedded agent registry source metadata is incomplete")
	}
	seen := map[string]bool{}
	for _, agent := range registry.Agents {
		if agent.ID == "" || seen[agent.ID] {
			return registry, fmt.Errorf("embedded agent registry has missing or duplicate id %q", agent.ID)
		}
		seen[agent.ID] = true
		if agent.DisplayName == "" || !safeProjectSkillDir(agent.ProjectDir) {
			return registry, fmt.Errorf("embedded agent registry entry %q is invalid", agent.ID)
		}
		if agent.Tier != "verified" && agent.Tier != "registry" {
			return registry, fmt.Errorf("embedded agent registry entry %q has invalid tier", agent.ID)
		}
	}
	return registry, nil
}

func safeProjectSkillDir(value string) bool {
	if value == "" || filepath.IsAbs(value) {
		return false
	}
	clean := filepath.Clean(filepath.FromSlash(value))
	return clean != "." && clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

func cmdAgents(args []string) error {
	fs := flag.NewFlagSet("agents", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("agents takes no positional arguments")
	}
	registry, err := loadAgentRegistry()
	if err != nil {
		return err
	}
	root, _ := os.Getwd()
	for _, agent := range registry.Agents {
		detection := "explicit"
		if agent.Tier == "verified" {
			if agentDetected(agent, root) {
				detection = "detected"
			} else {
				detection = "not detected"
			}
		}
		fmt.Printf("%-22s %-24s %-10s %s\n", agent.ID, agent.DisplayName, agent.Tier, detection+" · "+agent.ProjectDir)
	}
	return nil
}

func cmdRegister(args []string) error {
	fs := flag.NewFlagSet("register", flag.ContinueOnError)
	var agents agentListFlag
	fs.Var(&agents, "agent", "agent id, comma-separated ids, or auto")
	root := fs.String("root", "", "repository root (detected from .git by default)")
	if err := parseOnePositional(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("register takes exactly one skill directory")
	}
	result, err := registerSkill(fs.Arg(0), *root, agents)
	if err != nil {
		return err
	}
	for _, item := range result {
		fmt.Printf("registered: %-22s %s\n", item.AgentID, item.Path)
		fmt.Printf("reload:     %-22s %s\n", item.AgentID, reloadHint(item.AgentID))
	}
	return nil
}

func reloadHint(agentIDs string) string {
	if strings.Contains(agentIDs, ",") {
		return "start a new session in each selected agent"
	}
	switch agentIDs {
	case "cursor":
		return "reload the Cursor window or start a new chat"
	case "codex", "claude-code":
		return "start a new agent session"
	default:
		return "reload the agent or start a new session"
	}
}

type registration struct {
	AgentID string
	Path    string
}

func registerSkill(skillArg, rootArg string, requested []string) ([]registration, error) {
	skillDir, repoRoot, sourceRel, metadata, manifest, _, selected, err := registrationInputs(skillArg, rootArg, requested)
	if err != nil {
		return nil, err
	}
	digest, err := protocol.DigestSkillDir(skillDir)
	if err != nil {
		return nil, fmt.Errorf("digest canonical workflow: %w", err)
	}
	launcher, err := launcherFor(manifest.Language, skillDir, repoRoot)
	if err != nil {
		return nil, err
	}
	content := renderAdapter(metadata, sourceRel, digest, launcher)
	byDestination := map[string][]string{}
	for _, agent := range selected {
		destination := filepath.Join(repoRoot, filepath.FromSlash(agent.ProjectDir), metadata.Name, "SKILL.md")
		byDestination[destination] = append(byDestination[destination], agent.ID)
	}
	paths := make([]string, 0, len(byDestination))
	for path := range byDestination {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var registrations []registration
	for _, path := range paths {
		if err := ensureContainedWrite(repoRoot, path); err != nil {
			return nil, err
		}
		if err := writeGeneratedAdapter(path, sourceRel, content); err != nil {
			return nil, err
		}
		rel, _ := filepath.Rel(repoRoot, path)
		ids := byDestination[path]
		sort.Strings(ids)
		registrations = append(registrations, registration{AgentID: strings.Join(ids, ","), Path: filepath.ToSlash(rel)})
	}
	return registrations, nil
}

func registrationInputs(skillArg, rootArg string, requested []string) (string, string, string, skillMetadata, skillManifest, agentRegistry, []agentConfig, error) {
	skillDir, err := filepath.Abs(skillArg)
	if err != nil {
		return "", "", "", skillMetadata{}, skillManifest{}, agentRegistry{}, nil, err
	}
	repoRoot, err := findRepoRoot(skillDir, rootArg)
	if err != nil {
		return "", "", "", skillMetadata{}, skillManifest{}, agentRegistry{}, nil, err
	}
	repoRoot, err = filepath.EvalSymlinks(repoRoot)
	if err != nil {
		return "", "", "", skillMetadata{}, skillManifest{}, agentRegistry{}, nil, fmt.Errorf("resolve repository root: %w", err)
	}
	skillDir, err = filepath.EvalSymlinks(skillDir)
	if err != nil {
		return "", "", "", skillMetadata{}, skillManifest{}, agentRegistry{}, nil, fmt.Errorf("resolve skill directory: %w", err)
	}
	sourceRel, err := filepath.Rel(repoRoot, skillDir)
	if err != nil || sourceRel == ".." || strings.HasPrefix(sourceRel, ".."+string(filepath.Separator)) {
		return "", "", "", skillMetadata{}, skillManifest{}, agentRegistry{}, nil, fmt.Errorf("skill directory must be inside repository root %s", repoRoot)
	}
	if strings.ContainsAny(sourceRel, ";\r\n<>") {
		return "", "", "", skillMetadata{}, skillManifest{}, agentRegistry{}, nil, fmt.Errorf("skill path contains characters that cannot be represented safely in an adapter marker")
	}
	metadata, err := readSkillMetadata(skillDir)
	if err != nil {
		return "", "", "", skillMetadata{}, skillManifest{}, agentRegistry{}, nil, err
	}
	manifest, err := readSkillManifest(skillDir)
	if err != nil {
		return "", "", "", skillMetadata{}, skillManifest{}, agentRegistry{}, nil, err
	}
	registry, err := loadAgentRegistry()
	if err != nil {
		return "", "", "", skillMetadata{}, skillManifest{}, registry, nil, err
	}
	selected, err := selectAgents(registry, requested, repoRoot)
	if err != nil {
		return "", "", "", skillMetadata{}, skillManifest{}, registry, nil, err
	}
	for _, agent := range selected {
		canonicalRoot := filepath.Join(repoRoot, filepath.FromSlash(agent.ProjectDir))
		if within(canonicalRoot, skillDir) {
			return "", "", "", skillMetadata{}, skillManifest{}, registry, nil, fmt.Errorf("canonical workflow must not live inside agent discovery directory %s", agent.ProjectDir)
		}
	}
	return skillDir, repoRoot, filepath.ToSlash(sourceRel), metadata, manifest, registry, selected, nil
}

func selectAgents(registry agentRegistry, requested []string, repoRoot string) ([]agentConfig, error) {
	if len(requested) == 0 {
		requested = []string{"auto"}
	}
	lookup := map[string]agentConfig{}
	for _, agent := range registry.Agents {
		lookup[agent.ID] = agent
	}
	selected := map[string]agentConfig{}
	for _, id := range requested {
		if id == "auto" {
			for _, agent := range registry.Agents {
				if agent.Tier == "verified" && agentDetected(agent, repoRoot) {
					selected[agent.ID] = agent
				}
			}
			continue
		}
		agent, ok := lookup[id]
		if !ok {
			return nil, fmt.Errorf("unsupported agent %q; run 'yskill agents' to list ids", id)
		}
		selected[id] = agent
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no verified coding agents detected; pass --agent <id> explicitly")
	}
	result := make([]agentConfig, 0, len(selected))
	for _, agent := range selected {
		result = append(result, agent)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func agentDetected(agent agentConfig, repoRoot string) bool {
	home, _ := os.UserHomeDir()
	exists := func(path string) bool { _, err := os.Stat(path); return err == nil }
	projectRoot := strings.SplitN(filepath.FromSlash(agent.ProjectDir), string(filepath.Separator), 2)[0]
	if repoRoot != "" && exists(filepath.Join(repoRoot, projectRoot)) {
		return true
	}
	switch agent.ID {
	case "cursor":
		return exists(filepath.Join(home, ".cursor")) || commandExists("cursor")
	case "codex":
		root := strings.TrimSpace(os.Getenv("CODEX_HOME"))
		if root == "" {
			root = filepath.Join(home, ".codex")
		}
		return exists(root) || exists("/etc/codex") || commandExists("codex")
	case "claude-code":
		root := strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR"))
		if root == "" {
			root = filepath.Join(home, ".claude")
		}
		return exists(root) || commandExists("claude")
	default:
		return false
	}
}

// ensureContainedWrite rejects an existing symlink in an adapter path when it
// would redirect the generated file outside the repository.
func ensureContainedWrite(repoRoot, destination string) error {
	if !within(repoRoot, destination) {
		return fmt.Errorf("adapter path escapes repository: %s", destination)
	}
	existing := destination
	for {
		if _, err := os.Lstat(existing); err == nil {
			break
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return fmt.Errorf("cannot resolve adapter path %s", destination)
		}
		existing = parent
	}
	resolved, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return err
	}
	if !within(repoRoot, resolved) {
		return fmt.Errorf("adapter path resolves outside repository: %s", destination)
	}
	return nil
}

func commandExists(name string) bool { _, err := exec.LookPath(name); return err == nil }

func findRepoRoot(skillDir, explicit string) (string, error) {
	if explicit != "" {
		root, err := filepath.Abs(explicit)
		if err != nil {
			return "", err
		}
		if info, err := os.Stat(root); err != nil || !info.IsDir() {
			return "", fmt.Errorf("repository root %s is not a directory", root)
		}
		return filepath.Clean(root), nil
	}
	for current := filepath.Clean(skillDir); ; current = filepath.Dir(current) {
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}
	return "", fmt.Errorf("cannot find repository root from %s; pass --root", skillDir)
}

func within(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func launcherFor(language, skillDir, repoRoot string) (string, error) {
	switch language {
	case "typescript":
		packageRoot, err := findTypeScriptPackageRoot(skillDir, repoRoot)
		if err != nil {
			return "", err
		}
		rel, _ := filepath.Rel(repoRoot, packageRoot)
		if rel == "." {
			return "npm exec -- yskill", nil
		}
		return "npm --prefix " + shellQuote(filepath.ToSlash(rel)) + " exec -- yskill", nil
	case "python":
		python := strings.TrimSpace(os.Getenv("YIELD_PYTHON"))
		if filepath.IsAbs(python) && within(repoRoot, python) {
			rel, err := filepath.Rel(repoRoot, python)
			if err == nil {
				return shellQuote(filepath.ToSlash(rel)) + " -m yieldskill", nil
			}
		}
		return "python -m yieldskill", nil
	case "go", "rust":
		return "yskill", nil
	default:
		return "", fmt.Errorf("unsupported workflow language %q", language)
	}
}

func findTypeScriptPackageRoot(skillDir, repoRoot string) (string, error) {
	var declared string
	for current := filepath.Clean(skillDir); within(repoRoot, current); current = filepath.Dir(current) {
		path := filepath.Join(current, "package.json")
		if hasTypeScriptPackageDependency(path) {
			if declared == "" {
				declared = current
			}
			if _, err := os.Stat(filepath.Join(current, "node_modules", "@operatorstack", "yield")); err == nil {
				return current, nil
			}
		}
		if current == repoRoot {
			break
		}
	}
	if declared != "" {
		return declared, nil
	}
	return "", fmt.Errorf("cannot find a package.json containing @operatorstack/yield between %s and %s", skillDir, repoRoot)
}

func hasTypeScriptPackageDependency(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var manifest struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if json.Unmarshal(b, &manifest) != nil {
		return false
	}
	return manifest.Dependencies["@operatorstack/yield"] != "" || manifest.DevDependencies["@operatorstack/yield"] != ""
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func renderAdapter(metadata skillMetadata, sourceRel, digest, launcher string) string {
	path := shellQuote(sourceRel)
	return fmt.Sprintf(`---
name: %s
description: %s
---

%s%s; digest: %s; version: %s -->

This adapter exposes the canonical Yield workflow at %s.
Read its SKILL.md, then run from the repository root:

    %s run %s

Follow each returned operation exactly. Resume after each response:

    %s resume <run-id> --response response.json --skill %s

Do not skip an operation or invent its response.
`, metadata.Name, yamlString(metadata.Description), generatedAdapterPrefix, sourceRel, digest, runtimeVersion(), "`"+sourceRel+"`", launcher, path, launcher, path)
}

func writeGeneratedAdapter(path, sourceRel, content string) error {
	if existing, err := os.ReadFile(path); err == nil {
		marker := generatedAdapterPrefix + sourceRel + ";"
		if !strings.Contains(string(existing), marker) {
			return fmt.Errorf("refusing to overwrite user-owned or differently sourced adapter %s", path)
		}
		if string(existing) == content {
			return nil
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".yskill-adapter-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func cmdDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	var agents agentListFlag
	fs.Var(&agents, "agent", "agent id, comma-separated ids, or auto")
	root := fs.String("root", "", "repository root (detected from .git by default)")
	runTest := fs.Bool("test", false, "run the workflow fixture after static checks")
	if err := parseOnePositional(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("doctor takes exactly one skill directory")
	}
	skillDir, repoRoot, sourceRel, _, manifest, _, selected, err := registrationInputs(fs.Arg(0), *root, agents)
	if err != nil {
		return err
	}
	if _, err := launcherFor(manifest.Language, skillDir, repoRoot); err != nil {
		return err
	}
	digest, err := protocol.DigestSkillDir(skillDir)
	if err != nil {
		return err
	}
	for _, agent := range selected {
		path := filepath.Join(repoRoot, filepath.FromSlash(agent.ProjectDir), filepath.Base(skillDir), "SKILL.md")
		if err := ensureContainedWrite(repoRoot, path); err != nil {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("%s adapter missing at %s; run yskill register", agent.ID, path)
		}
		text := string(b)
		if !strings.Contains(text, generatedAdapterPrefix+sourceRel+";") || !strings.Contains(text, "digest: "+digest+";") {
			return fmt.Errorf("%s adapter is stale or points elsewhere; run yskill register", agent.ID)
		}
		fmt.Printf("ok: %-22s %s\n", agent.ID, filepath.ToSlash(path))
	}
	if *runTest {
		if err := cmdTest([]string{skillDir}); err != nil {
			return err
		}
	}
	fmt.Printf("doctor: %s is ready for %d agent(s)\n", filepath.Base(skillDir), len(selected))
	return nil
}
