# Web video frameworks and product PV skill research

Reviewed 2026-10-18. Sources are upstream project documentation, not a quality ranking. The style data and WGSL in this repository are original. No upstream skill instructions are installed as part of this research.

Repository HEADs recorded during review: [HyperFrames dc0e80e](https://github.com/heygen-com/hyperframes/tree/dc0e80e73af185cc892ea90d5a9f1412a055e9a1), [video-shotcraft 43bd349](https://github.com/Vincentwei1021/video-shotcraft/tree/43bd349a136651865ecfa6ea1cf012f6e957d304), [ai-product-video 55501b4](https://github.com/dlazy-ai/ai-product-video/tree/55501b4f2e746f46317bfd972255897e00662dc5), [product-demo-video-skill cf91071](https://github.com/Afnanksalal/product-demo-video-skill/tree/cf9107157a3f2c63b5b9a297c0659196f4485ef6).

| Source | Useful pattern adopted | Integration status |
| --- | --- | --- |
| [Multiplane](https://github.com/LBEILC/Multiplane) | HTML shot contract, seek/render, deterministic verification, stills and MP4 jobs | Actual pinned export backend: npm `multiplane@0.3.10`, SHA-guarded GPU queue synchronization bridge |
| [HyperFrames](https://github.com/heygen-com/hyperframes) | Agent-oriented composition workflow, seekable timeline, production stages | Research reference; no HyperFrames runtime or render compatibility claim |
| [HyperFrames creative skill](https://github.com/heygen-com/hyperframes/blob/main/skills/hyperframes-creative/SKILL.md) | `frame.md` as video-scale visual direction; explicit style/motion decisions | Adapted into our direction and style-selection workflow |
| [HyperFrames product launch skill](https://github.com/heygen-com/hyperframes/blob/main/skills/product-launch-video/SKILL.md) | Brief, capture, storyboard/script, audio and review gates | Adapted as shot-planning guidance |
| [video-shotcraft](https://github.com/Vincentwei1021/video-shotcraft) | Shot cards, truthful product capture, hook/proof/payoff, camera and sound beats | Research reference; no Remotion backend bundled |
| [ai-product-video](https://github.com/dlazy-ai/ai-product-video) | Product footage, shot recipes and sound-aware motion | Research reference; no additional runtime installed |
| [product-demo-video-skill](https://github.com/Afnanksalal/product-demo-video-skill) | Product narrative grounded in observed capabilities | Research reference |

Style `reference_ids` resolve respectively to `hyperframes-creative`, `hyperframes-product-launch`, `video-shotcraft` and `product-demo-video`. They identify workflow influences shared by the recipe library; they do not imply an upstream author published or endorsed an individual preset.

## Architecture decision

HTML is a useful authoring format for type, layered imagery, diagrams and controlled motion. Deterministic frame time makes seeking, reordering, screenshots and offline encoding reproducible. Framework choice alone does not make a film premium: hierarchy, approved assets, timing, lighting, restraint and sound need deliberate direction.

Retain the actual tested native WebGPU/WGSL renderer. General web-video framework examples can use different GPU APIs, animation libraries or browser assumptions; none establishes a WebGPU-only export guarantee here. Borrow the planning and quality gates while keeping the supported backend explicit. MP4 delivery is engine-independent: one H.264/yuv420p video stream, optional AAC, verified timestamps and full decode. No engine SDK or project is required.

When evaluating another backend, first prove native WebGPU, deterministic seek, font/media readiness, GPU completion before capture, codec/cadence/audio behavior, error propagation, path confinement and repeatable cleanup with an actual short MP4. Only then document it as supported.
