# PV 案例源码

每个 `demo/{案例名字}/` 都是独立数据项目，可复制到新目录后修改。包含 HTML、场景/转场 JSON、WGSL、运行时、字体与锁定依赖清单；不需要其他案例的文件。

| 案例 | 内容 | 用途 |
| --- | --- | --- |
| [keynote-product](keynote-product/README.md) | AURA · 静场产品揭幕 | 发布会产品 PV |
| [anime-character](anime-character/README.md) | ASTERIA · 星辉角色 | 二次元角色介绍 MP4 |
| [game-trailer](game-trailer/README.md) | STELLAR ECHO · 奇幻概念预告 | 游戏宣传片编排示意 |
| [website-background](website-background/README.md) | 水下光纹 · 六秒循环 | 无烘焙文字、不可点击的网站背景 |

## 在仓库根目录运行

```powershell
go run . setup --shared
go run . render --project demo/anime-character
go run . mp4 verify --input demo/anime-character/deliverables/master.mp4 --silent
```

修改各项目 `template.json` 的素材、文案、时长和镜头；改变规格后运行 `catalog sync --project demo/案例名字`。默认逐个输出帧验证确定性，再编码并完整解码 MP4。已有导出不会被覆盖；可先验证已有文件，重新创作时通过 `render --out deliverables/new-version.mp4` 保留旧版本。

**导出的 MP4 不提交 Git，也不进入 skill ZIP**。`deliverables/`、`renders/`、`qa/`、`node_modules/` 均忽略。自动化夹具使用临时目录；全库 QA 使用固定缓存；报告替换写入，不累计时间戳报告目录。

这些案例为 draft 设计示意，不包含配乐，不声明真实游戏玩法或商业产品能力。角色立绘来源见各项目 `ASSET-SOURCES.md`；正式发布必须替换/确认素材权利、事实证据和人工审片批准。中文/日文请添加已授权的本地字体。
