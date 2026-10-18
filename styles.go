package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type StyleTypography struct {
	Scale        float64 `json:"scale"`
	WeightOffset int     `json:"weight_offset"`
	Tracking     float64 `json:"tracking"`
	Leading      float64 `json:"leading"`
}
type StyleGraphics struct {
	PanelRadius float64 `json:"panel_radius"`
	Overlay     string  `json:"overlay"`
	Opacity     float64 `json:"opacity"`
	LineWidth   float64 `json:"line_width"`
}
type StyleRender struct {
	BaseTemplate string          `json:"base_template"`
	Theme        ThemeData       `json:"theme"`
	Backgrounds  []string        `json:"backgrounds"`
	Layouts      []string        `json:"layouts"`
	Motion       MotionData      `json:"motion"`
	Transitions  []string        `json:"transitions"`
	OverlapRatio float64         `json:"overlap_ratio"`
	Typography   StyleTypography `json:"typography"`
	Graphics     StyleGraphics   `json:"graphics"`
}
type StyleDirection struct {
	Composition string `json:"composition"`
	Typography  string `json:"typography"`
	Lighting    string `json:"lighting"`
	Materials   string `json:"materials"`
	Camera      string `json:"camera"`
	Editing     string `json:"editing"`
	Sound       string `json:"sound"`
}
type StylePreset struct {
	SchemaVersion string         `json:"schema_version"`
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Family        string         `json:"family"`
	Summary       string         `json:"summary"`
	Keywords      []string       `json:"keywords"`
	Avoid         []string       `json:"avoid"`
	BestFor       []string       `json:"best_for"`
	Direction     StyleDirection `json:"direction"`
	BeatPlan      []string       `json:"beat_plan"`
	ReferenceIDs  []string       `json:"reference_ids"`
	Render        StyleRender    `json:"render"`
}

var styleIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{2,63}$`)
var styleFamilies = strings.Fields("keynote luxury tech editorial game anime cinema organic graphic digital")
var styleOverlays = strings.Fields("none frame registration scanlines halftone letterbox crosshair corner-ticks contours dot-field")

func validateStyle(s StylePreset, transitions map[string]TransitionData) error {
	bad := func(detail string) error { return fmt.Errorf("style %s: %s", s.ID, detail) }
	if s.SchemaVersion != "1.0" || !styleIDPattern.MatchString(s.ID) || strings.TrimSpace(s.Name) == "" || !contains(styleFamilies, s.Family) || len(s.Summary) < 15 {
		return bad("invalid identity or summary")
	}
	for name, values := range map[string][]string{"keywords": s.Keywords, "avoid": s.Avoid, "best_for": s.BestFor, "beat_plan": s.BeatPlan, "reference_ids": s.ReferenceIDs} {
		min := 1
		if name == "keywords" {
			min = 6
		}
		if name == "avoid" || name == "beat_plan" {
			min = 3
		}
		if len(values) < min || len(values) > 24 {
			return bad("invalid " + name + " count")
		}
		seen := map[string]bool{}
		for _, v := range values {
			key := strings.ToLower(strings.TrimSpace(v))
			if key == "" || seen[key] {
				return bad("empty/duplicate " + name)
			}
			seen[key] = true
		}
	}
	for _, v := range []string{s.Direction.Composition, s.Direction.Typography, s.Direction.Lighting, s.Direction.Materials, s.Direction.Camera, s.Direction.Editing, s.Direction.Sound} {
		if len(strings.TrimSpace(v)) < 12 {
			return bad("incomplete direction")
		}
	}
	r := s.Render
	if !styleIDPattern.MatchString(r.BaseTemplate) || len(r.Backgrounds) == 0 || len(r.Layouts) == 0 || len(r.Transitions) == 0 {
		return bad("missing render recipe")
	}
	for _, v := range r.Backgrounds {
		if !contains(backgroundTypes, v) {
			return bad("unknown background")
		}
	}
	for _, v := range r.Layouts {
		if !contains([]string{"left", "right", "center", "asymmetric"}, v) {
			return bad("unknown layout")
		}
	}
	for _, v := range r.Transitions {
		if _, ok := transitions[v]; !ok {
			return bad("unknown transition " + v)
		}
	}
	if !contains(motionTypes, r.Motion.Name) || !contains(easeTypes, r.Motion.Ease) || !finiteNonNegative(r.Motion.Strength) || r.Motion.Strength > 3 {
		return bad("invalid motion")
	}
	for _, v := range []string{r.Theme.Background, r.Theme.Foreground, r.Theme.Accent, r.Theme.Secondary, r.Theme.Surface} {
		if !hexColor(v) {
			return bad("invalid theme color")
		}
	}
	if !contains([]string{"sans", "display"}, r.Theme.Font) {
		return bad("invalid font")
	}
	p, g := r.Typography, r.Graphics
	bounded := func(v, lo, hi float64) bool { return finiteNonNegative(v) && v >= lo && v <= hi }
	if !bounded(p.Scale, .7, 1.3) || p.WeightOffset < -250 || p.WeightOffset > 250 || !bounded(p.Tracking, 0, 12) || !bounded(p.Leading, .95, 1.4) || !bounded(g.PanelRadius, 0, 120) || !bounded(g.Opacity, 0, .3) || !bounded(g.LineWidth, .5, 5) || !contains(styleOverlays, g.Overlay) || !bounded(r.OverlapRatio, 0, .3) {
		return bad("render tokens outside supported limits")
	}
	return nil
}

func readStyles(templates map[string]TemplateData, transitions map[string]TransitionData) (map[string]StylePreset, error) {
	styles := map[string]StylePreset{}
	err := fs.WalkDir(bundled, "assets/styles", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".json" {
			return nil
		}
		raw, err := bundled.ReadFile(path)
		if err != nil {
			return err
		}
		var s StylePreset
		if err := decodeStrict(raw, &s); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if filepath.Base(path) != s.ID+".json" {
			return fmt.Errorf("style filename/ID mismatch: %s", path)
		}
		if _, ok := styles[s.ID]; ok {
			return fmt.Errorf("duplicate style %s", s.ID)
		}
		if err := validateStyle(s, transitions); err != nil {
			return err
		}
		if _, ok := templates[s.Render.BaseTemplate]; !ok {
			return fmt.Errorf("style %s base template missing", s.ID)
		}
		styles[s.ID] = s
		return nil
	})
	return styles, err
}

func applyStyle(t TemplateData, s StylePreset) TemplateData {
	t.Scenes = append([]SceneData(nil), t.Scenes...)
	t.Style = &s
	t.Theme = s.Render.Theme
	for i := range t.Scenes {
		scene := &t.Scenes[i]
		scene.Background = s.Render.Backgrounds[i%len(s.Render.Backgrounds)]
		// A background-only periodic export needs a visible procedural field.
		if t.Loop && scene.Background == "solid" {
			for _, background := range s.Render.Backgrounds {
				if background != "solid" {
					scene.Background = background
					break
				}
			}
		}
		scene.Layout = s.Render.Layouts[i%len(s.Render.Layouts)]
		scene.Motion = s.Render.Motion
		if t.Loop {
			scene.TransitionDuration = 0
			scene.Transition = "cut"
			continue
		}
		scene.Transition = s.Render.Transitions[i%len(s.Render.Transitions)]
		overlap := s.Render.OverlapRatio * scene.Duration
		if i < len(t.Scenes)-1 && overlap > t.Scenes[i+1].Duration*.3 {
			overlap = t.Scenes[i+1].Duration * .3
		}
		if scene.Transition == "cut" || i == len(t.Scenes)-1 {
			overlap = 0
		}
		scene.TransitionDuration = overlap
	}
	return t
}

func runStyle(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: video-pv-cli style list|show|validate|apply|verify [options]")
		return 2
	}
	if args[0] == "verify" {
		return runStyleVerify(args[1:], stdout, stderr)
	}
	f := flag.NewFlagSet("style "+args[0], flag.ContinueOnError)
	f.SetOutput(stderr)
	id := f.String("id", "", "style ID")
	family := f.String("family", "", "filter family")
	query := f.String("query", "", "search names, keywords and direction")
	project := f.String("project", ".", "existing project for apply")
	jsonOut := f.Bool("json", false, "JSON output")
	if code := parseFlags(f, args[1:]); code >= 0 {
		return code
	}
	templates, transitions, err := readCatalog()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	styles, err := readStyles(templates, transitions)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	switch args[0] {
	case "validate":
		fmt.Fprintf(stdout, "Valid: %d independent style JSON files, %d families\n", len(styles), len(styleFamilies))
		return 0
	case "list":
		list := []StylePreset{}
		for _, s := range styles {
			data, _ := json.Marshal(s)
			if (*family == "" || s.Family == *family) && (*query == "" || strings.Contains(strings.ToLower(string(data)), strings.ToLower(*query))) {
				list = append(list, s)
			}
		}
		sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
		if *jsonOut {
			_ = json.NewEncoder(stdout).Encode(list)
		} else {
			for _, s := range list {
				fmt.Fprintf(stdout, "%-36s %-10s %s | %s\n", s.ID, s.Family, s.Name, strings.Join(s.Keywords, ", "))
			}
		}
		return 0
	case "show":
		s, ok := styles[*id]
		if !ok {
			fmt.Fprintln(stderr, "unknown style", *id)
			return 2
		}
		data, _ := json.MarshalIndent(s, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return 0
	case "apply":
		s, ok := styles[*id]
		if !ok {
			fmt.Fprintln(stderr, "unknown style", *id)
			return 2
		}
		root, err := filepath.Abs(*project)
		if err == nil {
			err = applyProjectStyle(root, s, transitions)
		}
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, "Style applied or already current:", *id)
		return 0
	default:
		fmt.Fprintln(stderr, "unknown style command", args[0])
		return 2
	}
}

func applyProjectStyle(root string, s StylePreset, transitions map[string]TransitionData) error {
	path, err := projectFilePath(root, "template.json")
	if err != nil {
		return err
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var t TemplateData
	if err := decodeStrict(original, &t); err != nil {
		return err
	}
	styled := applyStyle(t, s)
	if err := validateTemplate(styled, transitions); err != nil {
		return err
	}
	manifestPath, err := projectFilePath(root, "pv-project.json")
	if err != nil {
		return err
	}
	_, checks := validateManifest(manifestPath, false)
	if reportHasFailures(checks) {
		return fmt.Errorf("style cannot be applied to an invalid project: %v", checks.Checks)
	}
	before, _ := json.Marshal(t)
	after, _ := json.Marshal(styled)
	if bytes.Equal(before, after) {
		return nil
	}
	// Validate before writing; keep fixed first-application backups for recovery.
	manifestRaw, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	var p ProjectFile
	if err := decodeStrict(manifestRaw, &p); err != nil {
		return err
	}
	p.Project.DurationSeconds = templateDuration(styled)
	priorShots := map[string]Shot{}
	for _, shot := range p.Shots {
		priorShots[shot.ID] = shot
	}
	p.Shots = manifestForTemplate(styled).Shots
	p.Review = Review{}
	for i := range p.Shots {
		if prior, ok := priorShots[p.Shots[i].ID]; ok {
			p.Shots[i].AssetIDs = append([]string(nil), prior.AssetIDs...)
			p.Shots[i].ClaimIDs = append([]string(nil), prior.ClaimIDs...)
			p.Shots[i].VisualDescription = prior.VisualDescription
			p.Shots[i].Beat = prior.Beat
			p.Shots[i].Role = prior.Role
			continue
		}
		p.Shots[i].AssetIDs = nil
		for _, a := range p.Assets {
			p.Shots[i].AssetIDs = append(p.Shots[i].AssetIDs, a.ID)
		}
	}
	for i := range p.Assets {
		if p.Assets[i].Role == "template-data" {
			p.Assets[i].Status = "draft"
		}
	}
	newTemplate, _ := json.MarshalIndent(styled, "", "  ")
	newManifest, _ := json.MarshalIndent(p, "", "  ")
	for _, entry := range []struct {
		name string
		data []byte
	}{{"template.json.before-style", original}, {"pv-project.json.before-style", manifestRaw}} {
		backup, err := projectOutputPath(root, entry.name)
		if err != nil {
			return err
		}
		if _, err := os.Stat(backup); os.IsNotExist(err) {
			if err := writeFileAtomic(backup, entry.data); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	if err := writeFileAtomic(path, append(newTemplate, '\n')); err != nil {
		return err
	}
	if err := writeFileAtomic(manifestPath, append(newManifest, '\n')); err != nil {
		_ = writeFileAtomic(path, original)
		return err
	}
	return nil
}
