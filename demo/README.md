# Demo recordings

This folder contains the source scripts for the demo GIFs used in the
README and the landing page.

## `agentguard.tape` → `agentguard.gif`

The main hero demo. Reproducible via [vhs](https://github.com/charmbracelet/vhs).

```bash
# 1. Install vhs (one-time)
brew install vhs                 # macOS
scoop install vhs                # Windows (scoop)
winget install charmbracelet.vhs # Windows (winget)
# Linux: see https://github.com/charmbracelet/vhs#installation

# 2. Build the binary in the repo root
go build -o agentguard ./cmd/agentguard

# 3. Generate the GIF
vhs demo/agentguard.tape
```

Output lands at `demo/agentguard.gif` — commit it so it shows in the
README and on the landing page.

## Dashboard hero shot (OBS)

The terminal demo above doesn't show the web dashboard. For that, run:

```bash
agentguard dashboard
```

Then record `http://127.0.0.1:7878` in **OBS Studio** at 1920×1080,
30 fps, ~15 seconds:

1. Open the dashboard.
2. In another terminal, trigger a few tool calls through any wrapped
   agent (or run the e2e mock to generate traffic:
   `go run ./e2e/mock_mcp_server | agentguard wrap --upstream-name demo -- cat`).
3. Watch the rows flash in. Capture the live update animation.
4. Save as `demo/dashboard.mp4`, then convert:
   ```
   ffmpeg -i demo/dashboard.mp4 -vf "fps=15,scale=900:-1:flags=lanczos" \
          -loop 0 demo/dashboard.gif
   ```

Drop both GIFs into the README under the Hero section and into the
landing page hero terminal block (replacing the typed-out `<pre>` once
the real GIF is available).
