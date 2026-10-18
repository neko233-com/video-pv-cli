package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
)

type catalogPixelGate struct {
	frames, endpoints, loops int
	references               map[string]image.Image
}
type pixelEvidence struct {
	Case         string   `json:"case"`
	PNG          string   `json:"png"`
	SHA256       string   `json:"sha256"`
	LumaStdDev   float64  `json:"luma_stddev"`
	ReferenceMAE *float64 `json:"reference_mae,omitempty"`
}

func newCatalogPixelGate() *catalogPixelGate {
	return &catalogPixelGate{references: map[string]image.Image{}}
}
func imageMAE(a, b image.Image) float64 {
	if a.Bounds() != b.Bounds() {
		return math.Inf(1)
	}
	sum, n := 0.0, 0
	for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			r, g, bl, _ := a.At(x, y).RGBA()
			rr, gg, bb, _ := b.At(x, y).RGBA()
			sum += math.Abs(float64(r)-float64(rr)) + math.Abs(float64(g)-float64(gg)) + math.Abs(float64(bl)-float64(bb))
			n += 3
		}
	}
	return sum / float64(n) / 257
}
func (g *catalogPixelGate) checkBatch(project string, cases []AtlasCase) error {
	data, err := os.ReadFile(filepath.Join(project, "qa", "render-report.json"))
	if err != nil {
		return err
	}
	var report BuildReport
	if err = json.Unmarshal(data, &report); err != nil {
		return err
	}
	stills := ""
	for _, step := range report.Steps {
		if step.Command == "stills" {
			for _, artifact := range step.Result.Artifacts {
				if filepath.Base(artifact.Path) == "_sheet.png" {
					stills = filepath.Dir(artifact.Path)
				}
			}
		}
	}
	if stills == "" {
		return fmt.Errorf("missing stills job")
	}
	rows := []pixelEvidence{}
	for i, c := range cases {
		path := filepath.Join(stills, fmt.Sprintf("t-%.2f.png", float64(i)/6))
		if !filepath.IsAbs(path) {
			path = filepath.Join(project, path)
		}
		rel, err := filepath.Rel(project, path)
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
		if img.Bounds().Dx() != 1280 || img.Bounds().Dy() != 720 {
			return fmt.Errorf("%s: unexpected PNG dimensions", c.ID)
		}
		sum, square, n := 0.0, 0.0, 0.0
		for y := 0; y < 720; y += 3 {
			for x := 0; x < 1280; x += 3 {
				r, gg, b, a := img.At(x, y).RGBA()
				if a != 65535 {
					return fmt.Errorf("%s: transparent output", c.ID)
				}
				v := (.2126*float64(r) + .7152*float64(gg) + .0722*float64(b)) / 257
				sum += v
				square += v * v
				n++
			}
		}
		std := math.Sqrt(math.Max(0, square/n-(sum/n)*(sum/n)))
		if c.Transition == "" && std < .5 {
			return fmt.Errorf("%s: suspiciously blank output (luma stddev %.4f)", c.ID, std)
		}
		raw, _ := os.ReadFile(path)
		hash := sha256.Sum256(raw)
		row := pixelEvidence{Case: c.ID, PNG: filepath.ToSlash(rel), SHA256: hex.EncodeToString(hash[:]), LumaStdDev: std}
		key := ""
		if c.Transition != "" && (c.Phase == 0 || c.Phase == 1) {
			key = fmt.Sprintf("transition-endpoint-%g", c.Phase)
		}
		if strings.HasSuffix(c.ID, "/loop-start") {
			g.references[strings.TrimSuffix(c.ID, "/loop-start")] = img
		}
		if strings.HasSuffix(c.ID, "/loop-end") {
			key = strings.TrimSuffix(c.ID, "/loop-end")
		}
		if key != "" {
			if ref, ok := g.references[key]; ok {
				mae := imageMAE(ref, img)
				row.ReferenceMAE = &mae
				if mae > .15 {
					return fmt.Errorf("%s: endpoint/loop mismatch MAE %.5f > 0.15 channel levels", c.ID, mae)
				}
				if c.Transition != "" {
					g.endpoints++
				} else {
					g.loops++
					delete(g.references, key)
				}
			} else {
				g.references[key] = img
			}
		}
		g.frames++
		rows = append(rows, row)
	}
	data, err = json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(project, "qa", "pixel-report.json"), append(data, '\n'))
}
