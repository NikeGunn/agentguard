# Publishing checklist — `nikegunn/agentguard`

This is the operator playbook. Everything below is something **you**
run, because I (Claude) have no internet, no GitHub credentials, and
no access to your machine.

When you've finished these steps you'll have:
- A public repo at https://github.com/nikegunn/agentguard
- A live landing page at https://nikegunn.github.io/agentguard/
- A signed v1.0.0-rc1 release with binaries for 6 platforms
- A reproducible README demo GIF

Total time: ~15 minutes.

---

## 1. One-time setup (skip if you've done these)

```bash
# Install GitHub CLI if you haven't
winget install GitHub.cli      # Windows
brew install gh                # macOS
# then:
gh auth login
```

Pick: GitHub.com → HTTPS → Login with web browser → paste the one-time
code. Done.

---

## 2. Create the public repo and push

From the project root (`C:\Users\Nautilus\Desktop\ai agent\agentguard`):

```bash
# Use the GitHub CLI — it creates the repo AND wires the remote in one shot.
gh repo create nikegunn/agentguard \
  --public \
  --description "Zero-trust security sidecar for AI agents — MIT, single Go binary, <5ms p99" \
  --source . \
  --remote origin

git push -u origin main
git push origin v1.0.0-rc1   # the tag I created in M6
```

If you'd rather do it manually (no `gh`):

```bash
# Create the empty repo at https://github.com/new (name: agentguard, public)
git remote add origin https://github.com/nikegunn/agentguard.git
git branch -M main
git push -u origin main
git push origin v1.0.0-rc1
```

---

## 3. Activate GitHub Pages

1. Go to https://github.com/nikegunn/agentguard/settings/pages
2. **Source** → "GitHub Actions"
3. Save.

The `pages.yml` workflow I added will fire on every push to `main`.
First deploy takes ~30 seconds; subsequent deploys are faster.

Verify: https://nikegunn.github.io/agentguard/ should resolve within
2–3 minutes.

---

## 4. Record the demo GIF

```bash
# Install vhs once
winget install charmbracelet.vhs    # Windows
brew install vhs                    # macOS

# Build the binary
go build -o agentguard.exe ./cmd/agentguard

# Generate demo/agentguard.gif
vhs demo/agentguard.tape

# Commit and push
git add demo/agentguard.gif
git commit -m "docs: add reproducible demo GIF"
git push
```

The GIF lands in the README on the next push (no further changes needed
— the README already references `demo/agentguard.gif`).

For the **dashboard hero shot** (browser, not terminal), follow
`demo/README.md` — that needs OBS Studio.

---

## 5. Tag the v1.0.0 final and trigger the signed release

```bash
# When you're ready to publish binaries:
git tag -a v1.0.0 -m "v1.0.0 — initial public release"
git push origin v1.0.0
```

The `.github/workflows/release.yml` workflow I shipped will then:
- cross-build for darwin / linux / windows × amd64 / arm64
- compute SHA-256 checksums
- sign `checksums.txt` with cosign (keyless OIDC — no key to manage)
- attach archives + signatures to the GitHub release

Verify under: https://github.com/nikegunn/agentguard/releases

---

## 6. When you buy `agentguard.dev` (3 days from now)

```bash
# 1. Rename the placeholder CNAME so Pages picks it up
git mv site/CNAME.disabled site/CNAME
git commit -m "feat(site): enable custom domain agentguard.dev"
git push

# 2. At your DNS provider, add these records:
#    CNAME  www  →  nikegunn.github.io
#    A      @    →  185.199.108.153
#    A      @    →  185.199.109.153
#    A      @    →  185.199.110.153
#    A      @    →  185.199.111.153

# 3. In GitHub repo Settings → Pages, set the custom domain to
#    agentguard.dev and check "Enforce HTTPS".
```

---

## 7. Optional polish

- **Sponsor button**: Settings → Funding → check "Display sponsor button",
  add a `.github/FUNDING.yml` (I can add it on request).
- **Topics**: on the repo home page, add topics: `security`, `ai-agents`,
  `mcp`, `prompt-injection`, `golang`, `claude-code`, `cursor`. These
  drive GitHub's discovery feed.
- **Social preview**: Settings → General → Social preview → upload a
  1280×640 PNG (the OG image referenced in `site/index.html`'s
  `og:image` tag — render the landing-page hero to PNG and save as
  `site/og.png`).
- **Discussions**: Settings → Features → Discussions ON. Enables the
  community Q&A linked from the footer.
- **Star yourself**: yes, do it. The first star is a signal.

---

## What I (Claude) couldn't do for you, and why

| Thing                       | Why I couldn't                                    |
|-----------------------------|---------------------------------------------------|
| `gh repo create`            | No GitHub credentials in this sandbox.            |
| `git push`                  | No internet access.                               |
| Record screen / GIF         | No screen, no recording tool, no display.         |
| Activate Pages              | Requires a click in the GitHub UI.                |
| Buy / configure the domain  | Needs your card and your DNS provider.            |

Everything above is at most one command per item. The repo is fully
ready: signed-release workflow, Pages workflow, landing page,
docs site, demo tape, install scripts, 250+ tests passing, p99 gate
enforced.

Hit me up after you push — I can verify the deployed site looks
right and help fix anything that breaks in CI.
