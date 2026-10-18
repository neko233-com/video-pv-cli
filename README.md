# web2video

Go 1.27 纯 CLI：使用真实 [HyperFrames](https://github.com/heygen-com/hyperframes) 引擎，将 HTML / GSAP / 原生 WebGPU 合成为经过完整解码验证的 MP4。

面向产品发布会、二次元角色 PV、游戏概念宣传片和游戏官网循环背景。项目已从旧 skill 全面重构；不安装 skill，不依赖 Unity，不使用 GitHub Actions。

## 开始

需要 Go 1.27、Node.js >=24、支持硬件 WebGPU 的显卡和驱动。首次 setup 在当前用户缓存安装固定版本 HyperFrames 0.8.171、GSAP 3.14.2、渲染浏览器和 FFmpeg。

```powershell
go install github.com/neko233-com/web2video@latest
web2video setup
web2video init --dir my-film --template keynote-product
web2video preview --project my-film
```

依赖被清理或损坏时执行 `web2video setup --repair`，按锁文件重新安装并应用经过哈希校验的兼容修正。

在另一个终端导出：

```powershell
web2video render --project my-film
web2video verify --input my-film/renders/master.mp4 --fast-start
```

使用 preview 输出的 HTTP 地址。**不能直接双击 index.html / 使用 file://**；原生 HyperFrames 运行时和模块资源需要 HTTP 服务。

## 可编辑项目

编辑 `project.json` 中的文案、镜头时长、图片路径、裁切、缩放与转场，再运行 `web2video build --project my-film`。build 会重新生成 index.html、运行时和本地依赖文件。直接编写任意原生 HyperFrames HTML 时，可跳过 build，使用 render/lint/check 等命令。

- 4 个独立案例：`demo/keynote-product`、`demo/anime-character`、`demo/game-trailer`、`demo/website-background`。
- 100 个风格数据文件：`assets/styles/{id}.json`，包含关键词、导演方向及配色/字体参数。
- 76 个独立 WGSL 转场参数文件，使用 48 种原生算法。
- Windows WebGPU 编译器兼容处理使用 SHA-256 守卫；没有 WebGL2 渲染回退。
- MP4 交付给游戏引擎即可，不包含引擎集成。

```powershell
web2video templates
web2video styles
web2video styles --show anime-celestial-heroine
web2video init --dir character-film --template anime-character --style anime-celestial-heroine
web2video lint --project character-film
web2video check --project character-film --samples 9 --at-transitions
web2video snapshot --project character-film --at 1,4.5,7.5,10.5
web2video timeline --project character-film
web2video catalog --help
```

样例是原创生成静态美术的概念片，示范排版、镜头运动和转场；不代表实际玩法、角色骨骼动画或真实产品能力。100 个风格是可维护的方向配方，并不等于 100 部独立成片。

## 导出、幂等与验证

输出默认 `renders/master.mp4`。首次导出先写临时文件，校验通过后发布。匹配输入与收据时，重复 render 重新完整验证已有 MP4 并复用；输入变化时需换输出名或显式 `--force`。输出只允许放在项目 renders/ 内，避免覆盖源文件。

验证包括 MP4 box 边界/品牌、fast-start、H.264/yuv420p、音轨策略、帧数、单调时间戳、恒定帧距、尺寸/FPS/时长、完整 FFmpeg 解码及验证前后文件哈希。

```powershell
go test -count=1 ./...
web2video qa --media --gpu --catalog
```

`qa` 在本仓库根目录运行源码测试；独立视频项目使用 `check` 和 `verify`。

测试夹具使用临时目录自动清理。快照固定写入 `.web2video/snapshots`；渲染临时目录、浏览器 profile、历史记录在任务结束后清理。缓存收据和最终输出保留复用。**所有层级、大小写的 MP4 都被 Git 忽略；不会提交或放进发行包。**

`--gpu` 执行真实 WebGPU 导出、像素检查、复用和损坏 Shader 反例；`--catalog` 对 100 个风格和 76 个转场逐项截图，并检查 0 秒/循环终点的 PNG 完全一致。需要完整的浏览器资源；setup 会检测并修复缺失的 ICU、图形库和资源包。

网站背景接入见 `demo/website-background/embed.html`：muted + autoplay + loop + playsinline，pointer-events:none、inert、不可聚焦，正文链接保持交互，并考虑减少动态效果偏好。

[架构与限制](docs/architecture.md) · [案例说明](demo/README.md) · [许可与依赖](NOTICE)

Windows 实测证据见 [验证记录](docs/windows-validation.md)。本地打包发行：`go run ./scripts/release`，只包含 CLI、许可和说明，并生成 SHA-256 校验文件。
