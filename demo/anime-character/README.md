# ASTERIA · 星辉角色

1920×1080 / 30fps，约 10.6 秒。`anime-celestial-heroine` 风格：星云背景、银蓝轮廓、身份揭示、能力题签、角色收尾。使用原创生成的静态立绘与程序化镜头运动，演示角色介绍视频的编排；不是角色骨骼动画。

在仓库根目录运行：

```powershell
go run . setup --shared
go run . render --project demo/anime-character
go run . bundle --project demo/anime-character --target mp4
go run . mp4 verify --input demo/anime-character/deliverables/mp4/video.mp4 --silent --fast-start
```

游戏引擎接收普通 MP4，无 Unity 依赖。替换 `astral-knight.png` 和 `template.json` 可制作自己的角色 PV；正式发布需确认权利与人工审片。MP4 不提交。
