# AgentGuard — User-Side System Design

> **Audience:** the human who just heard "you should secure your AI agent" and has 90 seconds of patience.
> **Companion to:** `system-design.md` (the engine) and `requirements.md` (the build). This document is the
> *experience* — how a real person goes from "never heard of it" to "protected and delighted" with the least
> possible manual work.
>
> **Design thesis:** the user should never have to read our docs to be safe. They install one binary, then tell
> the AI agent they *already use every day* — "I installed AgentGuard, set it up" — and the agent does the rest,
> because AgentGuard ships the knowledge the agent needs (a Skill) and the landing page hands the user a ready
> prompt. The tool that secures agents is itself set up *by* an agent. That is the whole trick.

---

## 1. Who the user is

Three personas, one funnel:

| Persona | What they have | What they fear | What "done" feels like |
|---------|----------------|----------------|------------------------|
| **Solo builder** (Nautilus) | Claude Code / Cursor open right now | "Is my agent leaking my keys? I can't tell." | One dashboard tab showing green ✓ on every call |
| **Security-curious dev** | A team, a threat model, a review board | "I can't add infra I have to operate." | Byte-identical uninstall + local-only audit log |
| **The skeptic** | A loud opinion on HN | "Another security-theater wrapper." | `agentguard scan` catches a *real* injection, live |

All three share one truth: **they will not read a manual before trying it.** So the manual must be *executed for them* by their agent, not read by them.

---

## 2. The core insight: let the agent set up its own guard

Every one of these users already talks to an AI coding agent all day. That agent can read files, run shell commands, and edit JSON/TOML configs — which is *exactly* what AgentGuard setup requires. So instead of making the human do it, we make the human's agent do it:

```
┌─────────────┐   1. one install command    ┌──────────────────────┐
│   Human     │ ──────────────────────────► │  agentguard binary   │
│             │                              │  on disk + Skill     │
│             │   2. "set up agentguard"     │  shipped to ~/.claude│
│             │ ──────────────────────────► │                      │
│  AI agent   │ ◄────── reads SKILL.md ───── │                      │
│ (Claude/    │   3. runs init, dashboard,   │                      │
│  Cursor/…)  │      verifies, explains      │                      │
└─────────────┘                              └──────────────────────┘
```

The human types two things, total:
1. The install one-liner (copied from the landing page, OS auto-detected).
2. A single sentence to their agent: *"I installed AgentGuard — set it up and show me it's working."*

Everything else — detecting agents, backing up configs, rewriting MCP entries, starting the dashboard, firing a test call, explaining the verdicts — is done by the agent, guided by the Skill we ship.

---

## 3. The four-touch journey (and what each touch costs the user)

| # | Touch | User effort | Who does the work |
|---|-------|-------------|-------------------|
| 1 | **Install** | paste 1 command | `install.sh` / `install.ps1` (auto OS-detect, SHA-256 verify, PATH, ships the Skill) |
| 2 | **Hand off to the agent** | paste 1 prompt | landing page *Prompt Generator* writes it; user picks their agent, clicks Copy |
| 3 | **Agent sets up** | watch | the agent, reading `skill/SKILL.md`: `init` → `dashboard` → test call → explain |
| 4 | **Confirm & live** | glance at dashboard | AgentGuard (the dashboard renders the proof) |

Two pastes. Zero config files opened by hand. Zero docs read. That is the bar.

---

## 4. Touch 1 — Install (already built, now hardened)

The install command is OS-aware on the landing page (`#installWidget` auto-detects and pre-selects the right tab):

- **macOS / Linux / Git Bash:** `curl -fsSL https://agentguard.space/install | sh`
- **Windows PowerShell:** `iwr -useb https://agentguard.space/install.ps1 | iex`

The installer must be **idempotent and re-runnable** — users *will* run it twice. (A prior version failed the second run because a leftover daemon process locked `agentguard.exe` on Windows; the installer now stops any running AgentGuard process and swaps the binary with copy-with-retry + rename-aside fallback. See `install/install.ps1`.)

**New responsibility for the installer:** after dropping the binary, it copies the shipped Skill into the user's Claude Code skills directory if that directory exists, so Touch 2 works with zero extra steps:

```
~/.claude/skills/agentguard/SKILL.md      (if ~/.claude exists)
```

For agents without a skill mechanism (Cursor, Codex, Gemini, Windsurf), Touch 2's pasted prompt is fully self-contained — it carries the instructions inline, so no pre-seeding is needed.

---

## 5. Touch 2 — The Prompt Generator (landing page)

A new landing-page section, **"Let your AI agent set it up"**, sits immediately after Install. The user:

1. Sees their OS already detected (reused from the install widget).
2. Picks their agent from a row of chips: **Claude Code · Cursor · Codex CLI · Gemini CLI · Windsurf**.
3. The page renders a tailored, copy-ready prompt in a code box. One **Copy prompt** button.
4. The user pastes it into that agent's chat. Done.

### Why per-agent prompts differ

Each agent looks for its config in a different place and has a different "how do I run a shell command" idiom. The generated prompt names the exact config path and tells the agent precisely what to verify, so the agent never has to guess:

| Agent | Config the prompt points at |
|-------|----------------------------|
| Claude Code | `~/.claude.json` (or `claude_desktop_config.json`) + the shipped Skill |
| Cursor | `~/.cursor/mcp.json` |
| Codex CLI | `~/.codex/config.toml` |
| Gemini CLI | `~/.gemini/settings.json` |
| Windsurf | `~/.codeium/windsurf/mcp_config.json` |

### The generated prompt (shape)

Every variant follows the same skeleton (Claude Code variant shown; others swap the config path and the "you already have a Skill for this" line):

```
I just installed AgentGuard (https://agentguard.space) — a local security sidecar
that inspects every MCP tool call my agent makes.

Please set it up for me:
1. Confirm the binary works:  agentguard --version  and  agentguard doctor
2. Run  agentguard init --non-interactive  to detect this agent and route its
   MCP servers through AgentGuard. It backs up my config first; don't edit any
   config by hand.
3. Start the dashboard:  agentguard dashboard --no-browser  and give me the URL
   (http://127.0.0.1:7878).
4. Trigger one real tool call so I can see a row appear in the dashboard, then
   tell me what verdict it got and what that means.
5. If anything is wrong, run  agentguard doctor  and explain the ✗ lines in plain
   language. Never disable AgentGuard or delete data without asking me first.

You have an AgentGuard Skill installed at ~/.claude/skills/agentguard/SKILL.md —
read it first and follow it.
```

The Copy button uses the existing clipboard helper in `site/script.js`; the generator is pure client-side string assembly (no network, consistent with the "0 bytes leave your machine" promise).

---

## 6. Touch 3 — The Skill that teaches the agent (`skill/SKILL.md`)

This is the meta-touch the requirements call out (§8). AgentGuard ships a Claude Code Skill so the agent knows *how* to set AgentGuard up and operate it safely — without the human pasting a wall of instructions.

The Skill encodes:

- **Setup playbook** — the exact command order (`doctor` → `init` → `dashboard` → test call), idempotent, safe to re-run.
- **Operating playbook** — how to read the audit DB read-only, how to change a policy via `agentguard policy …` (never by hand-editing YAML), how to investigate a flagged call.
- **Safety rails** — confirm before anything destructive (`uninstall`, `policy disable`, `--purge`); never weaken protection silently.
- **Teaching mode** — when the user is a beginner, explain each verdict (`allow` / `flag` / `transform` / `block`) in one plain sentence.

Because the Skill is data, not code, the same file works whether the agent reads it from `~/.claude/skills/` (auto-loaded) or from the pasted prompt's inline pointer. This is what lets *one* design serve every agent.

---

## 7. Touch 4 — Confirm it's live (the payoff)

The dashboard at `http://127.0.0.1:7878` is the proof. The agent points the user there and triggers one tool call; the user watches a row appear, color-coded by verdict. That single moment — "I can *see* what my agent is doing" — is the whole emotional payoff and the thing people screenshot.

For terminal lovers, `agentguard tail` is the same proof in a TUI. For the skeptic, `agentguard scan <server>` fires the canned attack corpus and proves a real catch.

---

## 8. Failure modes and how the design absorbs them

| If… | The user does NOT… | Because… |
|------|--------------------|----------|
| Install run twice | …hit a locked-binary error | installer stops the daemon + copy-with-retry (fixed) |
| Agent isn't auto-detected | …debug config paths | the generated prompt names the exact path; `doctor` reports coverage |
| User uninstalls | …lose their original configs | byte-identical restore from `.agentguard.bak`; daemon is stopped first (fixed) |
| User is a total beginner | …read a manual | the agent explains verdicts in plain language (Skill "teaching mode") |
| Agent has no Skill mechanism | …miss instructions | the pasted prompt is self-contained |

---

## 9. What we deliberately do NOT do (user-side scope discipline)

- **No GUI installer / .msi / .pkg.** The one-liner + agent handoff is faster and auditable. (Matches `requirements.md` §12.)
- **No account, no login, no telemetry on the setup path.** The generator is client-side; the binary phones nobody.
- **No "AgentGuard assistant" chatbot.** The user already has an agent; we enroll *it* rather than shipping another one.
- **No forced dashboard auto-open in the agent flow.** The agent hands over the URL; the user opens it when ready (avoids surprise browser launches in headless/CI contexts).

---

## 10. Success criteria (user-side Definition of Done)

- [ ] A first-time user is protected with **two pastes** (install command + agent prompt) and **zero** hand-edited config files.
- [ ] Re-running the installer never errors on a machine that already has AgentGuard.
- [ ] Pasting the generated prompt into Claude Code results in a working `init` + a live dashboard URL, unattended.
- [ ] The shipped Skill makes the agent refuse to weaken protection without explicit confirmation.
- [ ] A beginner who reads nothing still understands their first verdict, because the agent explained it.

If those five hold, setup is not a chore the user tolerates — it's a 90-second moment they enjoy and tell people about.
