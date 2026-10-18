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

## Unity character introduction

```powershell
video-pv-cli bundle --project character-pv --target unity
```

Copy `StreamingAssets/pv.mp4` into `Assets/StreamingAssets/pv.mp4`. Add the generated `PVVideoSurface.cs` to a dedicated UI RawImage. It requires VideoPlayer, creates a RenderTexture with export dimensions, prepares before playback and releases resources when disabled. `raycastTarget=false` lets surrounding UI handle input. The loop and audio switches are independent: a character reveal normally plays once with sound; a menu background loops silently.

This is a handoff example, not engine-side certification. Validate compilation in the target Unity/UGUI version, codec/platform support, texture color space, aspect handling, audio routing, enable/disable lifecycle and memory on target hardware. MP4 uses H.264/yuv420p with optional AAC; engine and OS decoding support varies. See the official [VideoPlayer API](https://docs.unity3d.com/6000.3/Documentation/ScriptReference/Video.VideoPlayer.html) and [loop control](https://docs.unity3d.com/6000.3/Documentation/ScriptReference/Video.VideoPlayer-isLooping.html).

For other engines, import the MP4 through its supported media player and route its texture to a non-interactive UI/material. This repository does not include tested Unreal/Godot adapters.

## Repeatable output

Delivery directories have ownership markers. Unowned nonempty directories are rejected. Reports record source MP4, manifest, embedded recipe and output hashes. Unchanged reruns validate input and reuse output. Changed owned outputs are regenerated via staging that is removed on success or failure. Unknown files are retained. Keep delivery output separate from source folders and user documents.

Automated tests use temporary projects. Set `PV_MEDIA_INTEGRATION=1` for real FFmpeg website/Unity bundles and run `go test -count=2` to compare file sets/hashes, verify HTTP Range behavior, preserve user notes and check staging cleanup. Browser/catalog evidence uses fixed owned cache directories; retain one current evidence set.
