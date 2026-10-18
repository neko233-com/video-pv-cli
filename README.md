# video-pv-cli

Go 1.27 CLI + `pv-production` agent skill：制作游戏宣传片、二次元角色 PV、发布会风格产品 PV 与游戏官网主视觉，通过 **HTML + 原生 WebGPU / WGSL** 导出并验证 MP4。

[四个独立案例源码](demo/README.md) · [100 风格画廊](docs/style-gallery/index.html) · [Windows 实测记录](docs/windows-validation.md)

案例按 `demo/{案例名字}/` 维护：产品发布会、二次元角色、游戏宣传片、官网循环背景。每个目录都包含完整可编辑源码与运行说明。**导出的 MP4 不提交仓库，也不进入 skill 压缩包**；渲染缓存和依赖同样忽略。

## 数据化模板库

- **100 个独立风格预设**：发布会、奢华、科技、编辑、游戏、二次元、电影、自然、平面和数字产品，每类 10 个。包含关键词、构图/字体/光材/镜头/剪辑/声音方向，以及可执行的字体、图形、背景、运动、转场参数。[风格手册](references/style-presets.md) · [HyperFrames 等研究](references/web-video-frameworks.md)。

- **128 套独立模板 JSON**：16 种编排结构 × 8 套配色，每个预设单独维护。
- **76 套独立转场 JSON**：48 种 WGSL 算法，包含八方向滑入/推移/覆盖/揭示。
- **33 种场景布局、12 种程序化背景**：产品揭幕、巨型文字、Bento、材质特写、影像长廊、游戏战斗节拍、角色揭示、官网循环等。
- **只使用 WebGPU**，记录实际适配器与 WGSL 后端；设备不可用、Shader 错误、设备丢失会阻止导出。
- **Windows 本地验证，不使用 GitHub Actions**。

运行 `preview` 后打开 `/gallery` 可按分类/关键词浏览 100 张实测缩略图，并直接进入对应风格的 WebGPU 验证台。索引可以从验证缓存幂等重建：`go run ./tools/stylegallery --evidence work/catalog-validation --out docs/style-gallery`。

完整目录和数据维护方式见 [模板手册](references/template-catalog.md)。128 套是结构与视觉变体，不代表 128 部独立导演概念。发布会风格示例建议 `keynote-product-porcelain`；动漫角色建议 `character-reveal-midnight`。

## 开始制作

```powershell
go install github.com/neko233-com/video-pv-cli@latest
video-pv-cli style list --family keynote
video-pv-cli catalog list
video-pv-cli init --dir my-pv --style keynote-silent-reveal --title AURA
video-pv-cli setup --shared
video-pv-cli render --project my-pv
video-pv-cli skill install
```

编辑 `my-pv/template.json` 的文案、图片、节奏、主题与转场。改变时长/分辨率/FPS/图片引用后，执行 `catalog sync --project my-pv` 同步清单；它保留旧清单备份、重建 draft 镜头并清除审片批准。已有 MP4 不覆盖，请用新文件名导出。

| 文件 | 内容 |
| --- | --- |
| `assets/styles/<id>.json` | 每个风格独立方向与渲染配方 |
| `assets/catalog/templates/<id>.json` | 每个模板独立文件 |
| `assets/catalog/transitions/<id>.json` | 每套转场独立参数文件 |
| 项目 `template.json` / `transitions.json` | 可直接编辑的项目数据 |
| `catalog-runtime.js` | 共用场景编排与绘制器 |
| `catalog.wgsl` / `library.wgsl` | 原生 GPU 转场和效果库 |
| `webgpu-runtime.js` | 设备、管线、纹理、Uniform 与错误处理 |
| `pv-project.json` | 交付规格、素材/声明、镜头与批准记录 |

产品模型是原创程序化示意，角色是原创生成的样例立绘。实际游戏、角色表演和产品素材需自行接入并审片；样例不代表实际玩法或产品能力。Manrope/Cinzel 为本地字体；中文/日文需要另行提供已授权的本地字体并在 setup 阶段加载。

`setup --shared` 将锁定依赖安装到当前用户缓存；重复执行复用现有依赖，`--refresh` 可主动重装。`render`、`preview`、`catalog verify`、`doctor` 优先使用本地已准备依赖，否则使用此缓存；显式指定依赖路径时不会换用其他目录。仍可用 `setup --project <项目>` 准备独立环境。

## 转场与导出

```powershell
video-pv-cli catalog list --kind transitions
video-pv-cli catalog show --id light-sweep
video-pv-cli catalog showcase --out transition-reel
video-pv-cli setup --project transition-reel
video-pv-cli render --project transition-reel
video-pv-cli check --manifest my-pv/pv-project.json --decode
```

`render` 自动执行数据/资产预检、lint、规格与 WebGPU 核对、逐帧确定性检查、联系表、H.264/yuv420p MP4 编码和完整解码。默认检查每个输出帧时刻，按 128 帧分组；`--verify-at` 可显式缩小采样范围，报告会注明覆盖程度。CLI 对锁定的 Multiplane 应用 SHA-256 校验的队列同步补丁，在截图前等待 WebGPU 完成。默认报告为 `qa/render-report.json`，逐步日志/静帧在 `renders/jobs/`。音轨用 `--audio soundtrack.wav`，须先声明为资产。`--release` 还要求真实的资产权利、镜头批准和人工审片记录。

渲染需要 Node.js 24+、锁定的 Multiplane 0.3.10、Chromium 与可用 WebGPU。Windows x64 缺少 FFmpeg 时可校验下载到当前用户缓存，不改系统 PATH。`--auto-install=false` 禁止下载。macOS/Linux 尚未作为本次验证目标；需自行准备 FFmpeg 与 WebGPU 环境。

## 网站背景与游戏引擎

```powershell
video-pv-cli init --dir hero-loop --template website-atmosphere-midnight --background-only
video-pv-cli render --project hero-loop
video-pv-cli bundle --project hero-loop --target website
video-pv-cli preview --bundle hero-loop/deliverables/website
video-pv-cli bundle --project character-pv --target mp4
video-pv-cli mp4 verify --input character-pv/deliverables/mp4/video.mp4 --fast-start
```

`--background-only` 清除烘焙进视频的标题，保留网页可访问的前景文案。网站包包括 MP4、海报、HTML 与哈希报告：自动静音循环、移动端内联、无控件、`pointer-events:none`、`inert`、不进入键盘焦点；减少动态效果偏好使用海报，标签隐藏时暂停。只有周期型官网模板可打成循环背景包。

`preview` 默认绑定 `127.0.0.1:7335`。主页提供 100 风格、模板/转场选择、时间轴、播放、保存画面及全库验证。打开 `/delivery-audit` 可观察两次实际循环、检查点击/焦点/海报并替换写入报告。

游戏引擎只接收 `video.mp4`、海报与验证报告，不生成 Unity 脚本或依赖引擎 SDK。独立 `mp4 verify` 检查 MP4 容器、H.264/yuv420p、可选 AAC、帧数、关键帧、时间戳、恒定帧间隔、音画时长和完整解码；`--silent`、`--require-audio`、`--fast-start` 可追加约束。[交付说明](references/delivery-targets.md)。

## 本地自动化验证

一键运行 Windows 本地测试、两轮真实 MP4 夹具、GPU 故障门禁和样片检查：

```powershell
go run . qa windows --deep
# 先按 demo/README.md 本地导出后，可追加已有样片检查：
go run . qa windows --samples demo/keynote-product/deliverables/master.mp4,demo/anime-character/deliverables/master.mp4
```

`--deep` 还执行或复验全库，复用固定 `work/catalog-validation`；用 `--catalog-out` 指定同一份缓存。`--gpu=false` 可只跑媒体/数据测试，报告会明确 GPU 未启用和跳过数量。日志与报告替换写入 `work/windows-qa`，输出目录必须有本工具的所有权标记。需先 `setup --shared`；不会启用 GitHub Actions。

```powershell
go test -count=1 ./...
go vet ./...
go run . style validate
go run . catalog validate
go run . catalog verify --mode deep --batch-size 32
go run . preview
```

Deep 模式包含 **2,640 个案例**：每个场景三个时刻、所有转场五个阶段、所有循环首尾，以及 100 风格的 300 个关键帧与 100 对周期背景接缝。每批以 720p/6fps 将每个案例写成真实 MP4 帧，执行重排/重载/多 worker/区间验证、静帧像素检查、转场端点与循环接缝比较、完整解码。`--limit` 或类别过滤会标记为有限覆盖。它不等于将 128 部完整影片全部按最终帧率渲染，也不替代审美审片。

完整集成测试（会产生真实视频和测试音频）：

```powershell
$env:PV_INTEGRATION_RUNTIME = Join-Path $env:LOCALAPPDATA 'video-pv-cli/runtime/multiplane-0.3.10'
$env:PV_INTEGRATION_RASTER = 'gpu'
$env:PV_MEDIA_INTEGRATION = '1'
go test -timeout 25m -v -count=2 -run 'TestIntegrationShaderMP4AndFailureGates|TestIntegrationBundle' .
```

先完成 setup --shared，测试仅借用其依赖；集成测试在 `t.TempDir()` 中创建 1 秒夹具，覆盖 MP4/AAC、帧数、像素格式、解码、损坏 WGSL、非确定动画、WebGPU 缺失/设备丢失和未批准发布。网站/MP4 打包测试连续比较文件集合与哈希、验证 Range 请求和损坏修复。还会验证 100 风格 × 128 模板的 12,800 个数据组合，以及安装/ZIP 的重复一致性。测试退出自动清理夹具。Canvas 2D 仅准备字体与图片纹理；最终 GPU 合成只用 WebGPU。`--raster software` 是浏览器光栅策略，不是另一个 GPU API。

### 幂等与留存

- Go 单元/集成测试均使用临时目录；重复运行不改输入项目、不留下测试 MP4 或音轨。
- `catalog verify` 默认固定 `work/catalog-validation` 并续跑；已完成批次重新校验源码/PNG 哈希和实际 MP4 解码，匹配后复用。源码或采样变化时拒绝复用；失败尝试保存在同一证据目录。
- `preview` 固定 `work/browser-validation`，报告与当前画面原位替换；只接受同源写入。
- `bundle` 固定 `deliverables/website` 或 `deliverables/mp4`，输入/配方/输出哈希匹配时复用，暂存自动清理；拒绝非本工具目录，保留未知用户文件。
- `render` 是创作导出，保留 MP4、日志和联系表，拒绝覆盖已有 MP4；自动化验证请复用固定缓存。用 `--renderer-project` 共享已准备依赖，避免每套模板重复安装。

本次 Windows 实测范围见 [验证记录](docs/windows-validation.md)。

## 安装 skill

`video-pv-cli skill install` 默认安装到 `CODEX_HOME/skills/pv-production`，未设置时使用 `~/.codex/skills/pv-production`。可用 `--dir` 指定位置；文件相同时直接复用并保留自定义文件，不新增备份。文件不同时用 `--force` 先保存隐藏备份。安装包包含 Go 源码和全部模板/风格数据，进入 skill 目录即可 `go run . <command>`。

```powershell
go build -trimpath -o dist/video-pv-cli.exe .
video-pv-cli skill pack --out pv-production.zip
```

入口：[SKILL.md](SKILL.md) · [WebGPU Shader 制作](references/shader-production.md) · [HTML 制作](references/html-production.md) · [导演参考](references/cinematic-direction.md) · [交付验证](references/validation.md)。

CLI、原创模板和 WGSL：Apache-2.0；字体、生成素材、Multiplane、FFmpeg 分别遵循自己的条款，见 [NOTICE](NOTICE)。
