package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompleteDataCatalogAndSceneCoverage(t *testing.T) {
	templates, transitions, err := readCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(templates) < 128 || len(transitions) < 76 {
		t.Fatalf("catalog shrank: %d %d", len(templates), len(transitions))
	}
	used := map[string]bool{}
	types := map[string]bool{}
	backgrounds := map[string]bool{}
	modes := map[int]bool{}
	for _, tr := range transitions {
		modes[tr.Mode] = true
	}
	for _, tpl := range templates {
		if templateDuration(tpl) <= 0 {
			t.Fatal("nonpositive timeline")
		}
		for i, s := range tpl.Scenes {
			types[s.Type] = true
			backgrounds[s.Background] = true
			if i < len(tpl.Scenes)-1 {
				used[s.Transition] = true
			}
		}
	}
	for id := range transitions {
		if !used[id] {
			t.Errorf("transition %s is never used by a preset", id)
		}
	}
	for _, kind := range sceneTypes {
		if !types[kind] {
			t.Errorf("scene type %s has no data template", kind)
		}
	}
	if len(modes) != 48 || len(backgrounds) != 12 {
		t.Fatalf("missing actual shader/scene coverage: modes=%d backgrounds=%d", len(modes), len(backgrounds))
	}
}

func TestTemplateDataPreflightAndManifestSync(t *testing.T) {
	templates, transitions, err := readCatalog()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "editable")
	tpl := templates["keynote-product-porcelain"]
	if err := initializeDataProject(root, tpl, transitions, ""); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "template.json")
	tpl.Scenes[0].Duration += 1
	data, _ := json.Marshal(tpl)
	_ = os.WriteFile(path, data, 0644)
	_, report := validateManifest(filepath.Join(root, "pv-project.json"), false)
	if !hasCheck(report, "template.data", "fail") {
		t.Fatal("profile mismatch accepted")
	}
	if err := synchronizeDataManifest(root); err != nil {
		t.Fatal(err)
	}
	p, report := validateManifest(filepath.Join(root, "pv-project.json"), false)
	if reportHasFailure(report) || math.Abs(p.Project.DurationSeconds-templateDuration(tpl)) > .000001 || p.Review.CreativeApproved || p.Shots[0].Status != "draft" {
		t.Fatalf("bad sync: %+v", report)
	}
	if _, err := os.Stat(filepath.Join(root, "pv-project.json.before-sync")); err != nil {
		t.Fatal(err)
	}
	if err := synchronizeDataManifest(root); err == nil {
		t.Fatal("sync backup overwritten")
	}
	tpl.Scenes[0].Image = "https://remote.invalid/art.png"
	data, _ = json.Marshal(tpl)
	_ = os.WriteFile(path, data, 0644)
	_, report = validateManifest(filepath.Join(root, "pv-project.json"), false)
	if !hasCheck(report, "template.data", "fail") {
		t.Fatal("remote image accepted")
	}
}

func TestTemplateValidationRejectsUnrenderableData(t *testing.T) {
	templates, transitions, err := readCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*TemplateData){
		"unknown-layout":   func(tpl *TemplateData) { tpl.Scenes[0].Layout = "oops" },
		"three-overlap":    func(tpl *TemplateData) { tpl.Scenes[0].TransitionDuration = 3; tpl.Scenes[1].TransitionDuration = 2 },
		"nan-motion":       func(tpl *TemplateData) { tpl.Scenes[0].Motion.Strength = math.NaN() },
		"missing-items":    func(tpl *TemplateData) { tpl.Scenes[0].Type = "map-trace"; tpl.Scenes[0].Items = nil },
		"missing-image":    func(tpl *TemplateData) { tpl.Scenes[0].Type = "gallery"; tpl.Scenes[0].Image = "" },
		"multi-scene-loop": func(tpl *TemplateData) { tpl.Loop = true },
	} {
		t.Run(name, func(t *testing.T) {
			tpl := templates["keynote-product-porcelain"]
			tpl.Scenes = append([]SceneData(nil), tpl.Scenes...)
			mutate(&tpl)
			if err := validateTemplate(tpl, transitions); err == nil {
				t.Fatal("invalid data accepted")
			}
		})
	}
	tr := transitions["crossfade"]
	tr.Mode = 48
	if err := validateTransition(tr.ID, tr); err == nil {
		t.Fatal("unimplemented shader mode accepted")
	}
	tr = transitions["crossfade"]
	tr.Direction = []float64{math.Inf(1), 0}
	if err := validateTransition(tr.ID, tr); err == nil {
		t.Fatal("invalid direction accepted")
	}
}

func TestAtlasCoverageIncludesEverySceneTransitionAndLoop(t *testing.T) {
	templates, transitions, err := readCatalog()
	if err != nil {
		t.Fatal(err)
	}
	cases := catalogCases(templates, transitions, "deep", "")
	seen := map[string]bool{}
	loops := 0
	endpoints := 0
	for _, c := range cases {
		if seen[c.ID] {
			t.Fatalf("duplicate case %s", c.ID)
		}
		seen[c.ID] = true
		if strings.HasSuffix(c.ID, "/loop-end") {
			loops++
		}
		if c.Transition != "" && (c.Phase == 0 || c.Phase == 1) {
			endpoints++
		}
	}
	if len(cases) != 2140 || loops != 16 || endpoints != 152 {
		t.Fatalf("coverage changed: cases=%d loops=%d endpoints=%d", len(cases), loops, endpoints)
	}
}

func TestPixelComparisonMeasuresVisibleDifference(t *testing.T) {
	a := image.NewRGBA(image.Rect(0, 0, 2, 2))
	b := image.NewRGBA(a.Bounds())
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			a.Set(x, y, color.RGBA{30, 30, 30, 255})
			b.Set(x, y, color.RGBA{33, 33, 33, 255})
		}
	}
	if math.Abs(imageMAE(a, b)-3) > .001 || imageMAE(a, a) != 0 || !math.IsInf(imageMAE(a, image.NewRGBA(image.Rect(0, 0, 3, 2))), 1) {
		t.Fatal("invalid pixel comparison")
	}
}

func TestDataProjectHasOneEditableTemplateFile(t *testing.T) {
	templates, transitions, err := readCatalog()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "keynote")
	tpl := templates["keynote-product-porcelain"]
	if err := initializeDataProject(root, tpl, transitions, "A product <test>"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "template.json"))
	if err != nil {
		t.Fatal(err)
	}
	var actual TemplateData
	if err := decodeStrict(data, &actual); err != nil {
		t.Fatal(err)
	}
	if actual.ProjectTitle != "A product <test>" {
		t.Fatal("title was not stored in data")
	}
	source, _ := os.ReadFile(filepath.Join(root, "pv.html"))
	if bytes.Contains(source, []byte("A product")) || !bytes.Contains(source, []byte("catalog-runtime.js")) {
		t.Fatal("project content was baked into HTML")
	}
	p, r := validateManifest(filepath.Join(root, "pv-project.json"), false)
	if reportHasFailure(r) || p.Project.Type != "product" {
		t.Fatalf("invalid data project: %+v", r)
	}
}

func TestCatalogRejectsUnknownReferencesAndFields(t *testing.T) {
	templates, transitions, err := readCatalog()
	if err != nil {
		t.Fatal(err)
	}
	t1 := templates["keynote-product-porcelain"]
	t1.Scenes = append([]SceneData{}, t1.Scenes...)
	t1.Scenes[0].Transition = "not-real"
	if err := validateTemplate(t1, transitions); err == nil {
		t.Fatal("unknown transition accepted")
	}
	raw, _ := json.Marshal(t1)
	raw = []byte(strings.Replace(string(raw), `"width":1920`, `"width":1920,"widht":1920`, 1))
	var out TemplateData
	if err := decodeStrict(raw, &out); err == nil {
		t.Fatal("unknown data field accepted")
	}
}
