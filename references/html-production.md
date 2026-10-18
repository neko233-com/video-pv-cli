# HTML motion and Multiplane rendering

Use this path when a shot needs exact copy, Chinese/Japanese/Korean text, branded typography, product UI, diagrams, data animation, title cards, controlled transitions, or layered 2.5D motion. HTML/CSS/JS gives direct layout and timing control; it does not create truthful gameplay or product proof, and it does not replace a full nonlinear editor when the PV mixes many live-action/game captures, sound layers, or complex editorial timing.

## Tool roles

- **Multiplane** renders HTML shots into MP4 and provides shot inspection, stills/contact sheets, linting, and deterministic frame verification. It is Apache-2.0. Check its current release notes before installing; its API is pre-1.0 and can change.
- **Go 1.27 `video-pv-cli check`** validates the project manifest, captions, and final encoded file. Keep it as the final delivery gate; it is complementary to Multiplane's shot-level checks.
- **A human reviewer** checks message, aesthetics, claims, likeness, readability, and the complete cut. A deterministic render can still be visually wrong.

## Set up a local project

The supported bundled route is:

```sh
video-pv-cli init --dir my-pv --type character
video-pv-cli setup --project my-pv
video-pv-cli doctor --project my-pv
video-pv-cli render --project my-pv
```

If the CLI is not on PATH, use `go -C <installed-skill-folder> run .` in place of `video-pv-cli`. `init` refuses an existing directory. `setup` installs the template's pinned Multiplane dependency and Chromium; subsequent setup uses `npm ci` with the bundled lockfile. The CLI calls Node and the local renderer directly, avoiding shell interpolation and implicit npx downloads. `render` connects the same resolved FFmpeg binary to Multiplane and final QA, including the Windows x64 user cache when needed.

The HTML is a full-timeline master whose resolution, FPS and duration must match its manifest. For independently rendered shots, use separate shot manifests or the upstream CLI and assemble them in the editor before final `check`. All inputs (including shader includes and textures) should be declared as assets so input hashes and rights review include them.

The original starter is a six-second WebGL2 website hero study. It exercises a custom fragment shader and local `#include` resolution; it is not a finished anime character performance or gameplay trailer. Replace the sample's direction and material with approved assets for the actual brief. See [shader-production.md](shader-production.md).

Check npm package metadata and GitHub release notes for the version you are installing; the README's displayed version can lag the npm package. The documented CLI workflow requires Node.js 24+, Playwright Chromium, and an FFmpeg build with `libx264`; Multiplane does not bundle FFmpeg. Pin the Multiplane version with `--save-exact` and commit the package lock so future renders use a known toolchain. On Linux, install Chromium system dependencies if `doctor` reports them missing.

```sh
npm init -y
npm install --save-exact multiplane
npx playwright install chromium
npx multiplane doctor
```

Use a project-local `package.json` and `package-lock.json`; `--save-exact` records the resolved package version, and the lockfile records the full dependency graph. Check the package's current version, repository, license, and runtime requirements before installation. Keep source art, fonts, audio, HTML shots, and renders in the PV project; do not check in `node_modules` or browser caches. Use local assets and fonts so rendering does not depend on network availability or remote content changes.

## Author shots for deterministic rendering

Each shot is a fixed-size `#stage` and a `Shot.define({ duration, fps, render(t) { ... } })` function. `render(t)` must synchronously set the complete visual state from time `t`; the same time must always produce the same frame. Load local fonts/images in `setup()` or the runtime's ready phase before frames render. Declare hard cuts so motion blur does not blend across them. Avoid wall-clock time, unseeded randomness, CSS animations, timers, and network-loaded resources. Let `multiplane lint` identify additional nondeterministic patterns.

Structure authored shots around the beat sheet. Use one HTML file per designed shot when that makes iteration and approval clearer; a single page may contain a full motion-graphics timeline when all content is designed motion. For mixed PVs, render HTML graphics as independent shots or transparent overlays, then assemble them with captured gameplay/product footage in the chosen editor. MP4 has no alpha; use Multiplane's transparent MOV or WebM output for overlays when the target editor supports the codec.

Keep gameplay and observed product behavior as original captures. Label engine cinematics, simulations, and generated scenes in the shot plan and disclose them in the edit where viewers might mistake them for proof. HTML may frame or annotate authentic evidence, but must not fabricate interface states, metrics, features, customer results, or player actions.

## Render and verify

Run these from the PV project directory; choose a new output path for each iteration.

```sh
npx multiplane inspect shots/reveal.html
npx multiplane lint shots/reveal.html
npx multiplane stills shots/reveal.html --every 0.5 --out renders/reveal-stills
npx multiplane verify shots/reveal.html
npx multiplane render shots/reveal.html --out renders/reveal.mp4
```

`render`, `stills`, and `verify` run lint automatically, but keep the explicit lint step in CI or a logged production run so its result is visible. `inspect` checks the shot's declared timing and stage; `verify` compares pixels rendered under different orders, page reloads, and worker counts. Review the stills/contact sheet for copy, line breaks, safe areas, missing assets, clipping, identity, and beat transitions before rendering a long/high-quality pass. For a smaller proof render, use `--draft`; do not use draft output as the release master.

Capture the Multiplane version, Node, Chromium, FFmpeg build, render command, output path, and result/job ID in a render record. If any input asset changes, rerun lint, verify, stills review, and render. If the renderer reports a nondeterministic frame, fix the source instead of waiving the check for the final master.

## Final master gate

After edit assembly and caption/audio work, validate the actual final MP4 with the Go checker:

```powershell
go -C <skill-root> run . check --manifest <pv-project.json> --media <final.mp4> --release --report <release-qa.json>
```

Use a project manifest whose duration, dimensions, frame rate, audio/caption requirements, claims, and approvals match the final master. `--release` probes stream metadata and fully decodes video and audio. Do not mark human review approved until the complete master and its variants have been watched and the visual, factual, accessibility, and rights checklist in [validation.md](validation.md) is complete.

## References

- [Multiplane repository and current README](https://github.com/LBEILC/Multiplane)
- [Multiplane agent skill](https://github.com/LBEILC/Multiplane/blob/main/skills/multiplane/SKILL.md)
- [PV technical and creative release checks](validation.md)
