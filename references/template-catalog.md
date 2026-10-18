# Data-driven template and transition catalog

## Inventory and selection

128 independent template JSON files = **16 sequence families × 8 palettes**. These are 16 distinct editorial structures with color/motion/transition variants, not 128 unrelated film concepts. Every preset is checked in separately under `assets/catalog/templates/<id>.json`; runtime does not regenerate it. Each of the 76 transition configurations also has its own `assets/catalog/transitions/<id>.json`. They cover 48 WGSL algorithms and directional variants.

| ID prefix | Structure |
| --- | --- |
| `keynote-product` | Device reveal, macro, focal feature, bento, specifications, end card |
| `keynote-type` | Giant type, kinetic copy, quote, title, end card |
| `keynote-features` | Title, bento, statistics, side-by-side comparison, end card |
| `keynote-material` | Macro, object rotation, material panels, logo, end card |
| `keynote-gallery` | Gallery, zoom gallery, film strip, split screen, end card |
| `game-launch` | Key art, chapter, action panels, feature grid, title, CTA |
| `game-world` | Cinematic frame, map trace, quote, world image, title |
| `game-combat` | Impact copy, action panels, split screen, stats, title |
| `game-event` | Countdown, chapter, feature grid, key art, CTA |
| `character-reveal` | Detail, name reveal, signature motif, title, end card |
| `character-profile` | Key art, profile grid, quote, signature motif, title |
| `character-story` | Chapter, detail, quote, name reveal, end card |
| `website-atmosphere` | One restrained periodic atmosphere loop |
| `website-product` | One periodic floating product loop |
| `social-teaser` | Giant type, key art, impact, end card |
| `brand-logo` | Geometric logo, monogram, title, end card |

Append a palette: `obsidian`, `porcelain`, `graphite`, `midnight`, `aurora`, `ember`, `sakura`, `monochrome`. Start restrained product work with `keynote-product-porcelain`; start anime direction with `character-reveal-midnight`. Keynote-inspired pacing and geometry do not grant use of Apple trademarks or product assets.

```powershell
video-pv-cli catalog list
video-pv-cli catalog list --category character --json
video-pv-cli catalog list --kind transitions
video-pv-cli catalog show --id keynote-product-porcelain
video-pv-cli init --dir my-pv --template keynote-product-porcelain --title AURA
```

## A project is data plus a shared renderer

| File | Responsibility |
| --- | --- |
| `template.json` | One editable composition: copy, assets, rhythm, palette and scene choices |
| `transitions.json` | Local transition definitions keyed by ID; safe to tune per project |
| `pv-project.json` | Delivery profile, derived beat windows, asset/claim ledger, approvals |
| `pv.html` | Shared stage and synchronous `Shot.define` entry point |
| `catalog-runtime.js` | Shared scene painter, explicit-time schedule and transition selection |
| `catalog.wgsl`, `library.wgsl` | Native GPU compositor, effects and local shader functions |
| `webgpu-runtime.js` | Adapter/device checks, pipelines, textures, uniforms and GPU submission |

Visible text is data, including item labels and copy. The product object is an original procedural design study. The included generated character is sample key art; replace it with approved artwork, layered animation or captured material for a real project. Scene type names do not imply rigged character acting, real gameplay, volumetric rendering or measured product performance.

Top-level fields: `schema_version` (`1.0`), `id`, `name`, `category`, `description`, `project_title`, `width`, `height`, `fps`, `loop`, `theme`, `scenes`. Theme colors are `#rrggbb`; font is `sans` or `display`. The bundle includes Manrope and Cinzel. For CJK localization, supply a licensed local CJK font, declare it as an asset, add a font-face and load it in `Shot.define.fonts`; system fallback does not guarantee identical glyphs across machines.

Every scene has an ID, `type`, `duration`, title/subtitle/overline, local `image` path, `background`, layout, item array, motion and outgoing transition. `{{title}}` expands to `project_title`. Text wraps inside each layout; unusually long copy still needs visual review. `layout` offers left/right/center/asymmetric direction hints; individual scene types control their own primary geometry.

33 scene types: title, giant-type, kinetic-type, quote, endcard, logo, monogram, hero-device, feature-focus, bento, spec-grid, compare, product-spin, macro, material-panels, gallery, zoom-gallery, film-strip, split-screen, image-hero, chapter, action-panel, landscape, map-trace, impact, stat-cards, countdown, character-detail, character-name, signature, profile-grid, ambient, orbital.

12 backgrounds: solid, mesh, aurora, nebula, grid, particles, rings, caustics, fluid, prism, horizon, silk. Motion choices: none, push-in, pull-out, drift-left/right/up, orbit, float, tilt, rise, stagger (staggered item grids). All motion derives from the supplied time.

## Timing and editing

The outgoing transition overlaps the next scene. Total duration is the sum of scene durations minus outgoing overlaps for every scene except the last. The manifest's contiguous beat windows describe primary scene ownership; overlapping render layers use the declared assets. Incoming animation continues through the overlap. Three simultaneous scenes are rejected. A `cut` switches at the incoming scene's start and is declared in the shot's hard-cut metadata.

After editing timing, dimensions, FPS or image references:

```powershell
video-pv-cli catalog sync --project my-pv
video-pv-cli check --manifest my-pv/pv-project.json
video-pv-cli render --project my-pv --out deliverables/revision-02.mp4
```

Sync saves `pv-project.json.before-sync`, rebuilds draft shots, clears review approval and adds new local image assets with pending rights. It preserves the claim ledger; reassociate claims with the new shots explicitly. Preserve/move the backup before another sync. Copy-only edits do not require sync unless they change derived fields. Existing exports are never replaced.

## Transition families and parameters

Editorial: cut, crossfade, dip-black, dip-white. Directional geometry: slide, push, cover, reveal in eight directions each. Other algorithms: zoom-in/out, iris, diamond, diagonal, blinds, checker, pixelate, radial, ripple, swirl, glitch, burn, noise-dissolve, displacement, chromatic, blur-cross, spin, shutter, split, luma, circle-morph, bars, venetian, door, tilt, page-turn, exposure, light-sweep, ink, cells, hexagon, wave, arc, soft-wipe, center-wipe, lens, prism, halftone, film-roll.

Each transition file stores `mode` (0–47), direction (two components), strength, softness, tiles and ease. Eases: linear, inOutCubic, outExpo, inOutQuint, swift, settle, spring. Scene `transition_duration` controls overlap; the transition's ease controls progress. Exact phase 0 and 1 return the original endpoints. Use restrained crossfades/zoom/focus changes for keynote work; noise, glitch and burn should serve a game or character beat.

Generate a full motion sampler as another ordinary editable data project:

```powershell
video-pv-cli catalog showcase --out transition-reel
video-pv-cli setup --project transition-reel
video-pv-cli render --project transition-reel
```

## Maintenance and Windows validation

Edit one preset's JSON for a preset change; edit the shared renderer only for layout/algorithm changes. `tools/cataloggen` is an explicit Go seed utility and can overwrite a destination; export to a new scratch directory and review the diff before adopting it. Never run it automatically during rendering or installation.

```powershell
go test -count=1 ./...
go vet ./...
video-pv-cli catalog validate
video-pv-cli catalog verify --runtime-project my-pv --mode deep --batch-size 64 --out work/catalog-validation
```

Deep mode samples every scene at three times, every transition at five phases, and every declared loop at its start/end: 2,140 cases for this catalog. Batches are 1280×720 at 6 fps so every case becomes one decoded MP4 frame. Every batch runs lint, profile checks, reordered/reloaded/multiworker/range determinism, stills, MP4 and full decode. Pixel gates check dimensions, opacity, suspicious blank scenes, common transition endpoints and loop endpoint MAE. This samples scene coverage; it does not render all 128 complete films at production FPS or certify creative quality.

The evidence directory contains the aggregate report, exact case data, source hashes, per-batch logs, contact sheets, PNG hashes and MP4 files. `--limit` or `--category` marks evidence limited. `smoke` samples three times per template. Use complete production-FPS renders for representative films, fast motion, audio and final delivery. Do not create GitHub Actions for this workflow.
