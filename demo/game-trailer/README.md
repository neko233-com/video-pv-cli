# STELLAR ECHO · 奇幻概念预告

1920×1080 / 30fps，约 11.4 秒。`game-epic-fantasy` 风格：暖金暗场、章节节拍、动作面板、转场冲击与片名落版。示例复用原创角色立绘，画面明确属于概念设计，不代表实际玩法或实机表现。

在仓库根目录运行：

```powershell
go run . setup --shared
go run . render --project demo/game-trailer
go run . bundle --project demo/game-trailer --target mp4
go run . mp4 verify --input demo/game-trailer/deliverables/mp4/video.mp4 --silent --fast-start
```

制作真实宣传片时接入已授权的游戏实录/场景图片、配乐与可验证的声明；保持镜头证据和审片记录。MP4 不提交。
