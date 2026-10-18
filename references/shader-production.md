# Shader cinematography and MP4 export

Use shaders to support the shot's subject: elemental energy behind a character, controlled volumetric atmosphere, water/refraction, portal or logo reveals, particle depth, dissolve/mask transitions, film treatment, and restrained bloom. Set the subject's identity, story beat, palette and focal point first. An animated gradient by itself is not a complete game or character PV.

## Renderer choices

| Path | Appropriate use | Export requirement |
| --- | --- | --- |
| Original GLSL + native WebGL2 | Custom full-screen atmosphere, distortions, procedural textures; minimal dependencies | Use the starter's local include loader, compile/link checks, synchronous `render(t)` and MP4 validation |
| Paper Shaders (`@paper-design/shaders`) | Designed website textures, gradients, image treatments | Pin the selected release, bundle locally with LICENSE/NOTICE, stop automatic animation and drive its public `setFrame(t * 1000)` in zero-speed mode; verify the exact bundle before delivery |
| Three.js `ShaderMaterial` / `RawShaderMaterial` | Lit 3D objects, image cards, meshes and camera-driven composition | Pin a version compatible with the selected Multiplane release; set uniforms and camera from `t`; render explicitly |
| `multiplane/three` | Image cards, depth of field, dimensional UI | Follow that release's three.js version requirement; no independent animation loop |
| ShaderGradient | Website atmosphere or a designed background | Disable automatic animation (`animate='off'`), set `uTime` from `t`, disable pointer/scroll/spring state, and use a manually advanced render loop; prove out-of-order determinism before adopting |
| LYGIA or other shader function collections | Reusable math, noise, filters, SDFs | Check the exact revision's license, vendor required modules locally, resolve includes before compilation and keep notices |

ShaderGradient's upstream README lists MIT; LYGIA's current LICENSE is Prosperity Public License 3.0.0 with a commercial trial limit. Do not assume either a project name or an old license covers a commercial game trailer. Confirm the selected revision and obtain appropriate rights when needed. Shadertoy examples also have author-specific terms; a publicly visible shader is not automatically reusable. The included original GLSL has no LYGIA or ShaderGradient dependency.

Paper Shaders upstream explicitly permits commercial end products under Apache-2.0 and asks redistributors to preserve LICENSE/NOTICE. Its vanilla `ShaderMount` supports `speed=0` and `setFrame` in milliseconds; it also contains internal wall-clock and resize-observer code. Keep speed zero, finish sizing before ready, set a known frame for every `render(t)`, and inspect the vendored source and lint findings. Do not globally suppress wall-clock findings or assume a live React component will be deterministic. The bundled templates test the original native-WebGL helper; Paper/Three/ShaderGradient are optional integrations that need their own verified shot, not blanket compatibility promises.

## Deterministic WebGL contract

- Load shader sources, includes, textures, geometry and local fonts during `setup()`/`ready`. Fail on failed requests, include cycles, shader compile/link errors, lost WebGL contexts or GL errors. Never replace an unavailable shader with an unnoticed blank canvas.
- Set `u_time = t` and `u_resolution` to the actual framebuffer dimensions for every frame/subsample. Use explicit fixed seeds. Device pixel ratio affects canvas backing storage, not the shot's logical composition dimensions.
- Derive camera transforms, material uniforms, particles and animation clip pose directly from `t`. Do not call an elapsed-time clock, start `requestAnimationFrame`, accumulate positions, or rely on React's asynchronous state commit before the screenshot. Reset all mutable state on each call.
- Temporal effects need special handling. Replace feedback trails, TAA history, physics and interactive pointer effects with a deterministic precomputation or a closed-form function. Reset bloom/compositor/render-target state between sampled frames. Verify at hard cuts, at the fastest action, near the ending and after arbitrary time jumps.
- Use a consistent linear lighting/tone-mapping pipeline and a known display transform. Avoid double gamma conversion. Review glow clipping, banding, moire, HDR-looking highlights flattened into SDR, and transparent edge halos in the encoded MP4.
- Keep render resources offline. Native WebGL does not resolve `#include` itself; the starter's JS loader resolves local nested includes in setup. Bundle third-party code in advance rather than fetching a CDN during frame rendering.

The built-in helper creates WebGL2 with a preserved drawing buffer, checks compile/link status and GL errors, and renders once per explicit `t`. The CLI runs Multiplane `verify` to test repeated/reordered calls and worker consistency. Small GPU/environment differences can still exist across computers; record the browser and renderer versions and inspect warnings. WebGPU/TSL is a separate path: only claim it is supported after the target browser, adapter, renderer and MP4 export have been exercised. This project currently exercises WebGL2.

## Verification ladder

1. Run a short representative shader shot at final resolution. Inspect the hero frame before building the full cut.
2. Run lint and inspection; confirm local resources, stage size, exact FPS and duration match the manifest.
3. Run determinism verification. Fix failures before exporting. A visual match at only the opening frame is insufficient.
4. Render stills at transitions, focal changes, extremes of movement and the last encoded frame. Examine copy, face/hair edges, weapons/hands, bloom, grain and fine detail at 100%.
5. Export H.264 MP4 with yuv420p for the declared SDR delivery profile. MP4 does not carry alpha; composite transparent MOV/WebM layers before the final export. Use even target dimensions for 4:2:0.
6. Fully decode the result and check stream size, FPS, duration, frame count and audio/caption requirements. Watch the actual MP4 with sound, including full-screen and mobile framing. A passing shader compile does not prove a good encoded image.

## Game, anime and website direction

For a game trailer, connect atmosphere to real player verbs and stakes. Use actual gameplay as proof; place designed titles and transitions around the capture. Convert video to a controlled frame sequence if it must be sampled inside a deterministic HTML shot; ordinary `<video>` playback is not controlled by `render(t)`.

For anime character PVs, build a continuity sheet covering face, hair silhouette, costume, accessory position, proportions, eye color and signature weapon. Use approved layered artwork or real rigged animation for character action. Direct close-up, silhouette, signature action and emotional payoff as distinct beats. A camera pan over one illustration may be suitable for an introductory moment; do not present it as finished character animation. Keep effects behind face/eyes and preserve line-art clarity. See [cinematic-direction.md](cinematic-direction.md).

For website hero loops, make the loop join a deliberate shot requirement. A six-second loop uses frames `[0, 6)`; confirm the transition from the final encoded frame to the first has no perceptible jump. Use periodic shader motion, restrained typography and enough dark/quiet space for actual website copy. Deliver a static poster plus an optimized silent MP4 variant; review site overlays and reduced-motion treatment separately. Native HTML preview alone does not prove a good website video.

## Primary references

- [Multiplane shot and determinism contract](https://github.com/LBEILC/Multiplane/blob/main/skills/multiplane/SKILL.md)
- [Three.js ShaderMaterial](https://threejs.org/docs/pages/ShaderMaterial.html)
- [ShaderGradient upstream README](https://github.com/ruucm/shadergradient)
- [Paper Shaders upstream README and license](https://github.com/paper-design/shaders)
- [Paper ShaderMount time control](https://github.com/paper-design/shaders/blob/main/packages/shaders/src/shader-mount.ts)
- [LYGIA upstream license](https://github.com/patriciogonzalezvivo/lygia/blob/main/LICENSE.md)
