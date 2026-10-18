# HTML, WebGPU and Multiplane production

Use HTML for controlled typography, diagrams, title cards, product presentation and layered designed motion. Use native WebGPU/WGSL for compositing and GPU effects. Capture actual gameplay/product behavior from the real build; assemble mixed footage, sound and graphics in an editor when that fits the brief.

## Start a data project

```powershell
video-pv-cli catalog list
video-pv-cli init --dir my-pv --template character-reveal-midnight
video-pv-cli setup --project my-pv
video-pv-cli doctor --project my-pv
video-pv-cli render --project my-pv
```

From the installed skill folder, `go run .` can replace the CLI executable. Go 1.27 builds the tools. Rendering needs Node.js 24+, the exact pinned Multiplane 0.3.10 dependency, Playwright Chromium, a working WebGPU adapter, and FFmpeg with libx264. `setup` uses the checked-in package lock with `npm ci --ignore-scripts`, then explicitly installs Chromium. It invokes Node/local CLI paths directly. Windows x64 can install checksum-verified FFmpeg into the current-user cache; it does not modify PATH.

`init` refuses existing directories. It copies common runtime resources and writes one `template.json`, a local `transitions.json` and a manifest. Edit content, rhythm and styling in the data file; see [template-catalog.md](template-catalog.md). Use `catalog sync` after changing derived timing/profile/image fields. Declare every external asset, module, font and texture in the manifest so hashes and rights checks cover it.

## Shot contract

A fixed-size `#stage` contains the WebGPU canvas. `Shot.define` exposes duration, FPS, setup, cuts and synchronous `render(t)`. Load fonts, images, WGSL modules and GPU pipelines before readiness. Every frame must derive from time `t`, with no RAF, CSS animation, timers, unseeded randomness or prior-frame history. Hard-cut metadata prevents unwanted temporal blending.

The shared renderer reads JSON data; it does not bake copy into HTML. Canvas 2D prepares text/artwork textures and WebGPU performs final compositing. Local fonts are pinned; CJK projects must provide and load a suitable licensed local font rather than relying on machine fallback. Custom WebGPU/compute/3D work follows [shader-production.md](shader-production.md).

The CLI's HTML input represents the full master timeline. For individual shots, use separate manifests and assemble their exports before the final media check. MP4 has no alpha; use a suitable intermediate format/editor for transparent layers before producing the final H.264 master. Do not claim this MP4 CLI produces transparent MP4.

## Logged production gate

`render` runs manifest/data preflight, then five logged Multiplane steps: lint, inspect, verify, stills and render. Inspect must match the manifest profile and identify native WebGPU/WGSL. Verify tests frame determinism; it cannot approve visual taste. Stills/contact sheets and actual MP4 remain review artifacts. The final Go gate probes and fully decodes the export. Input hashes are checked again after rendering; source changes fail the build.

```powershell
video-pv-cli render --project my-pv --out deliverables/revision-02.mp4
video-pv-cli render --project my-pv --draft --out deliverables/preview.mp4
video-pv-cli render --project my-pv --audio soundtrack.wav --quality high
video-pv-cli check --manifest my-pv/pv-project.json --media my-pv/deliverables/master.mp4 --decode
```

Audio must be declared as a manifest asset; prepare a mixed VO/music/SFX track before muxing. `audio_required` enforces a stream. Existing exports are refused. Drafts cannot be released. `--verify-at` deliberately restricts deterministic sampling; a short selection must not be described as full-frame verification. The default verifies the whole timeline. `--renderer-project` reuses dependencies from another prepared local project without installing another copy.

Logs, job IDs, renderer events, input SHA-256, device metadata and technical checks are retained. A SHA-256 guarded bridge makes pinned Multiplane await GPU completion before capturing each frame. Compiler/resource/device failures block success. `--raster software` still requires WebGPU and may expose no adapter in Chromium headless shell; it is not an API fallback. Preserve failed evidence and rerun the same cases after a fix; do not waive the determinism gate.

Default determinism checks explicitly cover every output-frame time, in groups of at most 128 to bound reference-image memory and Windows argument length. Multiplane's own default samples four times; the CLI overrides that default. Reports record the selected sample count and whether it covers the whole timeline. `--verify-at` opts into a smaller explicit selection. Each group retains its own logs and result.

## Windows local validation and release

```powershell
go test -count=1 ./...
go vet ./...
video-pv-cli catalog validate
video-pv-cli catalog verify --runtime-project my-pv --mode deep --out work/catalog-validation
```

No GitHub Actions are used. After watching the final master and recording actual approvals/rights/reviewer details, run `check --release`. Review story, identity, claims, copy, flashes, safe areas, captions, sound, loop seams and platform variants. Automated checks cannot certify creative quality or legal clearance.

Reference: [Multiplane upstream](https://github.com/LBEILC/Multiplane), [delivery checklist](validation.md).
