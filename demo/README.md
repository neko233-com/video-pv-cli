# Independent examples

Run `web2video setup` first. Each directory is a complete editable native
HyperFrames project with local assets; see its README.

| Directory | Purpose | Duration |
| --- | --- | --- |
| `keynote-product` | Fictional AURA handheld concept film | 12 seconds |
| `anime-character` | Original ASTERIA character introduction | 12 seconds |
| `game-trailer` | STELLAR ECHO original world concept trailer | 12 seconds |
| `website-background` | Silent, periodic website atmosphere | 6 seconds |

```powershell
web2video preview --project demo/anime-character
web2video render --project demo/anime-character
```

Keep the preview server running and use its HTTP URL. Direct file opening is
unsupported. Edit `project.json` and rebuild with `web2video build --project ...`.
The generated `renders/master.mp4` is ignored, not committed. These use generated
still artwork and camera motion, not gameplay capture or skeletal animation.

`website-background/embed.html` demonstrates noninteractive muted inline looping
video behind interactive website content. Serve it over HTTP after rendering.
