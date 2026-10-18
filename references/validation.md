# PV validation and release review

## Run the CLI

From the skill repository, validate the checked-in draft plan:

```powershell
go -C . run . check --manifest examples/pv-project.json
```

For a production project, run from the skill repository and pass a manifest path. `--media` overrides the manifest's `delivery.video_path`; otherwise that path is used if present. The checker treats manifest asset paths as project-relative, checks they stay inside that project folder, and accepts a separate absolute path for a rendered export when supplied through `--media`.

```powershell
go -C . run . check --manifest "D:\projects\my-pv\pv-project.json" --media "D:\projects\my-pv\deliverables\master.mp4" --report "D:\projects\my-pv\qa-report.json"
```

To apply the stricter machine release gate:

```powershell
go -C . run . check --manifest "D:\projects\my-pv\pv-project.json" --release --report "D:\projects\my-pv\release-qa.json"
```

Build a standalone binary with `go -C . build -o video-pv-cli.exe .`. Go 1.27 is required. `ffprobe` is used to inspect a supplied media file. `--release` also runs a full video/audio decode using `ffmpeg`; pass `--decode` to run that check earlier. Both tools can be selected explicitly with `--ffprobe` and `--ffmpeg`.

On Windows x64, if a required FFmpeg tool is missing from `PATH` and the current-user cache, `video-pv-cli check` downloads the FFmpeg essentials ZIP from the Windows build linked by FFmpeg's download page, verifies the published SHA-256, safely extracts it, checks both executable versions, and uses the cached binaries by absolute path. It does not add anything to system PATH. Network access is used only for this missing-tool setup. Use `--auto-install=false` to disable it, `--doctor` to inspect PATH/cache state, `--install-tools` to prepare the cache in advance, and `--tools-dir <path>` to choose a different cache directory. Other operating systems can use FFmpeg already on PATH; automatic cache setup currently supports Windows x64. `video-pv-cli check` does not render or edit the PV, so a game engine or editing application can produce video without FFmpeg.

Manifest-only validation has no FFmpeg or network requirement. Use `--json` for machine-readable standard output. Exit code `0` means no failed checks, `1` means validation failed, and `2` means the command could not run or read its inputs.

## What the automated checks cover

`video-pv-cli render --project <project>` executes five types of logged Multiplane steps (`lint`, `inspect`, one or more `verify` groups, `stills`, `render`) and then the Go media gate with full decode. `inspect` must match the manifest's master dimensions/FPS/runtime before rendering. Local source, shader/library/texture assets, package manifest and lockfile hashes are recorded and checked again after the render. Outputs cannot replace declared inputs; existing MP4s require a new path. NDJSON without a valid final result, a command mismatch or a nonzero renderer exit fails the build. A report is written on pipeline failures where possible.

The `render` input HTML describes a full-timeline master. The CLI accepts `--audio` for one pre-mixed local soundtrack, declared as a manifest asset, and lets the renderer mux it into MP4. Mix VO/music/SFX in the chosen audio tool first. Set `audio_required` to enforce an audio stream. `--draft` is a preview mode and cannot be combined with `--release`. Use `--quality`/`--motion-blur` for sampling control; visual taste and narrative quality come from the source direction.

- Manifest syntax (including duplicate key rejection), schema version, field names, dimensions, frame rate, ratio, and duration.
- Unique IDs, valid shot timing/order, asset and claim references, and claim-to-evidence linkage.
- Local asset existence and containment within the project directory; no path traversal.
- In release mode: all shot/asset approvals and rights states, complete timeline, media file, audio requirement, caption requirement, and recorded human review.
- SRT/VTT cue syntax, order, overlap, positive duration, and bounds against the planned runtime.
- With `ffprobe`: readable stream metadata, container, dimensions, frame rate, codecs, optional pixel format, encoded frame count when available, container/video duration tolerance, and required audio stream. A long audio/container duration does not conceal a short video stream.
- In release mode (or with `--decode`): full video/audio decode through FFmpeg to catch damaged or unreadable streams.
- FFmpeg setup: missing Windows x64 tools are placed in the current-user cache after checksum verification and archive path validation.
- A JSON report for Windows local automation or a later production step. This repository does not use GitHub Actions.

Warnings can indicate that a plan is incomplete but still structurally useful. Release mode turns applicable warnings into failures. A passing report means only that the automated checks passed; it is not a legal opinion or proof that the film is compelling or faithful.

## Human review required

Watch the complete export with sound. Also review a contact sheet or sampled frames from the opening, every major beat, every transition, and the ending. Check:

1. Hook and promise are clear; beats build and pay off within the target runtime.
2. Gameplay/product proof is from the claimed build and depicts the described behavior.
3. Character, logo, product, UI, props, world, geography, and screen direction remain consistent through motion.
4. No generated artifact, malformed text, accidental UI, cursor, loading screen, black frame, flash, or crop distracts or misleads.
5. Captions, CTA, text, logos, and platform overlays are readable and inside safe areas on every aspect ratio.
6. Dialogue, VO, music, and effects are intelligible and balanced; music and voice rights/consents are cleared.
7. Localization, claims, ratings-sensitive content, endorsements, third-party IP, availability, and platform requirements have the right owner approval.

`video-pv-cli check --release` requires a human reviewer attestation in the manifest, but it cannot verify that the review actually occurred. Only set it after completing this checklist and resolving every blocking finding.
