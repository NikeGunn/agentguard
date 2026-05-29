# Demo recordings

Source for the GIFs in the root README and the landing page. Committed GIFs
live in this folder (`demo/*.gif`); everything under `demo/out/` is a throwaway
render workspace and is git-ignored.

| GIF                | Source                         | What it shows                                   |
|--------------------|--------------------------------|-------------------------------------------------|
| `dashboard.gif`    | `playwright/record-dashboard.mjs` | The interactive web dashboard — overview, live feed, call detail, servers, command palette, theme toggle. |
| `doctor.gif`       | `tapes/doctor.tape`            | `agentguard doctor` health check.               |
| `tail.gif`         | `tapes/tail.tape`             | `agentguard tail` live TUI feed.                |
| `agentguard.gif`   | `agentguard.tape`             | Original terminal hero (install → version → help → doctor). |
| `scan.gif`*        | `tapes/scan.tape`             | `agentguard scan` firing the injection corpus.  |
| `install.gif`*     | `tapes/install.tape`          | One-line install + the agent-handoff story.     |
| `onboarding.gif`*  | `tapes/onboarding.tape`       | Paste the landing-page prompt → the agent sets everything up. |

\* Render on demand with the script below.

## Terminal GIFs (vhs)

The `.tape` files render with [vhs](https://github.com/charmbracelet/vhs).

```bash
# One-time tooling
winget install charmbracelet.vhs Gyan.FFmpeg tsl0922.ttyd   # Windows
brew install vhs ffmpeg                                       # macOS (ttyd via brew too)

# Build the demo binaries
go build -o demo/agentguard.exe ./cmd/agentguard
go build -o demo/mock.exe        ./e2e/mock_mcp_server
```

### Windows: use `render-tapes.ps1`, not bare `vhs`

On Windows, vhs drives headless Chrome via the `rod` library and **leaks that
Chrome process on teardown** — it writes the `.gif` but then hangs instead of
exiting. Left unmanaged, the leaked Chromes pile up (all sharing
`%TEMP%\rod\user-data`) until new renders hang at Chrome *startup*. The symptom
is a render that freezes right after the `Set …` directives echo.

`render-tapes.ps1` works around this: it waits for the output GIF to finish
writing (not for vhs to exit), kills vhs, and reaps the orphaned rod Chrome —
and does so before/after every render. It never touches your real browser
(those have no `%TEMP%\rod` profile).

```powershell
cd demo
# Seed the tail demo DB once (seeding inside a tape hangs ttyd on Windows):
.\agentguard.exe seed-demo --db .\out\tail-demo.db --count 120
pwsh -File render-tapes.ps1 doctor tail scan install onboarding
# or render everything in tapes/:
pwsh -File render-tapes.ps1
```

On macOS/Linux the leak doesn't occur — `vhs tapes/doctor.tape` works directly.

## Dashboard GIF (Playwright)

`dashboard.gif` is recorded by driving the real dashboard with Playwright, so
every pixel is the actual UI. It also saves the raw `.webm` (used for the
LinkedIn launch clip).

```powershell
cd demo
npm --prefix playwright install
npx --prefix playwright playwright install chromium

# Seed data + run the live feed + dashboard, then record:
.\agentguard.exe seed-demo --db .\out\dash.db --count 140
Start-Process .\agentguard.exe -ArgumentList "dashboard --db out/dash.db --no-browser"
Start-Process .\agentguard.exe -ArgumentList "seed-demo --db out/dash.db --live"
node playwright\record-dashboard.mjs   # -> out/video/*.webm + demo/dashboard.gif source
```

The recorder writes the raw video to `demo/out/video/` and copies it to your
Desktop. Convert to GIF with the two-pass palette method:

```powershell
ffmpeg -i out\video\<clip>.webm -vf "setpts=PTS/1.8,fps=13,scale=1000:-1:flags=lanczos,palettegen=stats_mode=diff" out\palette.png
ffmpeg -i out\video\<clip>.webm -i out\palette.png -lavfi "setpts=PTS/1.8,fps=13,scale=1000:-1:flags=lanczos[x];[x][1:v]paletteuse=dither=bayer:bayer_scale=3:diff_mode=rectangle" dashboard.gif
```
