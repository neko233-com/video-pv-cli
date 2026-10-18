package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type MotionData struct {
	Name     string  `json:"name"`
	Strength float64 `json:"strength"`
	Ease     string  `json:"ease"`
}
type ThemeData struct {
	Background string `json:"background"`
	Foreground string `json:"foreground"`
	Accent     string `json:"accent"`
	Secondary  string `json:"secondary"`
	Surface    string `json:"surface"`
	Font       string `json:"font"`
}
type SceneItem struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Value string `json:"value"`
}
type SceneData struct {
	ID                 string      `json:"id"`
	Type               string      `json:"type"`
	Duration           float64     `json:"duration"`
	Title              string      `json:"title"`
	Subtitle           string      `json:"subtitle"`
	Overline           string      `json:"overline"`
	Image              string      `json:"image"`
	Background         string      `json:"background"`
	Layout             string      `json:"layout"`
	Items              []SceneItem `json:"items"`
	Motion             MotionData  `json:"motion"`
	Transition         string      `json:"transition"`
	TransitionDuration float64     `json:"transition_duration"`
}
type TemplateData struct {
	SchemaVersion string       `json:"schema_version"`
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Category      string       `json:"category"`
	Description   string       `json:"description"`
	ProjectTitle  string       `json:"project_title"`
	Width         int          `json:"width"`
	Height        int          `json:"height"`
	FPS           float64      `json:"fps"`
	Loop          bool         `json:"loop"`
	Theme         ThemeData    `json:"theme"`
	Scenes        []SceneData  `json:"scenes"`
	Style         *StylePreset `json:"style,omitempty"`
}
type TransitionData struct {
	SchemaVersion string    `json:"schema_version"`
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Category      string    `json:"category"`
	Mode          int       `json:"mode"`
	Direction     []float64 `json:"direction"`
	Strength      float64   `json:"strength"`
	Softness      float64   `json:"softness"`
	Tiles         float64   `json:"tiles"`
	Ease          string    `json:"ease"`
}

var sceneTypes = strings.Fields("title giant-type kinetic-type quote endcard logo monogram hero-device feature-focus bento spec-grid compare product-spin macro material-panels gallery zoom-gallery film-strip split-screen image-hero chapter action-panel landscape map-trace impact stat-cards countdown character-detail character-name signature profile-grid ambient orbital")
var backgroundTypes = strings.Fields("solid mesh aurora nebula grid particles rings caustics fluid prism horizon silk")
var motionTypes = strings.Fields("none push-in pull-out drift-left drift-right drift-up orbit float tilt rise stagger")
var easeTypes = strings.Fields("linear inOutCubic outExpo inOutQuint swift settle spring")

func decodeStrict(data []byte, target any) error {
	if err := rejectDuplicateKeys(data); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("expected one JSON value")
	}
	return nil
}
func contains(items []string, s string) bool {
	for _, item := range items {
		if s == item {
			return true
		}
	}
	return false
}
func hexColor(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, c := range s[1:] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}

func readCatalog() (map[string]TemplateData, map[string]TransitionData, error) {
	templates := map[string]TemplateData{}
	transitions := map[string]TransitionData{}
	err := fs.WalkDir(bundled, "assets/catalog", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		data, err := bundled.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(path, "/transitions/") {
			var item TransitionData
			if err := decodeStrict(data, &item); err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			if _, ok := transitions[item.ID]; ok {
				return fmt.Errorf("duplicate transition %s", item.ID)
			}
			transitions[item.ID] = item
		} else if strings.Contains(path, "/templates/") {
			var item TemplateData
			if err := decodeStrict(data, &item); err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			if _, ok := templates[item.ID]; ok {
				return fmt.Errorf("duplicate template %s", item.ID)
			}
			templates[item.ID] = item
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	for id, t := range transitions {
		if err := validateTransition(id, t); err != nil {
			return nil, nil, err
		}
	}
	for id, t := range templates {
		if err := validateTemplate(t, transitions); err != nil {
			return nil, nil, fmt.Errorf("template %s: %w", id, err)
		}
	}
	return templates, transitions, nil
}

func validateTransition(id string, t TransitionData) error {
	if t.SchemaVersion != "1.0" || t.ID != id || t.ID == "" || t.Name == "" || t.Mode < 0 || t.Mode > 47 || len(t.Direction) != 2 || !contains(easeTypes, t.Ease) || !finiteNonNegative(t.Strength) || t.Strength > 3 || !finitePositive(t.Softness) || t.Softness > 1 || !finitePositive(t.Tiles) || t.Tiles > 256 {
		return fmt.Errorf("invalid transition %s", id)
	}
	for _, d := range t.Direction {
		if math.IsNaN(d) || math.IsInf(d, 0) || math.Abs(d) > 1 {
			return fmt.Errorf("invalid direction for %s", id)
		}
	}
	if t.Mode >= 4 && t.Mode <= 7 && t.Direction[0] == 0 && t.Direction[1] == 0 {
		return fmt.Errorf("directional transition %s needs a direction", id)
	}
	return nil
}

func validateTemplate(t TemplateData, transitions map[string]TransitionData) error {
	if t.Style != nil {
		if err := validateStyle(*t.Style, transitions); err != nil {
			return err
		}
	}
	if t.SchemaVersion != "1.0" || t.ID == "" || t.Name == "" || !contains([]string{"game", "character", "product", "website", "keynote", "social", "brand"}, t.Category) || t.Width < 320 || t.Height < 320 || t.Width%2 != 0 || t.Height%2 != 0 || !finitePositive(t.FPS) || t.FPS > 120 || len(t.Scenes) == 0 {
		return fmt.Errorf("invalid identity, profile or scenes")
	}
	for _, c := range []string{t.Theme.Background, t.Theme.Foreground, t.Theme.Accent, t.Theme.Secondary, t.Theme.Surface} {
		if !hexColor(c) {
			return fmt.Errorf("invalid theme color %q", c)
		}
	}
	if t.Theme.Font != "sans" && t.Theme.Font != "display" {
		return fmt.Errorf("invalid theme font")
	}
	seen := map[string]bool{}
	for i, s := range t.Scenes {
		if s.ID == "" || seen[s.ID] || !contains(sceneTypes, s.Type) || !contains(backgroundTypes, s.Background) || !contains(motionTypes, s.Motion.Name) || !contains(easeTypes, s.Motion.Ease) || !contains([]string{"left", "right", "center", "asymmetric"}, s.Layout) || !finitePositive(s.Duration) || s.Duration < 0.25 || !finiteNonNegative(s.TransitionDuration) || s.TransitionDuration >= s.Duration || !finiteNonNegative(s.Motion.Strength) || s.Motion.Strength > 3 {
			return fmt.Errorf("invalid scene %s", s.ID)
		}
		seen[s.ID] = true
		if i > 0 && t.Scenes[i-1].TransitionDuration+s.TransitionDuration >= s.Duration && i < len(t.Scenes)-1 {
			return fmt.Errorf("scene %s would create an unsupported three-scene overlap", s.ID)
		}
		if i > 0 && t.Scenes[i-1].TransitionDuration >= s.Duration {
			return fmt.Errorf("incoming transition exceeds scene %s", s.ID)
		}
		if contains([]string{"material-panels", "map-trace"}, s.Type) && len(s.Items) < 4 {
			return fmt.Errorf("scene %s requires four items", s.ID)
		}
		if s.Type == "compare" && len(s.Items) < 2 {
			return fmt.Errorf("comparison scene %s requires two items", s.ID)
		}
		if contains([]string{"gallery", "zoom-gallery", "film-strip", "split-screen", "action-panel", "profile-grid", "character-detail", "character-name", "image-hero"}, s.Type) && s.Image == "" {
			return fmt.Errorf("scene %s requires a local image", s.ID)
		}
		if contains([]string{"bento", "spec-grid", "stat-cards", "profile-grid"}, s.Type) && (len(s.Items) < 1 || len(s.Items) > 8) {
			return fmt.Errorf("scene %s requires 1-8 items", s.ID)
		}
		if _, ok := transitions[s.Transition]; !ok {
			return fmt.Errorf("unknown transition %s", s.Transition)
		}
		if s.Image != "" && (filepath.IsAbs(s.Image) || strings.Contains(s.Image, ":") || strings.Contains(s.Image, "..")) {
			return fmt.Errorf("image must be a local project asset")
		}
	}
	if t.Loop && (len(t.Scenes) != 1 || !contains([]string{"ambient", "orbital"}, t.Scenes[0].Type)) {
		return fmt.Errorf("loop presets currently require one periodic ambient/orbital scene")
	}
	return nil
}

func templateDuration(t TemplateData) float64 {
	duration := 0.0
	for i, s := range t.Scenes {
		duration += s.Duration
		if i < len(t.Scenes)-1 {
			duration -= s.TransitionDuration
		}
	}
	return duration
}
func templateNeedsImage(t TemplateData) bool {
	for _, s := range t.Scenes {
		if s.Image != "" {
			return true
		}
	}
	return false
}
func manifestForTemplate(t TemplateData) ProjectFile {
	kind := "game"
	if t.Category == "character" {
		kind = "character"
	} else if t.Category == "keynote" || t.Category == "product" || t.Category == "brand" {
		kind = "product"
	}
	p := ProjectFile{SchemaVersion: "1.0", Project: ProjectInfo{Title: t.ProjectTitle, Type: kind, DurationSeconds: templateDuration(t), Width: t.Width, Height: t.Height, FPS: t.FPS, AspectRatio: "16:9", Platforms: []string{t.Category + "-pv"}, Language: "en"}, Delivery: Delivery{VideoPath: "deliverables/master.mp4", Container: "mp4", VideoCodec: "h264", PixelFormat: "yuv420p"}}
	files := []struct{ id, path, role, source string }{{"html", "pv.html", "designed-motion-source", "Shared deterministic composition entrypoint"}, {"template", "template.json", "template-data", "Independent editable template: " + t.ID}, {"transitions", "transitions.json", "transition-data", "Independent local transition parameter catalog"}, {"shader", "catalog.wgsl", "shader-source", "Original full-frame native WGSL transition compositor"}, {"runtime", "catalog-runtime.js", "shader-source", "Original data-driven scene renderer"}, {"webgpu", "webgpu-runtime.js", "shader-source", "Original native WebGPU runtime"}, {"library", "library.wgsl", "shader-source", "Original local WGSL function library"}, {"font-sans", "manrope.ttf", "font", "Manrope, SIL Open Font License; MANROPE-OFL.txt"}, {"font-display", "cinzel.ttf", "font", "Cinzel, SIL Open Font License; FONT-OFL.txt"}}
	for _, f := range files {
		p.Assets = append(p.Assets, Asset{ID: f.id, Path: f.path, Role: f.role, Source: f.source, RightsStatus: "cleared", Status: "draft"})
	}
	if templateNeedsImage(t) {
		p.Assets = append(p.Assets, Asset{ID: "art", Path: "astral-knight.png", Role: "character-reference", Source: "Generated fictional key art; see ASSET-SOURCES.md", RightsStatus: "pending", Status: "draft"})
	}
	ids := []string{}
	for _, a := range p.Assets {
		ids = append(ids, a.ID)
	}
	start := 0.0
	for i, s := range t.Scenes {
		end := start + s.Duration
		if i < len(t.Scenes)-1 {
			end -= s.TransitionDuration
		}
		p.Shots = append(p.Shots, Shot{ID: s.ID, StartSeconds: start, EndSeconds: end, Beat: s.Overline + ": " + s.Title, Role: "designed-motion", VisualDescription: s.Type + "; transition=" + s.Transition + ". Primary beat windows; transition overlays use the same declared assets.", AssetIDs: ids, Status: "draft"})
		start = end
	}
	return p
}

func runCatalog(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: video-pv-cli catalog list|show|validate|verify|sync [options]")
		return 2
	}
	action := args[0]
	f := flag.NewFlagSet("catalog "+action, flag.ContinueOnError)
	f.SetOutput(stderr)
	category := f.String("category", "", "filter template category")
	kind := f.String("kind", "templates", "templates or transitions (list)")
	id := f.String("id", "", "template or transition ID")
	jsonOut := f.Bool("json", false, "machine-readable output")
	out := f.String("out", "", "write selected data JSON")
	project := f.String("project", ".", "data project for sync")
	if action == "verify" {
		return runCatalogVerify(args[1:], stdout, stderr)
	}
	if code := parseFlags(f, args[1:]); code >= 0 {
		return code
	}
	templates, transitions, err := readCatalog()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	switch action {
	case "sync":
		root, err := filepath.Abs(*project)
		if err == nil {
			err = synchronizeDataManifest(root)
		}
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, "Synchronized profile and draft shots; previous manifest saved as pv-project.json.before-sync. Review claim-to-shot assignments again.")
		return 0
	case "validate":
		styles, e := readStyles(templates, transitions)
		if e != nil {
			fmt.Fprintln(stderr, e)
			return 1
		}
		fmt.Fprintf(stdout, "Valid: %d independent template JSON files, %d transition JSON files, %d scene types, %d backgrounds\n", len(templates), len(transitions), len(sceneTypes), len(backgroundTypes))
		fmt.Fprintf(stdout, "Styles: %d independent JSON files\n", len(styles))
		return 0
	case "list":
		if *kind == "transitions" {
			list := []TransitionData{}
			for _, tr := range transitions {
				if *category == "" || tr.Category == *category {
					list = append(list, tr)
				}
			}
			sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
			if *jsonOut {
				_ = json.NewEncoder(stdout).Encode(list)
			} else {
				for _, tr := range list {
					fmt.Fprintf(stdout, "%-26s %-12s mode=%02d  %s\n", tr.ID, tr.Category, tr.Mode, tr.Name)
				}
			}
			return 0
		}
		if *kind != "templates" {
			fmt.Fprintln(stderr, "--kind must be templates or transitions")
			return 2
		}
		list := []TemplateData{}
		for _, t := range templates {
			if *category == "" || *category == t.Category {
				list = append(list, t)
			}
		}
		sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
		if *jsonOut {
			_ = json.NewEncoder(stdout).Encode(list)
		} else {
			for _, t := range list {
				fmt.Fprintf(stdout, "%-38s %-10s %5.1fs  %s\n", t.ID, t.Category, templateDuration(t), t.Name)
			}
		}
		return 0
	case "showcase":
		if *out == "" {
			fmt.Fprintln(stderr, "showcase requires --out <new-project-directory>")
			return 2
		}
		base := templates["keynote-product-porcelain"]
		base.ID = "transition-showcase"
		base.Name = "All transition motion showcase"
		base.ProjectTitle = "TRANSITIONS"
		base.Scenes = nil
		keys := []string{}
		for key := range transitions {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		seed := templates["keynote-product-porcelain"]
		for i := 0; i <= len(keys); i++ {
			s := seed.Scenes[0]
			s.ID = fmt.Sprintf("transition-%03d", i)
			s.Duration = 1.4
			s.TransitionDuration = .6
			s.Motion = MotionData{Name: "none", Strength: 0, Ease: "linear"}
			s.Background = "solid"
			s.Layout = "center"
			if i%2 == 1 {
				s.Type = "bento"
			} else {
				s.Type = "hero-device"
			}
			s.Transition = "crossfade"
			s.Title = "COMPLETE"
			s.Overline = "ORIGINAL TRANSITION CATALOG"
			s.Subtitle = "Editable JSON / deterministic HTML / MP4"
			if i < len(keys) {
				s.Transition = keys[i]
				s.Title = keys[i]
				s.Overline = fmt.Sprintf("%02d / %02d", i+1, len(keys))
			}
			base.Scenes = append(base.Scenes, s)
		}
		root, err := filepath.Abs(*out)
		if err == nil {
			err = initializeDataProject(root, base, transitions, base.ProjectTitle)
		}
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, "Created data-driven transition showcase:", root)
		return 0
	case "show":
		var value any
		if t, ok := templates[*id]; ok {
			value = t
		} else if t, ok := transitions[*id]; ok {
			value = t
		} else {
			fmt.Fprintln(stderr, "unknown catalog ID")
			return 2
		}
		data, _ := json.MarshalIndent(value, "", "  ")
		if *out != "" {
			file, err := os.OpenFile(*out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 2
			}
			_, err = file.Write(append(data, '\n'))
			closeErr := file.Close()
			if err != nil || closeErr != nil {
				fmt.Fprintln(stderr, "write catalog data failed")
				return 2
			}
		} else {
			fmt.Fprintln(stdout, string(data))
		}
		return 0
	default:
		fmt.Fprintln(stderr, "unknown catalog action")
		return 2
	}
}
