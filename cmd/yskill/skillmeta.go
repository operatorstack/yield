package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

var portableSkillName = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type skillManifest struct {
	Version      int      `json:"version"`
	YieldVersion string   `json:"yield_version"`
	Language     string   `json:"language"`
	Run          []string `json:"run"`
}

type skillMetadata struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func readSkillManifest(dir string) (skillManifest, error) {
	b, err := os.ReadFile(filepath.Join(dir, "skill.json"))
	if err != nil {
		return skillManifest{}, fmt.Errorf("read skill.json: %w", err)
	}
	var manifest skillManifest
	if err := json.Unmarshal(b, &manifest); err != nil {
		return skillManifest{}, fmt.Errorf("skill.json does not decode: %w", err)
	}
	if manifest.Version != 1 {
		return skillManifest{}, fmt.Errorf("skill.json version must be 1")
	}
	if !releaseVersionPattern.MatchString(manifest.YieldVersion) {
		return skillManifest{}, fmt.Errorf("skill.json version 1 requires an exact yield_version")
	}
	switch manifest.Language {
	case "typescript", "python", "go", "rust":
	default:
		return skillManifest{}, fmt.Errorf("skill.json language must be typescript, python, go, or rust")
	}
	if len(manifest.Run) == 0 {
		return skillManifest{}, fmt.Errorf("skill.json run must not be empty")
	}
	return manifest, nil
}

func readSkillMetadata(dir string) (skillMetadata, error) {
	b, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return skillMetadata{}, fmt.Errorf("read SKILL.md: %w", err)
	}
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return skillMetadata{}, fmt.Errorf("SKILL.md must start with YAML frontmatter")
	}
	rest := text[4:]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return skillMetadata{}, fmt.Errorf("SKILL.md frontmatter is not closed")
	}
	var metadata skillMetadata
	if err := yaml.Unmarshal([]byte(rest[:end]), &metadata); err != nil {
		return skillMetadata{}, fmt.Errorf("SKILL.md frontmatter does not decode: %w", err)
	}
	metadata.Name = strings.TrimSpace(metadata.Name)
	metadata.Description = strings.TrimSpace(metadata.Description)
	if !portableSkillName.MatchString(metadata.Name) || len(metadata.Name) > 64 {
		return skillMetadata{}, fmt.Errorf("skill name must match [a-z0-9]+(-[a-z0-9]+)* and be at most 64 characters")
	}
	if metadata.Name != filepath.Base(filepath.Clean(dir)) {
		return skillMetadata{}, fmt.Errorf("skill name %q must match directory %q", metadata.Name, filepath.Base(filepath.Clean(dir)))
	}
	if metadata.Description == "" || strings.Contains(strings.ToLower(metadata.Description), "todo") {
		return skillMetadata{}, fmt.Errorf("skill description must explain what the skill does and when to use it")
	}
	if len(metadata.Description) > 1024 {
		return skillMetadata{}, fmt.Errorf("skill description must be at most 1024 characters")
	}
	return metadata, nil
}

func yamlString(value string) string { return strconv.Quote(value) }
