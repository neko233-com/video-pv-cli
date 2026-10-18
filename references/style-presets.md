# 100 data-driven style presets

Each `assets/styles/<id>.json` is an independently editable original recipe. Ten families have ten recipes each: keynote, luxury, tech, editorial, game, anime, cinema, organic, graphic and digital. These are directions and executable design tokens, not 100 finished films or promises of photoreal materials.

All 100 have explicit individual palettes. Tests check foreground contrast against base/surface colors and named light/dark directions. In `preview`, open `/gallery` for a searchable index of verified GPU start frames; each card opens its actual style in the workbench. The repository-only `tools/stylegallery` rebuilds it from a passed evidence directory, validates PNG hashes and confines paths, then atomically replaces the same files. The index is illustrative sampling, not a substitute for full-motion review.

```powershell
video-pv-cli style list --family keynote
video-pv-cli style list --query "rim light" --json
video-pv-cli style show --id anime-celestial-heroine
video-pv-cli init --dir character --style anime-celestial-heroine
video-pv-cli style apply --project character --id anime-samurai-ink
video-pv-cli style validate
video-pv-cli style verify --mode deep --batch-size 32
```

`init --style` chooses the style's base sequence unless `--template` is explicit. `style apply` preserves copy, images, scene types, scene durations, assets and claims. It changes theme, backgrounds, layout, motion, easing, overlap and transitions; recalculates the timeline; clears approvals; and keeps fixed first-application backups. Applying the same style twice changes no files. Use `--template website-atmosphere-midnight --background-only` for a styled website loop. A solid first background selects the recipe's first procedural background for a visible periodic field.

## Recipe contract

| Field | Purpose |
| --- | --- |
| `keywords`, `avoid`, `best_for` | Searchable art direction and constraints |
| `direction` | Composition, typography, lighting, materials, camera, editing, sound |
| `beat_plan` | Hook, establishment, truthful proof, identity payoff |
| `reference_ids` | Workflow research provenance, resolved in the framework reference |
| `render.base_template` | Starting sequence, independently overridable |
| `render.theme` | Five colors and local font family |
| `render.backgrounds`, `layouts` | Cycling procedural fields and scene alignment |
| `render.motion`, `transitions`, `overlap_ratio` | Executable motion and edit choices |
| `render.typography` | Scale, weight offset, tracking, leading |
| `render.graphics` | Panel radius and optional frame/grid/contour/print overlays |

The native WebGPU compositor renders the resulting local textures and WGSL fields. Canvas 2D prepares typography and graphic textures. Lighting/material/camera/sound prose guides asset production and editing; it does not synthesize arbitrary 3D scenes or audio. A watch macro needs a real watch render/capture; an anime action shot needs approved character imagery or animation. Custom WGSL can extend the field or effect library, with compilation and deterministic checks.

## Use as a director

1. Choose a family matching the audience and emotional target. Inspect at most a few strong recipes before choosing; do not blend every effect.
2. Create `frame.md`: aspect/safe areas, palette roles, font licenses and local files, hierarchy, spacing, material references, lighting, motion curves, cut density, audio beats, identity invariants and prohibited imagery.
3. Prepare shot cards with in/out time, purpose, focal point, real/generated status, asset parents, motion, transition motivation, copy, sound cue and acceptance criteria. Open with a readable hook, then actual proof and a clear payoff.
4. Test representative wide/detail/type/proof frames. Inspect final-size type and subject identity. Revise recipe tokens before producing the full film.
5. Generate/export, inspect contact sheets, play the complete MP4 and run technical verification. Record what was actually checked. Automated pass flags do not grade aesthetics.

## Maintenance and validation

Strict JSON rejects unknown fields, duplicate keys, unsupported tokens, missing bases and out-of-range values. Tests require exactly 100 IDs, 10 per family, all 12,800 style/template applications, preserved evidence bindings and unique render recipes after removing palette/base-template identity. The repository-only authoring helper `tools/stylegen` refuses a nonempty output directory; runtime never rewrites the library. Hand-edit one JSON when maintaining a preset.

Deep style QA exports 500 actual frames: three times per recipe plus each recipe applied to a background-only periodic template at start/end. Full catalog QA includes these 500 cases. Edge adds start/middle/end/start for all 100 styles, checking actual native GPU pixels and repeatability. Neither sampling set claims every frame of every possible customized film; a production `render` verifies every output frame by default.
