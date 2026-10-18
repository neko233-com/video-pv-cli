# 水下光纹 · 官网循环背景

1920×1080 / 30fps / 6 秒周期。以 `organic-underwater-caustics` 为方向，项目 JSON 将背景明确改为原生 WGSL `caustics`。单镜头无烘焙文字，网页文案作为前景 HTML，便于响应式排版。

在仓库根目录运行：

```powershell
go run . setup --shared
go run . render --project demo/website-background
go run . bundle --project demo/website-background --target website
go run . mp4 verify --input demo/website-background/deliverables/website/background.mp4 --silent --fast-start
go run . preview --bundle demo/website-background/deliverables/website
```

打开 `/delivery-audit` 可验证实际播放与两次循环。网站包自动静音、循环、内联播放；视频无控件、不可点击、不进入键盘焦点，减少动态效果偏好显示海报。`background.html`、MP4 与海报留在本地 `deliverables/website/`，不提交。
