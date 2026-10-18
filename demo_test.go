package main

import (
	"encoding/json"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestDemoSourcesAreSelfContainedAndDraft(t *testing.T) {
	for _, name := range []string{"keynote-product", "anime-character", "game-trailer", "website-background"} {
		t.Run(name, func(t *testing.T) {
			root := filepath.Join("demo", name)
			project, report := validateManifest(filepath.Join(root, "pv-project.json"), false)
			report.finish(false)
			if !report.Passed {
				t.Fatalf("demo preflight: %+v", report)
			}
			for _, asset := range project.Assets {
				if _, err := bundled.ReadFile("demo/" + name + "/" + asset.Path); err != nil {
					t.Fatal("demo depends on a missing embedded source", asset.Path, err)
				}
			}
			_, release := validateManifest(filepath.Join(root, "pv-project.json"), true)
			release.finish(true)
			if release.Passed {
				t.Fatal("sample unexpectedly approved for release")
			}
			var data TemplateData
			raw, _ := bundled.ReadFile("demo/" + name + "/template.json")
			if err := json.Unmarshal(raw, &data); err != nil {
				t.Fatal(err)
			}
			if data.Style == nil || data.Width != 1920 || data.Height != 1080 || data.FPS != 30 {
				t.Fatal("demo lost its style or delivery profile")
			}
			if name == "website-background" {
				if !data.Loop || len(data.Scenes) != 1 || data.Scenes[0].Background != "caustics" {
					t.Fatal("website demo must be a periodic caustic field")
				}
				s := data.Scenes[0]
				if s.Title != "" || s.Subtitle != "" || s.Overline != "" || s.Image != "" {
					t.Fatal("website demo has baked foreground content")
				}
			}
		})
	}
	err := fs.WalkDir(bundled, "demo", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(strings.ToLower(path), ".mp4") || strings.Contains(path, "/deliverables/") || strings.Contains(path, "/renders/") || strings.Contains(path, "/qa/") {
			t.Fatal("generated demo output embedded", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
