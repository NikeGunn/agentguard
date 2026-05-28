<!-- Thanks for the PR! Quick checklist before merge: -->

## What & why
<!-- One sentence per. What does this change? Why does it need to change? -->

## How
<!-- Bullet-point the approach. Skip if obvious from the diff. -->

## Test plan
- [ ] `go test ./...` passes
- [ ] `golangci-lint run` passes
- [ ] Manual smoke (if user-facing): describe what you ran
- [ ] Bench (if hot path): no regression vs. previous run

## Risk
<!-- What's the blast radius if this is wrong? "Local-only", "race on cold paths", "data migration", etc. -->

## Linked issues
<!-- Closes #123 -->
