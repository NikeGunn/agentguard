# Contributing to AgentGuard

Thanks for considering a contribution. AgentGuard is built in public and every
PR matters.

## Dev setup

```bash
git clone https://github.com/<you>/agentguard
cd agentguard
make install-tools
make build
make test
```

You need Go 1.23+ and `make`. On Windows, install `make` via scoop or use WSL.

## Commit style

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(proxy): forward inbound JSON-RPC notifications without buffering
fix(store): close batched writer on context cancel
docs(readme): correct install snippet for fish shell
```

Each commit must answer: what changed, why, and how it was tested.

## PR process

1. Branch off `main` (`feat/...`, `fix/...`, `docs/...`).
2. One feature per commit; squash trivial fixups before opening.
3. `make test` and `make lint` must be green.
4. Every new function ships with a test.
5. New dependency? Justify it in the PR description.

## Reporting bugs

Open an issue with reproduction steps and `agentguard doctor` output.
For security issues, see `SECURITY.md` instead — do not open a public issue.
