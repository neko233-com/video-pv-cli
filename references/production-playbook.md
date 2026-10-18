# PV production playbook

Use this as a decision aid. Keep the creative direction specific to the brief and let actual source assets, audience, and distribution format determine the cut.

## Pick the primary promise

Choose one primary deliverable before drafting the edit:

| PV type | What the audience should understand | Useful evidence or material |
| --- | --- | --- |
| Game announcement / teaser | The fantasy, world, tone, and reason to care | Approved concept art, cinematic capture, an honest development-status label |
| Game gameplay / launch | What the player does, how the loop works, and what is available | Capture from the named build, with version and capture date logged |
| Update / DLC | What changed and why an existing player should return | Before/after or new-content capture from the correct build; verified release details |
| Character PV | Who the character is, what they want, and their defining action or feeling | Approved character sheet, voice/persona rules, expression and costume anchors |
| Product launch / demo | Which need it serves and how the product demonstrably helps | Real product capture, approved specifications, observed results, and approved brand assets |
| Social cutdown | One idea, one hook, one clear next action | A safe-area-aware edit derived from an approved master |

If the brief asks for several audiences or platforms, define one master promise and plan distinct cutdowns instead of forcing every message into the same opening.

## Intake and evidence

Create a short brief with: working title; product/game/version; audience; desired response; deliverable and destination; duration, aspect ratio, resolution, frame rate, and language; CTA; factual claims; source assets; rights/consent and attribution; budget or generation limit; review owner; and known restrictions.

Maintain a claim ledger. For each gameplay, feature, performance, compatibility, availability, price, customer, award, or results claim, record exact wording, evidence asset or source, build/version and date where relevant, and approval state. If evidence is absent, recast it as clearly framed aspiration or remove it. Do not infer performance or feature availability from art, a roadmap, or a generated shot.

For mixed footage, label each shot in the shot list as `captured gameplay`, `captured product`, `engine cinematic`, `designed motion`, `generated`, or another clear source class. Do not intercut generated images into a gameplay proof sequence in a way that implies they are captured gameplay.

## Direction and edit rhythm

Write a one-line promise before the script. Give each beat one job: hook, context, reveal, proof, escalation, emotional turn, payoff, or CTA. A short PV usually benefits from a strong image or action immediately, a small number of legible beats, and an ending with enough time to read the title or CTA. Allocate time to the message and action; do not fill runtime with repeated logo cards.

For every shot, specify:

- beat and audience question;
- in/out time and intended transition;
- subject, action, framing, camera motion, screen direction, and focal attention;
- source class and parent assets;
- VO, dialogue, music, SFX, on-screen copy, and captions;
- visual continuity anchors and what must not change;
- claims shown or spoken and their supporting evidence;
- acceptance notes that can be reviewed in the result.

Lock a continuity bible for recurring characters and products. Record identity anchors (face or silhouette, costume, proportions, materials, logo placement, colors, interface labels), approved variants, location and prop state, screen direction, chronology, and intentional changes. Use approved reference images as parents for dependent shots. Check continuity throughout motion, not only on a poster frame.

## Type-specific direction

### Games

- Separate announcement, gameplay, launch, update, store-page, and social goals. A cinematic teaser may sell tone; a gameplay PV should demonstrate real player verbs, readable UI where relevant, systems, stakes, and agency.
- Plan capture from the correct build. Log build/version, platform, capture settings, HUD treatment, and whether footage is representative. Do not stage impossible play as ordinary gameplay. If a shot is a controlled or edited demonstration, make that clear in the production notes and disclose it when needed to avoid a misleading impression.
- Treat ratings, store requirements, platform marks, review quotes, release windows, multiplayer, performance, and availability as approval-sensitive facts. Verify current platform requirements from the platform's own guidance when the deliverable depends on them.
- Design a store-page video and a vertical cutdown as separate framing problems: protect UI, title, subtitles, and CTA from crops and platform overlays.

### Characters

- Direct a readable arc through action and expression: establish identity, show a signature behavior or ability, introduce a change or tension, then land on an emotional or iconic image.
- Keep face, silhouette, costume, accessory, palette, proportions, and voice/persona anchors in the continuity bible. Track changes between shots as intentional states.
- For dialogue, establish exact approved wording, speaker, language, performance, and captions. Voice cloning, likeness, or performance transfer requires documented authorization and a review of disclosure needs.

### Products

- Pick a use case and show the actual product or product behavior when demonstrating a capability. Use motion design to clarify hierarchy and frame evidence, not to fake a screen, customer outcome, metric, or feature.
- Capture product screens from a known version with deterministic steps where possible. Remove secrets, real personal data, and test artifacts from capture. Make explanatory graphics visibly distinct from captured interface.
- Verify specifications, availability, prices, and comparative language against an approved source and date. Use precise, observable benefit statements.

## Choose how each shot is made

Use real capture when the shot proves a feature, gameplay action, product result, or factual detail. Use designed motion for typography, diagrams, transitions, and controlled product framing. Use generated media for mood, backgrounds, or fictional coverage when it helps and can preserve the required identity. Prefer a small approved keyframe/pilot before a large generation batch. Choose per shot based on truth needs, identity constraints, motion, control, time, cost, and editability.

For exact text, CJK typography, product UI, animated diagrams, logo reveals, and layered 2.5D layouts, HTML/CSS/JS can be a more controllable source than asking an image/video model to render those details. Multiplane renders deterministic HTML shots to video and can verify repeated frames; use it as the designed-motion renderer, not as evidence for gameplay or observed product behavior. See [html-production.md](html-production.md) for its workflow and gates.

Before using a remote generation or rendering tool, verify its current schema, supported duration/aspect ratio/references, data retention, rights terms, and price from the connected tool or provider documentation. Log each job and preserve input/output relationships. Do not assume capabilities from an old model list or another endpoint.

## Audio, text, and variants

Treat music, sound effects, voices, and dialogue as separate assets with source, license/consent, language, and approval state. Keep copy in an editable post-production layer. Captions should match the spoken words and meaning, have adequate reading time, avoid unsafe crops, and remain legible over changing backgrounds. Review spelling, names, timing, punctuation, and localization in context.

Keep a clean master, project timeline, source captures, approved graphics, audio stems, and caption files. Produce variants from the same approved source timeline when practical, then review each complete export because reframing, timing, and safe areas change by aspect ratio.

## Reference projects

These public GitHub projects informed the workflow patterns here; this skill does not copy their code or assume their provider integrations:

- [calesthio/generative-media-skills: game-trailer-production](https://github.com/calesthio/generative-media-skills/tree/main/skills/production/content-formats/game-trailer-production) separates trailer goals, build facts, a truth ledger, capture planning, beat maps, accessibility, and generated b-roll boundaries.
- [SamurAIGPT/open-ai-video-agent: storyboard-to-video](https://github.com/SamurAIGPT/open-ai-video-agent/blob/main/agents/storyboard-to-video/SKILL.md) stabilizes continuity and shot plans, reviews keyframes before dependent animation, and tracks per-shot generation constraints and job lineage.
- [Afnanksalal/product-demo-video-skill](https://github.com/Afnanksalal/product-demo-video-skill) ties product claims to real proof and combines technical export checks with contact-sheet and visual review.
- [calesthio/generative-media-skills: media-qc-delivery](https://github.com/calesthio/generative-media-skills/tree/main/skills/production/governance-delivery/media-qc-delivery) treats technical conformance, captions/accessibility, provenance, and handoff as part of final delivery.
- [LBEILC/Multiplane](https://github.com/LBEILC/Multiplane) provides a deterministic HTML-to-video renderer with static linting, frame-by-frame determinism checks, stills/contact sheets, and MP4 output; use its renderer for authored motion and `video-pv-cli check` for project and release validation.
