# AURA · 静场揭幕

1920×1080 / 30fps，约 22.43 秒。`keynote-silent-reveal` 风格：深色舞台、克制留白、原创程序化产品示意、逐层揭幕和稳定收尾。修改 `template.json` 即可调整文案、配色与节奏；程序化形体不是实物产品模型。

在仓库根目录运行：

```powershell
go run . setup --shared
go run . render --project demo/keynote-product
go run . bundle --project demo/keynote-product --target mp4
go run . mp4 verify --input demo/keynote-product/deliverables/mp4/video.mp4 --silent --fast-start
```

MP4、本地渲染缓存和报告均不提交。此案例保留 draft 审片状态。
