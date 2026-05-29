package cli

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// embeddedSkill is the AgentGuard Claude Code Skill, shipped inside the binary
// so `init` can drop it into ~/.claude/skills/ with no extra download. This is
// what lets the user paste one prompt ("set up AgentGuard") and have the agent
// follow our playbook — see user-side-system-design.md §6.
//
//go:embed embed/agentguard_skill.md
var embeddedSkill string

// installSkill writes the embedded Skill into the Claude Code skills directory
// if (and only if) Claude Code is present (~/.claude exists). It is best-effort:
// other agents don't have a skill mechanism, and the pasted setup prompt carries
// the instructions inline for them, so a miss here is never fatal.
//
// Returns the path written, or "" if Claude Code wasn't found. Existing skills
// are overwritten so re-running init keeps the Skill current with the binary.
func installSkill(home string, out io.Writer) (string, error) {
	claudeDir := filepath.Join(home, ".claude")
	if info, err := os.Stat(claudeDir); err != nil || !info.IsDir() {
		return "", nil // Claude Code not installed — nothing to do.
	}

	skillDir := filepath.Join(claudeDir, "skills", "agentguard")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return "", fmt.Errorf("create skill dir: %w", err)
	}
	dest := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(dest, []byte(embeddedSkill), 0o644); err != nil {
		return "", fmt.Errorf("write skill: %w", err)
	}
	if out != nil {
		fmt.Fprintf(out, "  ✦ installed Claude Code Skill — %s\n", dest)
	}
	return dest, nil
}
