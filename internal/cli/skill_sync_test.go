package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// TestEmbeddedSkillMatchesSource guards against the embedded Skill drifting from
// the canonical skill/SKILL.md that ships in the repo and docs. If you edit one,
// edit both (or copy skill/SKILL.md over internal/cli/embed/agentguard_skill.md).
func TestEmbeddedSkillMatchesSource(t *testing.T) {
	// Walk up to the repo root from this package directory.
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "skill", "SKILL.md")
	want, err := os.ReadFile(src)
	if err != nil {
		t.Skipf("canonical skill not found (%v) — skipping drift check", err)
	}
	if string(want) != embeddedSkill {
		t.Fatalf("embedded skill differs from %s — re-copy it:\n"+
			"  cp skill/SKILL.md internal/cli/embed/agentguard_skill.md", src)
	}
}
