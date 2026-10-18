package main

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
)

func styleLuminance(hex string) float64 {
	sum := 0.0
	for i, weight := range []float64{.2126, .7152, .0722} {
		channel, _ := strconv.ParseUint(hex[1+i*2:3+i*2], 16, 8)
		v := float64(channel) / 255
		if v <= .04045 {
			v /= 12.92
		} else {
			v = math.Pow((v+.055)/1.055, 2.4)
		}
		sum += v * weight
	}
	return sum
}

func TestStylePalettesMatchDirectionAndReadableForegrounds(t *testing.T) {
	templates, transitions, _ := readCatalog()
	styles, _ := readStyles(templates, transitions)
	palettes := map[string]bool{}
	for id, s := range styles {
		raw, _ := json.Marshal(s.Render.Theme)
		palettes[string(raw)] = true
		foreground := styleLuminance(s.Render.Theme.Foreground)
		for _, surface := range []string{s.Render.Theme.Background, s.Render.Theme.Surface} {
			background := styleLuminance(surface)
			contrast := (math.Max(foreground, background) + .05) / (math.Min(foreground, background) + .05)
			if contrast < 4.5 {
				t.Fatal("low base text contrast", id, contrast)
			}
		}
	}
	if len(palettes) != 100 {
		t.Fatal("style-specific palettes missing", len(palettes))
	}
	for _, id := range []string{"luxury-pearl-satin", "tech-medical-clean", "game-cozy-world", "anime-shojo-petal", "cinema-high-key-studio"} {
		if styleLuminance(styles[id].Render.Theme.Background) < .7 {
			t.Fatal("light direction has a dark field", id)
		}
	}
	if styleLuminance(styles["keynote-monochrome-stage"].Render.Theme.Background) > .02 {
		t.Fatal("black stage lost contrast direction")
	}
}

func TestStyleCatalogHas100IndependentRecipes(t *testing.T) {
	templates, transitions, err := readCatalog()
	if err != nil {
		t.Fatal(err)
	}
	styles, err := readStyles(templates, transitions)
	if err != nil {
		t.Fatal(err)
	}
	if len(styles) != 100 {
		t.Fatal(len(styles))
	}
	families := map[string]int{}
	recipes := map[string]string{}
	for id, s := range styles {
		families[s.Family]++
		r := s.Render
		r.Theme = ThemeData{}
		r.BaseTemplate = ""
		raw, _ := json.Marshal(r)
		if prior, ok := recipes[string(raw)]; ok {
			t.Fatalf("palette-only duplicate: %s / %s", prior, id)
		}
		recipes[string(raw)] = id
		base := templates[s.Render.BaseTemplate]
		original, _ := json.Marshal(base)
		styled := applyStyle(base, s)
		if err := validateTemplate(styled, transitions); err != nil {
			t.Fatal(id, err)
		}
		repeated, _ := json.Marshal(applyStyle(styled, s))
		first, _ := json.Marshal(styled)
		if !bytes.Equal(first, repeated) {
			t.Fatal("non-idempotent recipe", id)
		}
		after, _ := json.Marshal(base)
		if !bytes.Equal(original, after) {
			t.Fatal("mutated base", id)
		}
		for i, scene := range styled.Scenes {
			source := base.Scenes[i]
			if scene.Title != source.Title || scene.Image != source.Image || scene.Type != source.Type || scene.Duration != source.Duration {
				t.Fatal("style altered content", id)
			}
		}
		for templateID, template := range templates {
			if err := validateTemplate(applyStyle(template, s), transitions); err != nil {
				t.Fatal("style/template combination", id, templateID, err)
			}
		}
	}
	for _, family := range styleFamilies {
		if families[family] != 10 {
			t.Fatal(family, families[family])
		}
	}
	cases := styleCases(templates, transitions, styles, true)
	if len(cases) != 500 {
		t.Fatal(len(cases))
	}
	for _, c := range cases {
		if err := validateTemplate(c.Template, transitions); err != nil {
			t.Fatal(c.ID, err)
		}
	}
}

func TestStyleApplyRejectsInvalidProjectWithoutWriting(t *testing.T) {
	templates, transitions, _ := readCatalog()
	styles, _ := readStyles(templates, transitions)
	root := filepath.Join(t.TempDir(), "invalid")
	if err := initializeDataProject(root, templates["keynote-product-porcelain"], transitions, ""); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "pv-project.json")
	raw, _ := os.ReadFile(path)
	var p ProjectFile
	_ = json.Unmarshal(raw, &p)
	p.Assets[0].Path = "../outside"
	raw, _ = json.Marshal(p)
	_ = os.WriteFile(path, raw, 0600)
	before := snapshotFiles(t, root)
	if applyProjectStyle(root, styles["keynote-silent-reveal"], transitions) == nil {
		t.Fatal("invalid project accepted")
	}
	if !reflect.DeepEqual(before, snapshotFiles(t, root)) {
		t.Fatal("failed style application wrote files")
	}
}

func TestStyleRejectsInvalidTokens(t *testing.T) {
	templates, transitions, _ := readCatalog()
	styles, _ := readStyles(templates, transitions)
	base := styles["keynote-silent-reveal"]
	mutations := []func(*StylePreset){
		func(s *StylePreset) { s.ID = "../escape" }, func(s *StylePreset) { s.Family = "unknown" },
		func(s *StylePreset) { s.Render.Typography.Scale = math.NaN() }, func(s *StylePreset) { s.Render.Graphics.Opacity = 1 },
		func(s *StylePreset) { s.Render.Graphics.Overlay = "unknown" }, func(s *StylePreset) { s.Render.Backgrounds = []string{"webgl"} },
		func(s *StylePreset) { s.Render.Transitions = []string{"unknown"} }, func(s *StylePreset) { s.Keywords = []string{"same", "same"} },
		func(s *StylePreset) { s.Render.OverlapRatio = .9 }, func(s *StylePreset) { s.Direction.Materials = "" },
	}
	for i, change := range mutations {
		s := base
		change(&s)
		if validateStyle(s, transitions) == nil {
			t.Fatal("accepted", i)
		}
	}
}

func TestApplyStylePreservesAssetsAndIsIdempotent(t *testing.T) {
	templates, transitions, _ := readCatalog()
	styles, _ := readStyles(templates, transitions)
	root := filepath.Join(t.TempDir(), "project")
	if err := initializeDataProject(root, templates["character-reveal-midnight"], transitions, "Character"); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(root, "pv-project.json"))
	var before ProjectFile
	_ = json.Unmarshal(raw, &before)
	before.Claims = []Claim{{ID: "truth", Text: "Approved fictional sample", EvidenceAssetIDs: []string{before.Assets[0].ID}}}
	before.Shots[0].ClaimIDs = []string{"truth"}
	raw, _ = json.Marshal(before)
	if err := os.WriteFile(filepath.Join(root, "pv-project.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	s := styles["anime-celestial-heroine"]
	if err := applyProjectStyle(root, s, transitions); err != nil {
		t.Fatal(err)
	}
	first := snapshotFiles(t, root)
	if err := applyProjectStyle(root, s, transitions); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, snapshotFiles(t, root)) {
		t.Fatal("repeat changed files")
	}
	raw, _ = os.ReadFile(filepath.Join(root, "pv-project.json"))
	var after ProjectFile
	_ = json.Unmarshal(raw, &after)
	if !reflect.DeepEqual(before.Assets, after.Assets) {
		t.Fatal("assets changed")
	}
	if !reflect.DeepEqual(before.Claims, after.Claims) || !reflect.DeepEqual(before.Shots[0].ClaimIDs, after.Shots[0].ClaimIDs) || !reflect.DeepEqual(before.Shots[0].AssetIDs, after.Shots[0].AssetIDs) {
		t.Fatal("style lost evidence bindings")
	}
	if _, report := validateManifest(filepath.Join(root, "pv-project.json"), false); reportHasFailures(report) {
		t.Fatal(report)
	}
	for _, name := range []string{"template.json.before-style", "pv-project.json.before-style"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestInitStyleAndFilterCLI(t *testing.T) {
	var out, stderr bytes.Buffer
	root := filepath.Join(t.TempDir(), "styled")
	if code := realMain([]string{"init", "--dir", root, "--style", "keynote-silent-reveal"}, &out, &stderr); code != 0 {
		t.Fatal(code, stderr.String())
	}
	raw, _ := os.ReadFile(filepath.Join(root, "template.json"))
	var tpl TemplateData
	_ = json.Unmarshal(raw, &tpl)
	if tpl.Style == nil || tpl.Style.ID != "keynote-silent-reveal" {
		t.Fatal("missing style")
	}
	out.Reset()
	if realMain([]string{"style", "list", "--family", "anime", "--json"}, &out, &stderr) != 0 {
		t.Fatal(stderr.String())
	}
	var list []StylePreset
	if json.Unmarshal(out.Bytes(), &list) != nil || len(list) != 10 {
		t.Fatal("filter failed")
	}
}
