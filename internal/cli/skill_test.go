package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallSkill_WritesWhenClaudePresent(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	dest, err := installSkill(home, &out)
	if err != nil {
		t.Fatalf("installSkill: %v", err)
	}
	want := filepath.Join(home, ".claude", "skills", "agentguard", "SKILL.md")
	if dest != want {
		t.Fatalf("dest = %q, want %q", dest, want)
	}
	body, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read skill: %v", err)
	}
	if !strings.Contains(string(body), "name: agentguard") {
		t.Fatal("written skill is missing its frontmatter")
	}
	if !strings.Contains(out.String(), "installed Claude Code Skill") {
		t.Fatal("expected a confirmation line on the writer")
	}
}

func TestInstallSkill_NoopWhenClaudeAbsent(t *testing.T) {
	home := t.TempDir() // no ~/.claude
	dest, err := installSkill(home, nil)
	if err != nil {
		t.Fatalf("installSkill: %v", err)
	}
	if dest != "" {
		t.Fatalf("expected no-op (empty dest), got %q", dest)
	}
}

func TestInstallSkill_OverwritesStale(t *testing.T) {
	home := t.TempDir()
	skillDir := filepath.Join(home, ".claude", "skills", "agentguard")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(stale, []byte("old version"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := installSkill(home, nil); err != nil {
		t.Fatalf("installSkill: %v", err)
	}
	body, _ := os.ReadFile(stale)
	if string(body) == "old version" {
		t.Fatal("stale skill was not overwritten")
	}
}
