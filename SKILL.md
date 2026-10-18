---
name: pv-production
description: "Create and verify game trailers, anime character PVs, keynote-style product promos, and cinematic game website videos. Use for direction, storyboards, data-driven HTML/WebGPU/WGSL motion, transitions, MP4 export, and local Windows QA."
---

# PV Production

Create reviewable promotional videos from a brief through an approved, technically checked deliverable. Work with the user's available capture, image, video, audio, and editing tools; do not assume a particular provider or renderer.

Use the bundled Go 1.27 `video-pv-cli` for local HTML/WebGPU production. From this skill folder, `go run . <command>` works without a preinstalled binary. Choose from 128 independent template JSON files (16 sequence families × 8 palettes), 76 transition JSON files / 48 native WGSL algorithms, 33 scene types and 12 procedural backgrounds. Use `catalog list`, then `init --dir <new-project> --template <id>`. Edit `template.json`; run `catalog sync --project <project>` after timing/profile edits. `setup` installs the locked renderer; `render` produces MP4, contact sheets and reports. Read [references/template-catalog.md](references/template-catalog.md) before selecting or maintaining templates.

The graphics backend is **WebGPU only**, with native WGSL and explicit device metadata. Do not add a WebGL fallback or route through a library that silently chooses one. Unsupported adapters, compilation failures and device loss must fail the job. Run verification locally on Windows; do not create GitHub Actions workflows. Run `check --release` only after actual final-export review.

## Workflow

1. **Frame the brief.** Identify the PV type, audience, platform, runtime, aspect ratio, language, CTA, desired reaction, source build/product/version, available footage and brand assets, claims, rights, budget, approval owner, and delivery deadline. Infer ordinary creative details; flag facts that affect truth, cost, or delivery instead of inventing them.
2. **Choose a production path.** Decide what must be real capture, what may be designed motion, and what can be generated. For precise typography, product UI, diagrams, title cards, and controlled 2.5D motion, consider HTML/CSS/JS rendered with Multiplane; use the workflow in [references/html-production.md](references/html-production.md). For gameplay and product capabilities, use capture from the real build or product as proof. Label cinematic, pre-release, simulated, or generated shots in the plan and, where viewers could mistake them for proof, in the video.
3. **Write the direction bible and beat sheet.** Lock visual identity, character/product invariants, palette, typography, audio direction, continuity rules, and references. Give each shot one purpose, one viewer takeaway, an in/out time, framing and motion, asset parents, copy/audio, and acceptance notes. Keep claims linked to evidence. Use [references/production-playbook.md](references/production-playbook.md) for scope and [references/cinematic-direction.md](references/cinematic-direction.md) for game trailers, anime characters and website hero loops.
4. **Make a small proof first.** Prepare a contact sheet or keyframe board and one representative shot or edit test. Review story clarity, likeness, product truth, typography, and sound before multiplying shots or using a materially costly service. Preserve approved parents and record rejected versions and reasons.
5. **Produce and assemble.** Work shot by shot. Check the connected tool's current input schema, duration, ratio, reference-image limits, retention, and price before submitting jobs. Record source IDs, prompts/settings, job IDs, costs, output paths, rights/provenance, and decisions in the project manifest. Keep clean source captures, generated assets, edit project, captions, and masters separate. Make platform variants from approved shared material.
6. **Run the gates.** For Multiplane shots, run its `lint` and deterministic `verify`, then inspect stills/contact sheets before final rendering. Validate the project manifest and final export with `video-pv-cli check` as described in [references/validation.md](references/validation.md). Review the full video and a contact sheet for narrative, visual, identity, claims, motion artifacts, pacing, safe areas, flashes, and localization. Automated checks do not certify creative quality, legal clearance, or truthfulness; record the human review in the manifest before release checks.
7. **Hand off.** Return the master and variants, edit/source project, captions, storyboard/shot list, asset and claim ledger, machine QA report, human review notes, and unresolved issues. State which checks passed, failed, or could not run.

For website backgrounds, initialize a periodic `website-*` preset with `--background-only`, export MP4, and use `bundle --target website`. Keep it muted, looping, inline, inert and outside pointer/keyboard interaction; provide a poster and respect reduced motion. Use `preview --bundle <bundle-dir>` and `/delivery-audit` for real playback checks. For a game-engine character introduction, use `bundle --target unity` and validate the helper in the actual engine/platform. Read [references/delivery-targets.md](references/delivery-targets.md).

Make automated verification idempotent. Run integration fixtures in temporary directories with automatic cleanup; `PV_INTEGRATION_RUNTIME` supplies dependencies without modifying that project. Run twice with `-count=2`. Reuse the fixed `catalog verify` evidence directory with source/hash/media revalidation. Browser reports replace stable files; delivery bundles reuse matching hashed outputs and remove staging directories. Preserve creative masters and user files. Record whether coverage was all timeline frames, explicit samples, catalog cases or browser cases.

Use `setup --shared` for the pinned current-user Multiplane runtime; reruns reuse it. Rendering/preview/catalog verification can use that cache without copying node_modules into every project. An explicit renderer-project path is binding. `--refresh` intentionally reinstalls the shared dependency; do not refresh it during a live render.

## Operating constraints

- Never present generated or staged imagery as actual gameplay, an observed product result, a customer, a metric, or a real event. Keep factual claims tied to evidence and label footage whose status could mislead.
- Preserve character, product, logo, interface, and world identity using approved references. Do not promise a model can preserve details that have not been inspected across the clip.
- Record source, rights state, attribution, and consent for supplied, licensed, generated, music, voice, and third-party assets. Unknown or restricted material blocks the affected shot from release.
- Keep credentials out of prompts, manifests, logs, and reports. Do not submit paid jobs, publish, upload to an external channel, or contact third parties unless the user has authorized that action.
- Prefer reversible local work and the smallest useful iteration. Do not silently change a claim, feature, story beat, or approved shot to work around a failed generation.

## Resources

- Read [references/production-playbook.md](references/production-playbook.md) when selecting a PV structure or directing game, character, or product shots.
- Read [references/html-production.md](references/html-production.md) when composing designed motion in HTML/CSS/JS and rendering it through Multiplane.
- Read [references/shader-production.md](references/shader-production.md) for native WebGPU/WGSL, local shader modules, deterministic GPU work, device failures and encoded MP4 checks.
- Read [references/template-catalog.md](references/template-catalog.md) for template families, transition parameters, independent data files, timing, showcase generation and maintenance.
- Read [references/cinematic-direction.md](references/cinematic-direction.md) for cinematic pacing, anime character continuity, focal hierarchy and website loop requirements.
- Read [references/project-schema.md](references/project-schema.md) when creating or editing `pv-project.json`.
- Read [references/validation.md](references/validation.md) when checking captions, final media, and the release gate.
- Read [references/delivery-targets.md](references/delivery-targets.md) for looping inert website backgrounds, posters, Unity handoff and idempotent delivery verification.
- `video-pv-cli check` checks the manifest and rendered output; it does not render or edit video. Game engines, capture pipelines, and editing tools can render a PV without FFmpeg. For authored HTML motion, Multiplane renders MP4 and checks shot determinism; see [references/html-production.md](references/html-production.md). Multiplane has its own FFmpeg configuration, separate from `video-pv-cli check`.
- Local media inspection uses `ffprobe`, while `--decode` and `--release` use `ffmpeg` for a full decode. When either tool is missing, the CLI automatically downloads the Windows x64 FFmpeg essentials build to the current-user cache, verifies its SHA-256, and uses it by absolute path without changing system PATH. This download only happens when media inspection needs it. Use `--auto-install=false` to disable downloads, `--doctor` to inspect availability, or `--install-tools` to prepare the cache explicitly. Other platforms need FFmpeg installed separately and available on PATH. The CLI itself requires Go 1.27 to build; manifest-only checks do not require FFmpeg or network access.
