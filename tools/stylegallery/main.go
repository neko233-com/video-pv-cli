// stylegallery builds an idempotent visual index from verified native-GPU PNGs.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type entry struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Family   string   `json:"family"`
	Keywords []string `json:"keywords"`
	PNG      string   `json:"source_png"`
	SHA256   string   `json:"source_png_sha256"`
	X, Y     float64
	Index    int
}
type preset struct {
	ID, Name, Family string
	Keywords         []string
}

func read(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}
func confined(root, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute evidence path")
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(resolvedRoot, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("evidence path escapes root")
	}
	return path, nil
}
func atomic(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".pv-gallery-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func build(evidence, out string) error {
	var report struct {
		Passed        bool
		Styles        int
		Batches       []string
		CatalogSHA256 string            `json:"catalog_sha256"`
		SourceSHA256  map[string]string `json:"source_sha256"`
	}
	if err := read(filepath.Join(evidence, "catalog-report.json"), &report); err != nil {
		return err
	}
	if !report.Passed || report.Styles != 100 {
		return fmt.Errorf("100 verified styles required")
	}
	entries := map[string]entry{}
	for _, batch := range report.Batches {
		root, err := confined(evidence, batch)
		if err != nil {
			return err
		}
		atlasPath, err := confined(root, "template.json")
		if err != nil {
			return err
		}
		pixelPath, err := confined(root, "qa/pixel-report.json")
		if err != nil {
			return err
		}
		var atlas struct {
			Cases []struct {
				ID       string
				Template struct{ Style *preset }
			}
		}
		if err := read(atlasPath, &atlas); err != nil {
			return err
		}
		var pixels []struct{ Case, PNG, SHA256 string }
		if err := read(pixelPath, &pixels); err != nil {
			return err
		}
		if len(pixels) != len(atlas.Cases) {
			return fmt.Errorf("pixel coverage differs")
		}
		for i, c := range atlas.Cases {
			if !strings.HasPrefix(c.ID, "style/") || !strings.HasSuffix(c.ID, "/0") {
				continue
			}
			s := c.Template.Style
			if s == nil || c.ID != "style/"+s.ID+"/0" || pixels[i].Case != c.ID {
				return fmt.Errorf("style evidence mismatch")
			}
			path, err := confined(root, pixels[i].PNG)
			if err != nil {
				return err
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(raw)
			if hex.EncodeToString(sum[:]) != pixels[i].SHA256 {
				return fmt.Errorf("changed source PNG")
			}
			relative, _ := filepath.Rel(evidence, path)
			entries[s.ID] = entry{ID: s.ID, Name: s.Name, Family: s.Family, Keywords: s.Keywords, PNG: filepath.ToSlash(relative), SHA256: pixels[i].SHA256}
		}
	}
	if len(entries) != 100 {
		return fmt.Errorf("expected 100 style images, got %d", len(entries))
	}
	marker := []byte("video-pv-cli/style-gallery/1\n")
	if existing, err := os.ReadDir(out); err == nil && len(existing) > 0 {
		raw, err := os.ReadFile(filepath.Join(out, ".pv-gallery-owner"))
		if err != nil || !bytes.Equal(raw, marker) {
			return fmt.Errorf("unowned gallery output")
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		return err
	}
	list := []entry{}
	for _, v := range entries {
		list = append(list, v)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	const w, h, columns = 320, 180, 8
	sprite := image.NewNRGBA(image.Rect(0, 0, w*columns, h*((len(list)+columns-1)/columns)))
	for i := range list {
		item := &list[i]
		item.Index = i + 1
		item.X = float64(i%columns) * 100 / float64(columns-1)
		item.Y = float64(i/columns) * 100 / float64((len(list)+columns-1)/columns-1)
		path, err := confined(evidence, item.PNG)
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		img, err := png.Decode(file)
		file.Close()
		if err != nil {
			return err
		}
		b := img.Bounds()
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				sprite.Set(i%columns*w+x, i/columns*h+y, img.At(b.Min.X+x*b.Dx()/w, b.Min.Y+y*b.Dy()/h))
			}
		}
	}
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, sprite); err != nil {
		return err
	}
	var htmlData bytes.Buffer
	if err := template.Must(template.New("gallery").Parse(page)).Execute(&htmlData, list); err != nil {
		return err
	}
	index, _ := json.MarshalIndent(list, "", "  ")
	spriteHash := sha256.Sum256(imageData.Bytes())
	provenance, _ := json.MarshalIndent(map[string]any{"schema": "video-pv-cli/style-gallery/1", "styles": 100, "catalog_sha256": report.CatalogSHA256, "renderer_source_sha256": report.SourceSHA256, "sprite_sha256": hex.EncodeToString(spriteHash[:]), "frame_selection": "verified start frame /0"}, "", "  ")
	for _, file := range []struct {
		name string
		data []byte
	}{{".pv-gallery-owner", marker}, {"sprite.png", imageData.Bytes()}, {"index.json", append(index, '\n')}, {"index.html", htmlData.Bytes()}, {"provenance.json", append(provenance, '\n')}} {
		if err := atomic(filepath.Join(out, file.name), file.data); err != nil {
			return err
		}
	}
	return nil
}
func main() {
	evidence := flag.String("evidence", "work/catalog-validation", "passed style/full catalog evidence")
	out := flag.String("out", "docs/style-gallery", "owned visual index directory")
	flag.Parse()
	root, err := filepath.Abs(*evidence)
	if err == nil {
		err = build(root, *out)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Built 100 verified style thumbnails:", *out)
}

const page = `<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>PV · 100 风格索引</title><style>
:root{font-family:system-ui,"Microsoft YaHei",sans-serif;color-scheme:dark;background:#111316;color:#e9edf3}*{box-sizing:border-box}body{margin:0 auto;max-width:1500px;padding:42px 28px}header{display:flex;justify-content:space-between;align-items:end;gap:24px;border-bottom:1px solid #343b45;padding-bottom:24px}h1{font-size:30px;font-weight:550;margin:0 0 14px}p{color:#b3bfce;font-size:14px;line-height:1.7;max-width:760px}label{font-size:13px;display:grid;gap:9px}input{font:inherit;width:300px;background:#1c222b;border:1px solid #4b5667;color:inherit;padding:11px}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(320px,1fr));gap:24px;margin:30px 0}article{border:1px solid #343b45;background:#171c23;padding:18px;overflow:hidden}.thumb{width:100%;aspect-ratio:16/9;background-image:url(sprite.png);background-repeat:no-repeat;background-size:800% 1300%}h2{font-size:17px;font-weight:550;margin:18px 0 7px}code{font-size:11px;color:#c7d1df;overflow-wrap:anywhere}article p{font-size:12px;margin-bottom:0}article a{display:inline-block;color:#c7d9ff;margin-top:14px;font-size:13px;text-underline-offset:3px}article[hidden]{display:none}#count{font-size:13px;white-space:nowrap}@media(max-width:700px){body{padding:24px 12px}header{display:block}input{width:100%}.grid{grid-template-columns:1fr}article{padding:12px}}</style>
<header><div><h1>PV · 100 风格索引</h1><p>来自 Windows 原生 WebGPU 实测的起始帧。每个预设包含独立关键词、导演方向和可执行渲染参数；实际产品、角色与材质仍需使用授权素材制作。这是选型索引，完整运动请在验证台或实际 MP4 中检查。</p><span id="count">100 / 100</span></div><label>按风格、分类、关键词筛选<input id="search" type="search" placeholder="例如：anime / keynote / rim light"></label></header><main class="grid">{{range .}}<article><div class="thumb" role="img" aria-label="{{.Name}}起始帧" style="background-position:{{.X}}% {{.Y}}%"></div><h2>{{.Index}} · {{.Name}}</h2><code>{{.ID}}</code><p>{{.Family}} · {{range $i,$word:=.Keywords}}{{if $i}} / {{end}}{{$word}}{{end}}</p><a href="/?style={{.ID}}">在验证台预览</a></article>{{end}}</main><script>const input=document.getElementById('search'),cards=[...document.querySelectorAll('article')];input.addEventListener('input',()=>{const query=input.value.trim().toLowerCase();let shown=0;for(const card of cards){card.hidden=!card.textContent.toLowerCase().includes(query);if(!card.hidden)shown++;}document.getElementById('count').textContent=shown+' / 100';});</script></html>`
