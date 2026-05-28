# Pipeline stages

See [How it works](./how-it-works.md) for the stage table.

## Verdict lattice

Stages return one of six verdicts:

- `pass` — frame is fine, continue
- `flag` — informational, accumulate but continue
- `transform` — replace `Message.Raw` with `StageResult.Transform` and continue
- `block` — terminate immediately, return error to agent
- `skip` — stage opted out (e.g. no payload to scan), counts as `pass`
- `error` — stage itself failed, treat as block

The chain returns the most severe verdict observed. Block and error
short-circuit; transform mutates the frame for downstream stages.

## Adding a stage

Implement `pipeline.Stage`:

```go
type Stage interface {
    Name() string
    Run(ctx context.Context, m *Message) StageResult
}
```

Hand it to `pipeline.NewChain(...)`. Order matters: cheap rejects go
first so the expensive stages never see them.
