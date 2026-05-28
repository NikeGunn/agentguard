# AgentGuard Developer Manual

The operator playbook. Open this when you forget how to do something.
Everything is copy-paste ready for PowerShell on Windows; Bash sections
are marked. Time estimates next to each task assume you're alone and
caffeinated.

---

## 0. One-time setup

Skip this section after the first time.

```powershell
# 1. Tools (winget)
winget install GoLang.Go
winget install GitHub.cli
winget install Git.Git
winget install charmbracelet.vhs     # demo GIFs only
winget install Gyan.FFmpeg            # vhs dep
winget install tsl0922.ttyd           # vhs dep

# 2. GitHub login
gh auth login                         # follow the prompts

# 3. Clone + dependencies
git clone https://github.com/NikeGunn/agentguard
cd agentguard
go mod download

# 4. Optional: install golangci-lint v2 (matches CI exactly)
$env:Path = "$env:USERPROFILE\go\bin;" + $env:Path
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
```

If your PowerShell loses `~/.agentguard/bin` from PATH on shell startup
(Anaconda's `conda init` strips it), the fix is already in
`$PROFILE`. If a fresh box again: append this to
`$env:USERPROFILE\Documents\WindowsPowerShell\profile.ps1`:

```powershell
$env:Path = "$env:Path;$env:USERPROFILE\.agentguard\bin"
```

---

## 1. Daily loop — edit, test, commit, push

Total time: ~3 minutes for a small fix.

```powershell
# 1. branch off main (optional for tiny fixes; mandatory for risky work)
git checkout -b fix/short-description

# 2. edit code in your editor
# ...

# 3. local smoke test BEFORE you commit
go build ./...                        # must compile across all packages
go test ./...                         # all tests pass
golangci-lint run --timeout=5m        # lint clean (matches CI)

# 4. commit (Conventional Commits style)
git add -A
git commit -m "fix(proxy): one-line summary in imperative

Body explaining WHY (not what). Reference issues/PRs if relevant.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"

# 5. push
git push -u origin fix/short-description    # if on a branch
# OR
git push origin main                        # if you pushed straight to main
```

### Commit message format

Type prefix is enforced by repo norm (not by hooks):

| Prefix | When |
|--------|------|
| `feat(scope):` | new user-visible feature |
| `fix(scope):` | bug fix |
| `chore(scope):` | refactor, tooling, no behavior change |
| `docs(scope):` | docs only |
| `ci(scope):` | workflow/CI changes |
| `deps(scope):` | dependency bumps |
| `test(scope):` | tests only |

`scope` is the affected package or area: `proxy`, `cli`, `dashboard`,
`store`, `pipeline`, `site`, `release`, etc.

---

## 2. Cutting a new release

You ship via git tags. CI does the rest.

```powershell
# 1. Make sure main is at the commit you want to release
git checkout main
git pull

# 2. Decide the version number (semver)
#    v1.0.X = bug fix          (no behavior change to users)
#    v1.X.0 = new feature       (backward-compatible)
#    vX.0.0 = breaking change   (uninstall pre-existing wraps recommended)

# 3. Tag + push
git tag -a v1.0.2 -m "v1.0.2 - short description

Longer notes if needed. These show on the GitHub release page."
git push origin v1.0.2
```

That's it. The release workflow (`.github/workflows/release.yml`)
auto-triggers on `v*` tags:

1. Cross-builds 6 binaries (darwin/linux/windows × amd64/arm64).
2. Computes SHA-256 for each archive.
3. Signs `checksums.txt` with cosign keyless OIDC.
4. Attaches archives + `checksums.txt` + `.sig` + `.pem` to a GitHub release.
5. Updates the `latest` pointer.

New users running `curl -fsSL https://agentguard.space/install | sh`
get the new binary automatically because the install script asks the
GitHub API for "latest release".

### Watching a release

```powershell
gh run watch --repo NikeGunn/agentguard            # live tail
gh release view v1.0.2 --repo NikeGunn/agentguard  # confirm assets
```

### If a release fails

```powershell
gh run list --repo NikeGunn/agentguard --workflow release --limit 3
gh run view <run-id> --log-failed --repo NikeGunn/agentguard
# Fix the issue, retag (delete old tag locally + remote, recreate):
git tag -d v1.0.2
git push origin :refs/tags/v1.0.2
gh release delete v1.0.2 --repo NikeGunn/agentguard --yes
git tag -a v1.0.2 -m "..."
git push origin v1.0.2
```

### IMPORTANT: code on main !== released binary

Pushing to main only updates source. Users still get the binary tied to
the latest **tag**. After every bugfix that affects users, you must cut
a new release tag.

---

## 3. Updating the landing page (agentguard.space)

The site lives in `site/`. Push to main triggers `.github/workflows/pages.yml`.

```powershell
# 1. edit site/index.html, site/style.css, site/script.js
# 2. preview locally (recommended)
python -m http.server 8765 --directory site
# open http://127.0.0.1:8765/

# 3. commit + push
git add site/
git commit -m "site: short description of change"
git push origin main

# 4. wait ~30s, hard refresh https://agentguard.space/
```

If Pages doesn't pick up the change:
```powershell
gh workflow run pages.yml --repo NikeGunn/agentguard --ref main
gh run watch --repo NikeGunn/agentguard
```

### Custom domain notes

- `site/CNAME` contains `agentguard.space` and tells Pages to serve there.
- Namecheap DNS has 4 A records (185.199.108-111.153) + 1 CNAME `www → nikegunn.github.io.`.
- HTTPS is enforced server-side; cert auto-renews via Let's Encrypt.

---

## 4. Updating the docs site (mdbook)

```powershell
# 1. edit files in docs/src/
# 2. preview locally (optional)
mdbook serve docs              # http://localhost:3000

# 3. commit + push
git add docs/
git commit -m "docs(scope): what changed"
git push origin main

# 4. docs.yml workflow builds and uploads as artifact (not auto-deployed yet)
```

To wire mdbook output to its own Pages site later: build → upload to a
`gh-pages` branch or a second Pages site. Today the source markdown is
the canonical docs; the built HTML is a CI artifact only.

---

## 5. Re-recording the demo GIF

Required only when the TUI/CLI changes meaningfully.

```powershell
# 1. Build a fresh binary that the tape will exercise
go build -o agentguard.exe ./cmd/agentguard

# 2. Render the GIF (needs vhs + ttyd + ffmpeg installed)
vhs demo/agentguard.tape
# output: demo/agentguard.gif

# 3. Preview, then commit
Start-Process demo/agentguard.gif
git add demo/agentguard.gif
git commit -m "docs(demo): refresh GIF for vX.Y.Z"
git push origin main
```

If vhs gives `sh: not recognized`: open `demo/agentguard.tape`, ensure
`Set Shell "powershell"` (not `"bash"`).

---

## 6. Adding a new MCP server detector

When a new AI agent comes along (e.g. Aider, Cline, Continue):

```powershell
# 1. Find that agent's MCP config path on disk
#    (usually ~/.<agentname>/<something>.json or ~/.config/<agentname>/...)

# 2. Copy an existing detector as a template
cp internal/agent_detect/cursor.go internal/agent_detect/aider.go
# Edit: Kind, DisplayName, config paths, server-array key

# 3. Register the new detector
# Edit internal/agent_detect/detect.go DetectAll() - add to the list

# 4. Add a test
cp internal/agent_detect/cursor_test.go internal/agent_detect/aider_test.go
# Adapt fixtures

# 5. Run
go test ./internal/agent_detect/
agentguard init --interactive   # confirm the new agent appears

# 6. Commit + push + (optional) release
```

---

## 7. Adding a new rule pack

```powershell
# 1. Write YAML in internal/policy/builtin/<name>.yaml
# 2. Test compile
agentguard pack verify <name>
agentguard pack show <name>

# 3. Smoke-run with a wrapped server
agentguard replay --pack <name> --since 24h
```

User-defined packs live in `~/.agentguard/packs/*.yaml` and don't
require recompilation; they're loaded at runtime.

---

## 8. Live demo at the dashboard

```powershell
# 1. Start dashboard (in window 1)
agentguard dashboard

# 2. Run the demo driver (window 2)
drive-demo -round=all          # benign + injection + leaks
drive-demo -round=injection    # just the injection scenario
drive-demo -round=leaks        # just secret redaction
```

If `drive-demo` isn't found:
```powershell
go build -o "$env:USERPROFILE\.agentguard\bin\drive-demo.exe" ./demo/drive
```

The demo wires the bundled `mock-mcp.exe` server through agentguard
wrap and pipes JSON-RPC frames at it. Verdicts appear in the dashboard
under http://127.0.0.1:7878 within seconds.

To rebuild the mock:
```powershell
go build -o "$env:USERPROFILE\.agentguard\bin\mock-mcp.exe" ./e2e/mock_mcp_server
```

---

## 9. Debugging CI failures

```powershell
# List recent runs
gh run list --repo NikeGunn/agentguard --branch main --limit 5

# Look at a specific failing run
gh run view <run-id> --repo NikeGunn/agentguard
gh run view <run-id> --log-failed --repo NikeGunn/agentguard | Select-Object -First 100

# Re-run a flaky job (don't re-run if the failure is real)
gh run rerun <run-id> --failed --repo NikeGunn/agentguard
```

### Common failure modes

| Failure | Cause | Fix |
|---------|-------|-----|
| `golangci-lint exit 3` | Lint config is v2 but action installed v1 | Action pinned to `golangci/golangci-lint-action@v8` w/ `version: v2.5.0` |
| `Verify go.mod is tidy` | You added a dep but forgot `go mod tidy` | `go mod tidy && git add go.mod go.sum && git commit -m "deps: tidy"` |
| `cgo.exe exit 2` | Race-test needs CGO; only runs on Linux CI | Local race tests skipped on Windows. CI Linux is the source of truth. |
| `gofmt` complaints | UTF-8 BOM at file start or whitespace | `gofmt -w .` then re-commit |
| Branch protection blocks merge | Some required check failed | `gh pr checks <pr#>` shows which; fix that check |

---

## 10. Handling Dependabot PRs

Dependabot opens grouped PRs every Monday under `.github/dependabot.yml`.

- **Patch + minor bumps** → auto-merged by `dependabot-auto-merge.yml`
  once CI is green. No action needed.
- **Major bumps** → land in your inbox for review. Inspect release notes
  for breaking changes:
  ```powershell
  gh pr view <pr#> --repo NikeGunn/agentguard
  gh pr checks <pr#> --repo NikeGunn/agentguard
  ```
  If you trust the bump:
  ```powershell
  gh pr merge <pr#> --squash --delete-branch --repo NikeGunn/agentguard
  ```
  If you want to wait:
  ```powershell
  gh pr close <pr#> --comment "Holding for evaluation" --delete-branch --repo NikeGunn/agentguard
  ```

To ask Dependabot to rebase a stale PR against current main:
```powershell
gh pr comment <pr#> --body "@dependabot rebase" --repo NikeGunn/agentguard
```

---

## 11. Branch protection on main

Currently configured (solo-dev mode):

- No force-push, no deletion
- Linear history required (no merge commits)
- 8 required status checks: `Lint and test`, `e2e (POSIX shell)`,
  6× `Cross-build`
- No required reviews (you're the only reviewer)
- Conversation resolution required

To adjust (rare):
```powershell
gh api -X PUT repos/NikeGunn/agentguard/branches/main/protection --input some.json
```

To temporarily bypass for an emergency fix:
```powershell
gh api -X DELETE repos/NikeGunn/agentguard/branches/main/protection
# do the thing
gh api -X PUT  repos/NikeGunn/agentguard/branches/main/protection --input some.json
```

---

## 12. Hot fixes (production-critical)

When a v1.X.X user is seeing crashes:

```powershell
# 1. Reproduce locally
# ...

# 2. Write a failing test that catches the bug, in the appropriate
#    package. Run only that test:
go test -run TestNameOfNewTest ./internal/<pkg>/
#    Expect: FAIL

# 3. Write the smallest fix
# 4. Re-run the test
go test -run TestNameOfNewTest ./internal/<pkg>/
#    Expect: PASS

# 5. Run the full suite
go test ./...

# 6. Commit + push
git add -A
git commit -m "fix(scope): one-line summary

Repro: ...
Cause: ...
Fix: ..."
git push origin main

# 7. Tag + release immediately (patch bump)
git tag -a v1.X.(Y+1) -m "v1.X.Y+1 - hotfix description"
git push origin v1.X.(Y+1)
```

---

## 13. Reverting a bad release

If a release ships broken:

```powershell
# 1. Roll back the install pointer by re-tagging an older version as
#    'latest' is harder than just shipping a v1.X.(Y+1) patch that
#    reverts the change. PREFER that.

# 2. Hard rollback (rarely needed): delete the bad release so 'latest'
#    falls back to the previous tag:
gh release delete v1.X.Y --repo NikeGunn/agentguard --yes
git push origin :refs/tags/v1.X.Y
git tag -d v1.X.Y

# 3. Communicate: pin a note to README, post in Discussions.
```

---

## 14. File layout cheat sheet

```
agentguard/
├── cmd/agentguard/             # main.go - entry point
├── internal/
│   ├── cli/                    # cobra commands (init, dashboard, tail, ...)
│   ├── proxy/                  # stdio JSON-RPC pump
│   ├── pipeline/               # Stage interface + 6 stages
│   ├── ml/                     # heuristic classifier
│   ├── policy/                 # YAML rule pack engine + builtin packs
│   ├── store/                  # SQLite, batched writer, migrations
│   ├── dashboard/              # chi HTTP server + SSE + embedded SPA
│   ├── agent_detect/           # per-agent config detectors + patcher
│   ├── registry/               # npm/PyPI/GitHub metadata + trust score
│   ├── otel/                   # OTLP/HTTP span exporter
│   └── version/                # build-time version stamp (ldflags target)
├── e2e/                        # end-to-end tests + mock_mcp_server
├── bench/                      # performance benchmarks + p99 gate
├── docs/                       # mdbook source for the docs site
├── site/                       # landing page (agentguard.space)
├── scripts/                    # install.sh, install.ps1
├── demo/                       # vhs tape + drive/ (live demo)
└── .github/
    ├── workflows/              # ci.yml, release.yml, pages.yml, ...
    ├── dependabot.yml          # weekly grouped bumps
    ├── CODEOWNERS              # routes reviews to @NikeGunn
    └── pull_request_template.md
```

---

## 15. Quick commands you'll forget

```powershell
# Inspect what's recorded in the local DB
sqlite3 "$env:USERPROFILE\.agentguard\data\agentguard.db" "SELECT verdict, COUNT(*) FROM tool_calls GROUP BY verdict;"

# Tail logs (TUI)
agentguard tail

# Wipe local state for a fresh demo
Remove-Item "$env:USERPROFILE\.agentguard\data\agentguard.db*" -Force

# Reset install (uninstall + reinstall)
agentguard uninstall --purge
iwr -useb https://agentguard.space/install.ps1 | iex

# Generate a quick rule pack
@'
name: my-pack
version: 1
rules:
  - id: block_aws_keys
    when:
      content_regex: "AKIA[0-9A-Z]{16}"
    action: block
    reason: "AWS access key"
'@ | Out-File -Encoding utf8 "$env:USERPROFILE\.agentguard\packs\my-pack.yaml"
agentguard pack verify user/my-pack

# Force a rebuild from source (avoid `dev (none, unknown)` issue)
go build -ldflags "-X github.com/agentguard/agentguard/internal/version.Version=v1.0.X-local" -o "$env:USERPROFILE\.agentguard\bin\agentguard.exe" ./cmd/agentguard
```

---

## 16. When in doubt

1. Look at recent commits for the pattern: `git log --oneline -20`.
2. The CHANGELOG.md has a milestone-by-milestone record of every
   feature and its rationale.
3. Issues on the repo: https://github.com/NikeGunn/agentguard/issues
4. Discussions: https://github.com/NikeGunn/agentguard/discussions
5. CI logs: `gh run view <run-id> --log --repo NikeGunn/agentguard`

You built this. You know it better than anyone. The tooling makes the
slow stuff fast — most "how do I X?" questions have an `agentguard
<command>` answer.
