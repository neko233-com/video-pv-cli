# Native WebGPU shader production

The supported graphics backend is **WebGPU only**, using WGSL. There is no WebGL fallback. Set the subject, camera, palette and beat first; atmosphere and transitions support the film's direction. A procedural background or a pan over one illustration does not establish gameplay, character acting or product performance.

## Runtime contract

`webgpu-runtime.js` requests an adapter/device, checks shader compilation messages, creates an asynchronous render pipeline during setup, and records actual adapter information. Missing secure context/API/adapter, pipeline failures, device loss and uncaptured GPU errors fail the job. Unsupported hardware must not produce a successful blank export.

`catalog.wgsl` contains the full-screen vertex/fragment pipeline and 48 transition modes. `library.wgsl` contains original noise/math functions. Setup expands local `// @include "library.wgsl"` directives; these are a project preprocessor convention, not WGSL syntax. Include cycles/depth and remote origins are rejected.

The composition painter uses CPU Canvas 2D for font layout and local artwork. It uploads explicit pixel arrays to GPU textures; final compositing, backgrounds and transition shaders run through WebGPU. This is not a second graphics backend. Browser `--raster gpu/software` controls Chromium's raster policy; it does not change `getContext('webgpu')` to another API. The inspect gate requires `meta.gpu.backend='webgpu'` and `shaderLanguage='wgsl'` from the initialized runtime.

Every `render(t)` must set complete state synchronously and submit GPU commands for that time. Upstream Multiplane does not await an async `render`. This CLI applies a small **SHA-256 guarded capture bridge** to pinned Multiplane 0.3.10: the capture callback awaits `__PV_WEBGPU_FLUSH()` before every screenshot. The helper waits for `device.queue.onSubmittedWorkDone()` and checks fatal GPU errors. Unknown/modified upstream capture sources are rejected. The render report records bridge ID and patched source hash; `npm ci` restores upstream code and the next render safely reapplies the bridge. Custom shots must expose this completion hook too.

Pipeline creation, asset decoding, buffer creation and asynchronous preparation belong in setup. Never start RAF/timers, use wall-clock deltas or accumulate simulation state between frames. Queued uploads/draws execute in order. Verify repeated, reordered, reloaded, worker and range captures to detect stale state. Returning a promise from `Shot.render` does not extend the capture contract.

The uniform buffer has a fixed 112-byte layout, mirrored in the WGSL `Params` struct and JS offsets. Resolution/time/progress/mode/strength/softness/tiles/direction/background IDs, padded colors and loop duration are reset explicitly. Review alignment whenever adding a uniform. Textures are RGBA8 unorm with unpremultiplied image data; the shader composites alpha onto an opaque background. Review color conversion, banding and transparent edges in the encoded SDR file.

## Extending the shader library

For an original effect, add a local WGSL module and data parameters, declare every module/texture in the manifest, then implement explicit-time evaluation. A new transition needs its own data file, a unique supported mode, renderer branch, validation range update and endpoint/midpoint coverage. Update the 48-mode coverage test only after adding the implementation and evidence. Keep effects configurable rather than copying another template's JS.

WebGPU also provides compute pipelines, storage buffers/textures, render bundles and 3D render passes. For particles, skinning, fluid simulation or a 3D scene, create resources in setup and submit compute/render passes on the same device queue. Derive state analytically from `t`, precompute a fixed simulation cache, or recompute from a deterministic initial state for each frame; a simulation that advances once per callback fails out-of-order capture. Clear intermediate render targets and avoid temporal feedback from a previous sampled frame. Additional compute/3D effects require their own tested pipeline; the bundled compositor does not claim to provide a complete game engine or physical fluid solver.

Do not import a shader library merely because its name matches a visual effect. Verify its backend, exact revision, license, initialization, time control and actual MP4 output. A GLSL shader must be ported to WGSL; GLSL, Paper Shaders, ShaderGradient and a WebGL material cannot execute directly in this runtime. A future Three.js WebGPU/TSL integration must disable fallback, use an explicit-time renderer, pin compatible versions and pass the same Windows export checks before it is supported. The bundled engine currently has no such dependency. Commercial source licenses and per-asset rights must be recorded separately from this project's Apache-2.0 code license.

## Verification ladder

1. Compile and render one representative shot at the target resolution. Inspect the subject, not only the background.
2. Match dimensions, FPS and runtime; verify actual WebGPU metadata and all local resources.
3. Test reordered times, reloads, worker counts and subranges. Preserve and fix failures; do not loosen pixel thresholds to hide stateful output.
4. Sample transitions, fastest movement, face/hair edges, fine typography, glow and the last frame. Check actual MP4 compression.
5. Export H.264/yuv420p MP4, then fully decode video/audio and check frame count, duration, dimensions, codec and required streams.
6. For loops, compare `t=0` and `t=duration` and also watch the final encoded frame `[duration-1/fps]` returning to zero. Endpoint equality alone does not prove smooth velocity or a perceptually invisible seam.
7. Watch the complete film with sound; record the review before using `--release`.

The default built-in samples are designed motion/key art. Use real captured gameplay/product behavior as evidence, and approved layered/rigged animation for character performance. Decode source video to a controlled frame sequence before using it inside explicit-time HTML; live `<video>` playback is unsuitable.

## Primary references

- [WebGPU specification](https://www.w3.org/TR/webgpu/)
- [WGSL specification](https://www.w3.org/TR/WGSL/)
- [Chrome WebGPU troubleshooting and Windows limitations](https://developer.chrome.com/docs/web-platform/webgpu/troubleshooting-tips)
- [Multiplane shot contract](https://github.com/LBEILC/Multiplane/blob/main/skills/multiplane/SKILL.md)
