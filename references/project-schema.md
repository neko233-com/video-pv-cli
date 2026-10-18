# PV project manifest

`video-pv-cli check` reads one UTF-8 JSON file named `pv-project.json`. Paths are relative to the manifest's directory. Keep the manifest, media, captions, source captures, and reports inside a project folder where practical.

## Example

```json
{
  "schema_version": "1.0",
  "project": {
    "title": "Example Game PV",
    "type": "game",
    "duration_seconds": 30,
    "width": 1920,
    "height": 1080,
    "fps": 30,
    "aspect_ratio": "16:9",
    "platforms": ["store-page"],
    "language": "en",
    "audio_required": true,
    "captions_required": true
  },
  "assets": [
    {
      "id": "capture-01",
      "path": "assets/capture-01.mp4",
      "role": "gameplay-capture",
      "source": "Captured from build 0.8.4 on 2026-10-18",
      "rights_status": "cleared",
      "status": "approved",
      "attribution": ""
    }
  ],
  "claims": [
    {
      "id": "claim-coop",
      "text": "Play the campaign with up to three friends",
      "evidence_asset_ids": ["capture-01"]
    }
  ],
  "shots": [
    {
      "id": "shot-01",
      "start_seconds": 0,
      "end_seconds": 8,
      "beat": "Hook: the team enters the arena",
      "role": "hook",
      "visual_description": "Captured four-player gameplay, wide framing",
      "asset_ids": ["capture-01"],
      "claim_ids": ["claim-coop"],
      "status": "approved"
    },
    {
      "id": "shot-02",
      "start_seconds": 8,
      "end_seconds": 30,
      "beat": "Escalation, payoff, and CTA",
      "role": "payoff",
      "visual_description": "Approved gameplay and end card",
      "asset_ids": ["capture-01"],
      "claim_ids": [],
      "status": "approved"
    }
  ],
  "delivery": {
    "video_path": "deliverables/master.mp4",
    "container": "mp4",
    "video_codec": "h264",
    "audio_codec": "aac"
  },
  "review": {
    "creative_approved": true,
    "reviewer": "Producer name",
    "approved_at": "2026-10-18T12:00:00Z"
  }
}
```

The example is a shape guide, not a factual game claim or cleared asset. Replace every value with project evidence and actual review state.

## Fields

- `schema_version`: currently `1.0`.
- `project.type`: `game`, `character`, or `product`.
- `project.duration_seconds`, `width`, `height`, `fps`: final master target. If `aspect_ratio` is set, it must be a ratio such as `16:9` and agree with dimensions within tolerance.
- `project.platforms`, `language`: intended destinations and primary language.
- `audio_required`, `captions_required`: release requirements.
- `assets`: stable IDs and source metadata. `path` may be empty for a planned asset; a populated path must resolve to a file in the project directory. `role` examples: `gameplay-capture`, `product-proof`, `character-reference`, `keyframe`, `designed-motion-source` (such as an HTML shot), `music`, `voice`, `caption`, `logo`, `reference`, `other`. Use `rights_status` `cleared`, `pending`, `restricted`, or `unknown`; use `status` `planned`, `draft`, `approved`, or `rejected`.
- `claims`: exact claim copy and the IDs of evidence assets. Attach claim IDs to the shots that communicate them. The validator checks that the shot uses at least one listed evidence asset.
- `shots`: ordered timeline intervals, story purpose, source asset IDs, associated claims, and approval status. Record a distinct row for every visually distinct shot or title-card beat.
- `delivery`: `video_path` to the intended export, plus optional expected `container`, `video_codec`, and `audio_codec` such as `mp4`, `h264`, and `aac`.
- `delivery.pixel_format`: optional expected pixel format; use `yuv420p` for the bundled H.264 MP4 delivery profile.
- `review`: a human review attestation. `video-pv-cli check --release` requires an approved status, reviewer, and RFC 3339 timestamp; the CLI cannot establish who performed this review.

Only the documented fields are accepted. This makes typos visible instead of silently dropping them. See the checked-in [example plan](../examples/pv-project.json) for a draft that can be validated before media exists. For separate masters whose delivery dimensions or runtime differ, keep a manifest per variant so each export has its own explicit target.
