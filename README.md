# video-pv-cli

Go 1.27 CLI 与配套 agent skill，用 HTML / WebGL / GLSL 制作游戏、二次元角色、产品与游戏官网主视觉 PV，再导出 MP4 并自动验收。

```sh
go install github.com/neko233-com/video-pv-cli@latest
video-pv-cli init --dir my-pv --type game
video-pv-cli setup --project my-pv
video-pv-cli render --project my-pv
video-pv-cli skill install
```

官网 Shader 循环示例：`init --type game`；二次元角色关键视觉示例：`init --type character`。后者包含原创生成的角色立绘、三个叙事节拍、图片纹理采样与能力光效，是 key-art motion 示例；角色表演与实际玩法需要接入经审片的动画/引擎/录制素材。

![角色 PV 的实际导出帧](https://raw.githubusercontent.com/neko233-com/video-pv-cli/main/docs/character-poster.png)

[角色 MP4 示例](https://github.com/neko233-com/video-pv-cli/blob/main/docs/character-demo.mp4) · [官网 Shader MP4 示例](https://github.com/neko233-com/video-pv-cli/blob/main/docs/shader-demo.mp4)

Node.js 24+、Multiplane 和 Chromium 用于渲染；Go CLI 调用项目本地锁定的 renderer，不通过 npx 隐式下载。Windows x64 可自动准备用户缓存中的 FFmpeg；macOS/Linux 需要 PATH 中可用的 FFmpeg（含 libx264）。

`render` 依次执行 manifest 检查、HTML lint、规格核对、确定性验证、静帧联系表、MP4 导出与完整解码；`check --release` 额外要求资产、权利与审片记录。实际画面质量、角色一致性与剪辑节奏需要审片。

```sh
video-pv-cli check --manifest my-pv/pv-project.json --media my-pv/deliverables/master.mp4 --decode
video-pv-cli check --manifest my-pv/pv-project.json --release --report my-pv/qa/release.json
video-pv-cli doctor --project my-pv
video-pv-cli skill pack --out pv-production.zip
```

首次安装 skill 默认写入 `CODEX_HOME/skills/pv-production`，未设环境变量时为 `~/.codex/skills/pv-production`。`--dir` 可选择路径；`--force` 会保存一个隐藏备份再替换已有同名 skill，不覆盖无关目录。安装包同时带 Go 源码和模板，可直接从 skill 目录运行 `go run .`。无需把整个仓库复制到用户缓存。

| 命令 | 常用选项 |
| --- | --- |
| `init` | `--dir` 新目录；`--type game/character/product`；`--title` |
| `setup` | `--project`；`--skip-browser`；`--timeout` |
| `render` | `--project`；`--shot`；`--manifest`；`--out`；`--audio`；`--workers`；`--raster gpu/software`；`--quality`；`--motion-blur`；`--draft`；`--timeout` |
| `check` | `--manifest`；`--media`；`--decode`；`--release`；`--json`；`--report`；`--auto-install=false` |
| `doctor` | `--project`；`--tools-dir` |
| `skill install / pack` | `--dir` / `--out` |

`render` 输出路径相对项目目录，默认 `delivery.video_path`；已存在 MP4 会被拒绝，请为新版本选择新路径。JSON 总报告默认 `qa/render-report.json`，包含 renderer 事件、输入哈希和媒体检查；各次联系表与逐步日志位于 `renders/jobs/`。音轨需先在 manifest 声明为资产；示例默认静音，适合官网主视觉。

Shader 选择与许可证见 [shader-production.md](references/shader-production.md)。原生 WebGL2、GLSL 本地 include 与图片纹理在模板中实际执行；Paper Shaders、Three.js 和 ShaderGradient 提供接入规范，新增库需要通过同样的确定性和 MP4 验证。

源代码构建：`go build -o video-pv-cli.exe .`；测试：`go test ./...`。

完整渲染集成测试：先用 `init`、`setup` 准备项目，再设置环境变量 `PV_INTEGRATION_PROJECT` 为该目录，执行 `go test -v -run TestIntegrationShaderMP4AndFailureGates`。测试会生成音频测试信号，验证 MP4 像素格式/帧数/解码，并确认损坏 Shader、非确定动画、未批准发布均被拦截。验证先在 Windows 本地执行，不使用 GitHub Actions。

游戏官网中使用导出文件时，采用 `muted autoplay loop playsinline` 的视频和独立 poster；减少动态效果模式显示静态 poster。循环边界、手机裁切和网站文字遮罩仍需按实际站点验收。

Skill 入口：[SKILL.md](SKILL.md)。Shader 制作与验收、游戏与角色导演参考位于 `references/`。内置 `Stellar Echo` 是原创 procedural shader 输出示例，不代表完整游戏宣传片或实际玩法。默认状态为 draft，不伪造人工批准。

CLI 与原创模板：Apache-2.0。Multiplane、FFmpeg、字体与可选 shader 库各自遵循其许可证；模板字体授权见 `assets/starter/FONT-OFL.txt`。
