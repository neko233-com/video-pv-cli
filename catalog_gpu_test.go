package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestGPUCatalogPixels(t *testing.T) {
	if os.Getenv("WEB2VIDEO_CATALOG_INTEGRATION") != "1" {
		t.Skip("enable WEB2VIDEO_CATALOG_INTEGRATION=1 for every style and transition")
	}
	for _, kind := range []string{"transitions", "styles"} {
		t.Run(kind, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), kind)
			var out, errOut bytes.Buffer
			if runInit([]string{"--dir", root, "--template", "game-trailer"}, &out, &errOut) != 0 {
				t.Fatal(errOut.String())
			}
			p, err := readProject(root)
			if err != nil {
				t.Fatal(err)
			}
			p.Width = 640
			p.Height = 360
			p.Scenes = nil
			p.Loop = false
			folder := "assets/styles"
			if kind == "transitions" {
				folder = "assets/catalog/transitions"
			}
			entries, _ := fs.ReadDir(bundled, folder)
			var times []string
			if kind == "transitions" {
				p.Scenes = append(p.Scenes, Scene{ID: "base", Duration: 1.5, Layout: "ambient", Image: "assets/world.png", Zoom: 1, X: .5, Y: .5, Transition: "cut"})
			}
			for i, entry := range entries {
				id := strings.TrimSuffix(entry.Name(), ".json")
				scene := Scene{ID: fmt.Sprintf("case-%03d", i), Duration: 1.5, Layout: "ambient", Image: "assets/world.png", Zoom: 1, X: .5, Y: .5, Transition: "crossfade"}
				if kind == "styles" {
					scene.Layout = "hero"
					scene.Title = "FRAME"
					scene.Kicker = id
					scene.Style = id
				} else {
					scene.Transition = id
					if i%2 == 0 {
						scene.Image = "assets/character.png"
					}
				}
				start := float64(len(p.Scenes)) * 1.5
				times = append(times, strconv.FormatFloat(start+.3, 'f', 3, 64))
				p.Scenes = append(p.Scenes, scene)
			}
			raw, _ := json.Marshal(p)
			os.WriteFile(filepath.Join(root, "project.json"), raw, 0600)
			if err = buildProject(root); err != nil {
				t.Fatal(err)
			}
			if runSnapshot([]string{"--project", root, "--at", strings.Join(times, ",")}, &out, &errOut) != 0 {
				t.Fatal(out.String(), errOut.String())
			}
			files, _ := filepath.Glob(filepath.Join(root, ".web2video", "snapshots", "*.png"))
			if len(files) != len(entries) {
				t.Fatalf("%s: expected %d snapshots, got %d", kind, len(entries), len(files))
			}
			distinct := map[string]bool{}
			for _, file := range files {
				f, e := os.Open(file)
				if e != nil {
					t.Fatal(e)
				}
				img, e := png.Decode(f)
				f.Close()
				if e != nil {
					t.Fatal(e)
				}
				low, high := uint32(65535), uint32(0)
				for y := 0; y < img.Bounds().Dy(); y += 4 {
					for x := 0; x < img.Bounds().Dx(); x += 4 {
						r, g, b, _ := img.At(x, y).RGBA()
						v := (r + g + b) / 3
						if v < low {
							low = v
						}
						if v > high {
							high = v
						}
					}
				}
				if high-low < 4096 {
					t.Fatal("flat catalog frame", file)
				}
				sum, _ := fileSHA256(file)
				distinct[sum] = true
			}
			if len(distinct) < len(entries)/3 {
				t.Fatalf("catalog effect wiring collapsed: %d distinct frames / %d", len(distinct), len(entries))
			}
			t.Logf("%s: %d native WebGPU frames, %d distinct pixel hashes", kind, len(files), len(distinct))
		})
	}
}

func TestGPUPeriodicLoop(t *testing.T) {
	if os.Getenv("WEB2VIDEO_CATALOG_INTEGRATION") != "1" {
		t.Skip("enable catalog GPU integration")
	}
	root := filepath.Join(t.TempDir(), "loop")
	var out, errOut bytes.Buffer
	if runInit([]string{"--dir", root, "--template", "website-background"}, &out, &errOut) != 0 {
		t.Fatal(errOut.String())
	}
	if runSnapshot([]string{"--project", root, "--at", "0,3,6"}, &out, &errOut) != 0 {
		t.Fatal(out.String(), errOut.String())
	}
	first, err := fileSHA256(filepath.Join(root, ".web2video", "snapshots", "at-0s.png"))
	if err != nil {
		t.Fatal(err)
	}
	last, err := fileSHA256(filepath.Join(root, ".web2video", "snapshots", "at-6s.png"))
	if err != nil || first != last {
		t.Fatal("loop endpoint pixels differ", err)
	}
	middle, err := fileSHA256(filepath.Join(root, ".web2video", "snapshots", "at-3s.png"))
	if err != nil || middle == first {
		t.Fatal("loop has no pixel motion at its midpoint", err)
	}
	t.Log("loop endpoint PNGs at 0s and 6s are byte-identical")
}
