# Website backgrounds and engine PV delivery

## Website hero loop

Choose a single-scene periodic `website-atmosphere-*` or `website-product-*` preset. `init --background-only` removes video copy; place headings/CTA in HTML for accessibility, localization and responsive layout. Keep important content inside a crop-safe center and inspect `object-fit:cover` at desktop and phone ratios.

```powershell
video-pv-cli init --dir hero-loop --template website-atmosphere-midnight --background-only
video-pv-cli render --project hero-loop --renderer-project prepared-runtime
video-pv-cli bundle --project hero-loop --target website
video-pv-cli preview --renderer-project prepared-runtime --bundle hero-loop/deliverables/website
```

The website bundle contains H.264/yuv420p MP4 with fast-start metadata, no audio, first-frame poster and an embeddable HTML example. `autoplay muted loop playsinline` is combined with `controls=false`, `tabindex=-1`, an `inert`/`aria-hidden` wrapper and `pointer-events:none`. Foreground content retains normal interaction. Autoplay/decode failure exposes the poster. Reduced-motion preferences suppress video; hidden documents pause it. Browser autoplay policies can still reject playback, so retain the poster.

Use `/delivery-audit` to observe two actual playback wraps and check mute, loop, no controls, no pointer interaction, no focus, inline playback, poster decode and foreground hit testing. In Edge, also click the foreground link. Preview serves only declared delivery files, supports byte ranges and checks hashes at startup. Production hosting needs correct MIME types and Range support; preview is a loopback review server.

The animated export is authored with WebGPU; the website consumes ordinary MP4 and does not need the authoring renderer.

## Engine-independent character introduction MP4

```powershell
video-pv-cli bundle --project character-pv --target mp4
video-pv-cli mp4 verify --input character-pv/deliverables/mp4/video.mp4 --fast-start --report character-pv/qa/mp4-report.json
```

The bundle contains `video.mp4`, `poster.png` and a hash report. Import the MP4 using the target application's ordinary media support. No Unity script, SDK, engine project or engine-side test is part of this tool.

Portable validation requires one H.264 8-bit yuv420p video stream with even dimensions and optional single AAC audio. It parses MP4 box boundaries/brands, checks the first keyframe, decoded frame count, finite monotonic presentation timestamps, constant cadence, near-zero start and duration agreement, then fully decodes video/audio with FFmpeg error detection. Audio start/duration must be within the documented 0.12/0.25 second tolerances. `--silent`, `--require-audio` and `--fast-start` add delivery constraints; bundles always produce fast-start metadata. These checks establish the media contract, without claiming every engine/platform has been tested.

Reports use stable paths and protect the input MP4 against overwrites. The source is read only. A website export is silently remuxed; an engine-independent export preserves optional AAC. Both outputs are inspected and decoded before a successful report.

## Repeatable output

Delivery directories have ownership markers. Unowned nonempty directories are rejected. Reports record source MP4, manifest, embedded recipe and output hashes. Unchanged reruns validate input and reuse output. Changed owned outputs are regenerated via staging that is removed on success or failure. Unknown files are retained. Keep delivery output separate from source folders and user documents.

Automated tests use temporary projects. Set `PV_MEDIA_INTEGRATION=1` for real FFmpeg website/MP4 bundles and run `go test -count=2` to compare file sets/hashes, verify HTTP Range behavior, preserve user notes and check staging cleanup. Browser/catalog evidence uses fixed owned cache directories; retain one current evidence set.
