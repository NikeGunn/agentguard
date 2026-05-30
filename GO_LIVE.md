# AgentGuard — Launch Kit 🚀

**Copy-paste launch content for every platform.** Written in a calm, respectful,
student-builder voice — confident about the work, humble about the person. The
goal is for developers to respect the *project* and want to contribute, not to
sell a personality.

> Repo: https://github.com/nikegunn/agentguard
> Site: https://agentguard.space
> License: MIT · Single Go binary · Local-only · v1.2.2

---

## Tone guide (read this first — 30 seconds)

These rules are baked into every post below. If you edit anything, keep them:

- **Lead with the problem and the artifact, never with yourself.** "AI agents
  trust their tools blindly" is interesting. "I built a thing" is not.
- **Say "I'm a student" once, plainly, without apology.** It earns goodwill and
  sets expectations. Don't over-explain or fish for sympathy.
- **Use "I built" for the work, "we'd love" for the community.** The project is
  bigger than one person the moment you invite contributors.
- **Invite critique, don't defend.** "I'd genuinely like to know where this
  breaks" reads as a confident engineer. "This is production-ready" reads as
  someone who hasn't been bitten yet.
- **Never claim it's better than tools you haven't benchmarked.** Compare on
  facts (local-only, <5 ms p99, MIT) not on adjectives.
- **Credit the ecosystem.** MCP, the agent vendors, the libraries. Standing on
  shoulders is a good look.

---

## ⭐ Top 3 platforms (do these first, in this order)

If you only have one evening, post these three. They reach the exact people who
have this problem and who contribute to open source.

1. **Hacker News — Show HN** → highest-quality technical audience, best for a
   security tool. See [§1](#1-hacker-news-show-hn).
2. **Reddit — r/LocalLLaMA + r/selfhosted** → people running agents locally who
   immediately get the threat model. See [§2](#2-reddit).
3. **GitHub itself (Release + Discussions + good-first-issues)** → where the
   contributors you're recruiting actually live. See [§3](#3-github-release--contributor-call).

Everything after that (X/Twitter, LinkedIn, Dev.to, Lobste.rs, Discord) is in
[§4 onward](#4-x--twitter-thread) — fire them off over the following days, not
all at once.

---

# 1. Hacker News (Show HN)

**Best time to post:** Tuesday–Thursday, ~8–10am US Eastern (weekday US morning).
Post it yourself, then don't argue in the comments — answer questions, thank
critics, fix what's fair.

**Title** (HN dislikes hype; keep it factual):

```
Show HN: AgentGuard – a local security sidecar for AI agents (Go, MIT)
```

**Body:**

```
Hi HN,

I'm a student, and over the last few weeks I built AgentGuard: a single
Go binary that sits between an AI agent (Claude Code, Cursor, Codex,
Gemini CLI, Windsurf — anything that speaks MCP) and the tools it calls,
and inspects every JSON-RPC frame on the wire.

The problem I kept running into: agents trust their tools by default. A
poisoned or compromised MCP server can rewrite its own tool descriptions
between sessions (a "rug pull"), smuggle prompt injections inside ordinary
tool responses, or quietly forward credentials to a webhook. The IDE
approval prompt doesn't catch any of that — it only shows you the
human-readable intent, not the bytes.

AgentGuard inspects the bytes. Every frame runs through a small pipeline:
transport limits, JSON-RPC schema check, a tool-schema-drift detector
(the rug-pull alarm — it hashes the tool list and flags changes), YAML
rule packs, a regex secret redactor, and a heuristic prompt-injection
classifier. A per-session circuit breaker trips after repeated
block/error verdicts.

Setup is one command. `agentguard init` detects every agent on your
machine and rewrites its MCP config to route through the proxy. Backups
are byte-identical; `uninstall` restores them exactly. The agent doesn't
notice the proxy is there.

Everything is local. No account, no telemetry, no cloud — a dashboard
runs at 127.0.0.1:7878 with a live feed. Measured p99 on the cheap path
is ~500µs against a 5 ms budget that CI enforces.

A few honest caveats, since this is HN: the injection classifier is
heuristic, not an ONNX model (I deliberately skipped the 100+ MB
dependency for v1 — the precision was good enough without it, but I'd
believe a determined attacker could get past it). The rule packs only
cover the common secret formats so far. And I'm one student, so the
threat model has had exactly one set of eyes on it — which is the main
reason I'm posting.

I'd genuinely value your scrutiny on the design: heuristic vs. model,
SQLite vs. DuckDB, no-daemon vs. supervised, and especially anything in
the threat model I've gotten wrong.

Repo (MIT): https://github.com/nikegunn/agentguard
Site: https://agentguard.space

Thanks for reading.
```

**If it gets traction, top comment to pin / lead with:**

```
Happy to go deep on any design tradeoff. The two I most want pushback on:
(1) is a heuristic injection classifier defensible for a security tool, or
is that a footgun? (2) the rug-pull detector trusts first-seen tool
schemas — is TOFU the right default, or should it ship with a pinned
allowlist? Genuinely asking.
```

---

# 2. Reddit

Reddit punishes anything that smells like marketing. Lead with the threat, show
the demo, ask for feedback. Post to **r/LocalLLaMA** and **r/selfhosted** first;
**r/programming** and **r/golang** are secondary (and r/programming is harsh —
only post there once the README GIF is recorded).

### r/LocalLLaMA  /  r/selfhosted

**Title:**

```
I built a local, open-source security proxy for AI agents (catches tool poisoning + prompt injection on the wire) — would love feedback
```

**Body:**

```
I'm a student and I've been running coding agents locally (Claude Code,
Cursor, etc.) and got nervous about how much they trust their MCP tools.
A compromised tool server can change its own descriptions between runs,
hide prompt injections in normal-looking responses, or exfiltrate secrets
— and the approval prompt won't show you any of that.

So I built **AgentGuard**: a single Go binary that proxies the MCP
traffic and inspects every frame — schema drift / "rug pull" detection,
secret redaction, a heuristic prompt-injection classifier, rule packs,
and a per-session circuit breaker.

What might fit this sub specifically:
- 100% local. No account, no telemetry, no cloud. Your data never leaves
  the box.
- One binary, no daemon, no Docker required.
- `agentguard init` auto-detects your agents and wires itself in;
  `uninstall` restores the original config byte-for-byte.
- Local dashboard at 127.0.0.1:7878 with a live call feed.
- ~500µs p99 overhead, MIT licensed.

It's early and it's one student's threat model, so I'd really value people
poking holes in it. Repo + install one-liner:
https://github.com/nikegunn/agentguard

(Demo GIF in the README.) What would you want it to catch that it doesn't?
```

**Reddit etiquette:** reply to every comment for the first few hours, never get
defensive, and edit the post to add an **"Edit: thanks all, common questions
below"** FAQ once patterns emerge. Don't cross-post all subs in the same hour —
space them a day apart so it doesn't look like a spam campaign.

---

# 3. GitHub (Release + Contributor Call)

This is where the contributors you're recruiting actually are. Three pieces:

### 3a. GitHub Release notes (tag `v1.2.2` or the launch tag)

```
## AgentGuard v1.2.2 — first public release 🎉

AgentGuard is a local, open-source security sidecar for AI agents. It sits
between your agent (Claude Code, Cursor, Codex, Gemini CLI, Windsurf) and
the MCP tools it calls, and inspects every JSON-RPC frame for tool
poisoning, schema-drift "rug pulls", prompt injection, and credential
leaks — before they reach your model.

### Install
\`\`\`bash
curl -fsSL https://agentguard.space/install | sh
agentguard init        # detects + wires every installed agent
agentguard dashboard   # http://127.0.0.1:7878
\`\`\`

### Highlights
- Single Go binary, no daemon, no Docker, 100% local.
- Five-stage inspection pipeline + per-session circuit breaker.
- Byte-identical config backups; `uninstall` restores exactly.
- ~500µs p99 overhead (5 ms budget enforced in CI).
- Releases are cosign-signed via GitHub OIDC.

### Verify the download
The release notes include the exact `cosign verify-blob` command for
`checksums.txt`.

This is a student-built project released in public. Issues, PRs, and
especially threat-model critiques are all welcome — see CONTRIBUTING.md
and the "good first issue" label.
```

### 3b. Pinned GitHub Discussion — "Welcome + how to contribute"

Open **Discussions** on the repo (Settings → Features), create a "📣
Announcements" post and pin it:

```
# Welcome to AgentGuard 👋

Thanks for stopping by. AgentGuard is a local security proxy for AI agents,
built in the open. I'm a student maintaining this in my spare time, so this
is genuinely a community project — I can't and don't want to do it alone.

**Good places to start contributing (no permission needed, just open a PR):**

1. **Rule packs** — secret formats or dangerous tool patterns we don't catch
   yet. These are just YAML; one of the easiest, highest-value contributions.
2. **Agent detectors** — Aider, Cline, Continue, and others aren't supported
   yet. Each detector is one small, well-isolated file with a test.
3. **Prompt-injection corpora** — payloads our heuristic classifier misses.
   Adversarial examples make the whole project better.
4. **Docs + the demo** — clearer explanations, more GIFs, better onboarding.

Look for the **`good first issue`** and **`help wanted`** labels. Every PR
gets a real review and real credit. If something is unclear, that's a bug in
my docs — please tell me.

A request: if you find a way to get *past* AgentGuard, that's the most
valuable thing you can bring. Please report it (SECURITY.md for anything
sensitive) — breaking it is how it gets good.

— Nikhil
```

### 3c. Prep checklist before you call for contributors

Contributors bounce off a repo with nowhere obvious to start. Do these first:

- [ ] Add issue templates — `.github/ISSUE_TEMPLATE/` is currently empty. Add a
      bug report + feature request + "new rule pack" template.
- [ ] Create **5–10 `good first issue`s** with real, scoped tasks (one rule
      pack, one agent detector, one doc fix each). An empty issue tracker with
      a "contributors welcome!" sign convinces no one.
- [ ] Add the `good first issue` and `help wanted` labels (the `repo-pro` skill
      can set up the whole label taxonomy + repo metadata in one shot).
- [ ] Make sure CONTRIBUTING.md's clone URL points at `nikegunn/agentguard`
      (it currently has a `<you>` placeholder on the clone line).
- [ ] Record the README demo GIF — it's the single biggest conversion lever for
      every post in this file.

---

# 4. X / Twitter (thread)

Lead tweet carries the whole pitch; the rest is for people who clicked. Attach
the dashboard GIF to tweet 1.

```
1/ I'm a student, and I built AgentGuard: an open-source (MIT) security
sidecar for AI agents.

It sits between your agent and its tools and inspects every call for
prompt injection, tool poisoning, "rug pulls", and secret leaks.

Single Go binary. 100% local. 🧵
```

```
2/ The problem: agents trust their MCP tools by default.

A compromised tool server can rewrite its own descriptions, hide
injections in normal responses, or exfiltrate secrets — and your IDE's
approval prompt shows none of it.

AgentGuard inspects the actual bytes on the wire.
```

```
3/ Setup is one command:

curl -fsSL agentguard.space/install | sh
agentguard init

It auto-detects every agent on your machine and wires itself in. Backups
are byte-identical; uninstall restores them exactly. The agent never
notices.
```

```
4/ Under the hood: a 5-stage pipeline (transport → schema → schema-drift
detector → rule packs → secret redactor → injection classifier) and a
per-session circuit breaker.

~500µs p99 overhead, enforced in CI. Local dashboard with a live feed.
```

```
5/ It's early and it's one student's threat model, so I'd genuinely value
people breaking it.

Repo, docs, and a "good first issue" list for contributors:
github.com/nikegunn/agentguard

Rule packs and agent detectors are great first PRs 🙏
```

---

# 5. LinkedIn

LinkedIn rewards the human story. Slightly warmer, still humble, still
problem-first. Good for reaching mentors, recruiters, and other builders.

```
A few weeks ago I got nervous about something: the AI coding agents I use
every day trust their tools completely. A compromised tool server can hide
a prompt injection in a normal-looking response, silently change what it
claims to do, or leak credentials — and nothing in the IDE would show me.

So as a student project, I built AgentGuard — an open-source security
proxy that sits between an AI agent and its tools and inspects every call
before it reaches the model. It's a single Go binary, runs entirely
locally (no cloud, no telemetry), and adds well under a millisecond of
overhead.

I'm sharing it in public because I know one student's threat model has
blind spots, and the fastest way to find them is to let good engineers
poke at it. If you work with AI agents — or just enjoy security and Go —
I'd love your eyes on it, and contributors are genuinely welcome.

Repo (MIT): github.com/nikegunn/agentguard

Grateful for any feedback, and for everyone who's helped me learn enough
to build this.

#AI #Security #OpenSource #Golang
```

---

# 6. Dev.to / Hashnode (launch article)

A longer post does double duty as SEO and as your "Why we built AgentGuard"
blog. Suggested title and intro; expand the body from the README's "Why" and
"How it works" sections.

**Title:** `AgentGuard: a local security sidecar for AI agents (and why I built it as a student)`

**Intro:**

```
AI agents have a quiet trust problem: they believe their tools. This is a
walkthrough of AgentGuard — a small open-source Go proxy I built to inspect
what those tools actually send and receive — the threat model that motivated
it, the five-stage pipeline, and the honest limitations of doing security as
a one-person student project.
```

**Suggested sections:**
1. The trust problem (rug pulls, injection-in-responses, exfiltration)
2. What "inspect the wire" actually means (the pipeline, with a diagram)
3. The one-command setup and why byte-identical backups mattered to me
4. What I deliberately left out for v1 (ONNX model, DuckDB) and why
5. Where it breaks, and how you can help

End every article with the contributor call from §3b.

---

# 7. Lobste.rs

Invite-only and *very* allergic to marketing — only post if you have an account
with standing. Use the `security` and `go` tags. Reuse the HN body verbatim but
trim the intro to two sentences; this crowd wants signal density.

---

# 8. Communities (Discord / Slack — share, don't spam)

Only post where you're already a participant, and only in a #show-and-tell /
#projects channel. One-liner:

```
Built a small open-source thing as a student: AgentGuard, a local Go proxy
that inspects MCP traffic between AI agents and their tools (prompt
injection / tool poisoning / secret leaks). 100% local, MIT. Would love
feedback and it's open to contributors — github.com/nikegunn/agentguard
```

Relevant communities: MCP / Model Context Protocol, Cursor, the various
local-LLM and Claude developer servers, your university's CS / security clubs.

---

## Launch-day checklist

- [ ] README demo GIF recorded and rendering on GitHub
- [ ] `good first issue` + `help wanted` labels exist, with 5–10 real issues
- [ ] Issue templates added to `.github/ISSUE_TEMPLATE/`
- [ ] CONTRIBUTING.md clone URL fixed (`nikegunn/agentguard`, not `<you>`)
- [ ] Release tagged, notes pasted, cosign verify line included
- [ ] GitHub Discussions enabled + welcome post pinned (§3b)
- [ ] `agentguard.space/install` redirect live and tested on a clean machine
- [ ] Post HN (§1) → reply to every comment for the first 2–3 hours
- [ ] Post Reddit (§2), spaced a day apart per sub
- [ ] Post X thread (§4) and LinkedIn (§5)
- [ ] Dev.to article (§6) live, linked from the README
- [ ] **Don't argue. Thank critics. Fix what's fair. Log the rest as issues.**

---

*A final note on tone: the most respected solo maintainers aren't the loudest —
they're the ones who answer every question patiently, credit every contributor,
and treat a bug report as a gift. Post like that and the respect takes care of
itself.*
