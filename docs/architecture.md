# web2video architecture

`web2video` is a Go 1.27 command line application. The actual HTML renderer is
[HyperFrames](https://github.com/heygen-com/hyperframes), pinned to npm 0.8.171;
GSAP is pinned to 3.14.2. Node.js 24 or newer runs that engine. There is no skill
installer, desktop app, Unity dependency, account requirement or cloud renderer.

## Authoring

`init` copies a complete independent project. `project.json` compiles to native
HyperFrames `index.html`, a synchronously registered paused GSAP timeline, local
fonts and local image assets. `build` deterministically regenerates these files;
edit `project.json` for generated projects. To author arbitrary HTML directly,
edit the native composition and do not run `build` afterwards. Render, lint,
check, snapshot and timeline also accept native HyperFrames projects.

100 individual style JSON files provide art direction and keywords. The built-in
compiler applies background/foreground/accent colors and type scale/leading;
the remaining fields, including motion and graphics recipes, guide hand-authored
compositions. Camera, materials, audio and editorial direction remain creative
instructions, not automatically generated cinematography. Four authored example
templates are supplied. These are concept
films using generated still artwork, not captured gameplay, character animation
or verified product demonstrations. The examples are intentionally silent.

## Graphics

The original WGSL library contains 48 transition algorithms and 76 independent
parameter presets. Every generated project uses a raw WebGPU canvas. Image crop
and camera position are functions of absolute frame time; two raster image
textures are composed by WGSL. HTML typography is animated by GSAP. This path
does not require experimental HTML-as-texture browser APIs.

HyperFrames dispatches `hf-seek`. Each listener immediately registers its async
job using `event.detail.waitUntil`; GPU initialization, rendering and
`device.queue.onSubmittedWorkDone()` all finish before capture. There is no
accumulated simulation or autonomous animation clock. A loop uses a periodic
cosine camera function, so seeking to 0 and its duration gives the same state.

Generated projects reject canvas WebGL contexts. The Windows capture browser
also receives `--disable-webgl --disable-webgl2`, enforcing the rule for native
and imported projects. Native HyperFrames components
using WebGL shader transitions are forbidden, and render always passes
`--no-page-side-compositing`. Hardware WebGPU is required. Missing adapters,
devices, WGSL compile errors and device loss fail the render. HyperFrames itself
uses a small WebGL hardware capability probe outside the authored composition;
it is not the render backend.

### Windows compatibility bridge

[HyperFrames issue #5775](https://github.com/heygen-com/hyperframes/issues/5775)
documents the managed Windows headless shell missing DXC DLLs. The Go wrapper
adds `--disable-dawn-features=use_dxc` to the native browser argument builder on
Windows, selecting Dawn's other native shader compiler. This does not enable
WebGL or software rendering. The patch accepts only one exact SHA-256 of the
pinned upstream bundle and its exact patched result; changed bytes fail closed.
The installed dependency keeps its original license. No Microsoft DLLs are
redistributed. See `engine_windows_bridge.go`.

## Export and repeatability

Render stages output inside an owned `.web2video` temporary directory. It passes
native strict lint and media readiness gates, disables extracted-frame caching,
and uses a single screenshot capture worker. The resulting H.264/yuv420p MP4 must
pass bounded MP4 box parsing, fast-start, stream policy, decoded frame count,
monotonic PTS, constant cadence, source dimensions/fps/duration and a complete
FFmpeg decode. A source hash is checked again before publishing the output.

An existing matching receipt reuses the MP4 only after hashing and fully
verifying it again. Changed inputs or a corrupt output cannot count as a cache
hit. Explicit `--force` permits replacement after a new staged render succeeds.
A project job lock rejects overlapping render/snapshot jobs. A crash may leave
that lock; its recorded PID must be checked before manually removing it.

Temporary browser profiles, history and extraction files live under the owned
job directory and are removed on normal success/failure. Snapshot output uses
one fixed directory. Source MP4 footage participates in input hashes; exported
MP4 files are globally ignored by Git. No GitHub Actions are supplied.

`preview` runs a loopback Go HTTP server with a small read-only player. It loads
the real pinned HyperFrames browser runtime and uses its native player API for
play, pause and seek. It does not open Studio or record edit history. Keep the
terminal running and use its localhost URL. Direct `file://` opening cannot run
the HyperFrames runtime or load module resources correctly.

Snapshot/check revision recording is disabled by two exact SHA-256 guarded
local opt-out hooks in the installed engine. This prevents upstream UUID history
and artifact copies from accumulating during tests; explicit wrapper snapshots
remain in one owned directory. Browser profiles live in temporary user cache
jobs outside the project source and are cleaned after each invocation. Both
compatibility bridges are documented source changes, not silent version drift.
