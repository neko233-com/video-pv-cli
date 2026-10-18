package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Scene struct {
	ID         string  `json:"id"`
	Duration   float64 `json:"duration"`
	Start      float64 `json:"start"`
	Layout     string  `json:"layout"`
	Kicker     string  `json:"kicker"`
	Title      string  `json:"title"`
	Subtitle   string  `json:"subtitle"`
	Image      string  `json:"image"`
	Zoom       float64 `json:"zoom"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Transition string  `json:"transition"`
	Style      string  `json:"style,omitempty"`
}
type Project struct {
	Schema      string                     `json:"schema"`
	Title       string                     `json:"title"`
	Template    string                     `json:"template"`
	Style       string                     `json:"style"`
	Width       int                        `json:"width"`
	Height      int                        `json:"height"`
	FPS         int                        `json:"fps"`
	Loop        bool                       `json:"loop"`
	Scenes      []Scene                    `json:"scenes"`
	Duration    float64                    `json:"duration,omitempty"`
	Transitions map[string]json.RawMessage `json:"transitions,omitempty"`
	Looks       map[string]json.RawMessage `json:"looks,omitempty"`
}

var identifier = regexp.MustCompile(`^[a-z][a-z0-9-]{0,79}$`)

func decodeStrict(raw []byte, value any) error {
	if err := rejectDuplicateKeys(raw); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return fmt.Errorf("trailing JSON content")
	}
	return nil
}
func readProject(root string) (Project, error) {
	var p Project
	raw, err := os.ReadFile(filepath.Join(root, "project.json"))
	if err != nil {
		return p, err
	}
	if err = decodeStrict(raw, &p); err != nil {
		return p, err
	}
	if p.Schema != "web2video/project/1" || p.Title == "" || !identifier.MatchString(p.Style) {
		return p, fmt.Errorf("invalid project schema, title or style")
	}
	if p.Width < 64 || p.Width > 7680 || p.Height < 64 || p.Height > 7680 || p.Width%2 != 0 || p.Height%2 != 0 || p.FPS < 1 || p.FPS > 120 {
		return p, fmt.Errorf("even dimensions 64..7680 and fps 1..120 required")
	}
	if len(p.Scenes) == 0 || len(p.Scenes) > 200 {
		return p, fmt.Errorf("1..200 scenes required")
	}
	if p.Loop && len(p.Scenes) != 1 {
		return p, fmt.Errorf("seamless loop requires a single periodic scene")
	}
	seen := map[string]bool{}
	p.Duration = 0
	p.Transitions = map[string]json.RawMessage{}
	p.Looks = nil
	for i := range p.Scenes {
		s := &p.Scenes[i]
		if !identifier.MatchString(s.ID) || seen[s.ID] {
			return p, fmt.Errorf("invalid/duplicate scene id: %s", s.ID)
		}
		seen[s.ID] = true
		if !finitePositive(s.Duration) || s.Duration < 1.5 || s.Duration > 600 || !finitePositive(s.Zoom) || s.Zoom < 1 || s.Zoom > 5 || math.IsNaN(s.X) || math.IsNaN(s.Y) || s.X < 0 || s.X > 1 || s.Y < 0 || s.Y > 1 {
			return p, fmt.Errorf("invalid scene timing/crop: %s", s.ID)
		}
		if !contains([]string{"hero", "detail", "statement", "character", "ambient"}, s.Layout) {
			return p, fmt.Errorf("unknown layout: %s", s.Layout)
		}
		path, e := projectFilePath(root, s.Image)
		if e != nil {
			return p, e
		}
		info, e := os.Stat(path)
		if e != nil || !info.Mode().IsRegular() {
			return p, fmt.Errorf("missing image: %s", s.Image)
		}
		if !identifier.MatchString(s.Transition) {
			return p, fmt.Errorf("invalid transition")
		}
		transition, e := bundled.ReadFile("assets/catalog/transitions/" + s.Transition + ".json")
		if e != nil {
			return p, fmt.Errorf("unknown transition: %s", s.Transition)
		}
		p.Transitions[s.Transition] = transition
		s.Start = p.Duration
		p.Duration += s.Duration
	}
	return p, nil
}

func runLibrary(kind string, args []string, out, errOut io.Writer) int {
	f := flag.NewFlagSet(kind, flag.ContinueOnError)
	f.SetOutput(errOut)
	show := f.String("show", "", "print one JSON data file")
	if code := parseFlags(f, args); code >= 0 {
		return code
	}
	folder := "assets/" + kind
	if *show != "" {
		if !identifier.MatchString(*show) {
			return commandError(errOut, fmt.Errorf("invalid identifier"))
		}
		raw, err := bundled.ReadFile(folder + "/" + *show + ".json")
		if err != nil {
			return commandError(errOut, err)
		}
		_, _ = out.Write(raw)
		return 0
	}
	entries, err := fs.ReadDir(bundled, folder)
	if err != nil {
		return commandError(errOut, err)
	}
	for _, entry := range entries {
		fmt.Fprintln(out, strings.TrimSuffix(entry.Name(), ".json"))
	}
	return 0
}
func runInit(args []string, out, errOut io.Writer) int {
	f := flag.NewFlagSet("init", flag.ContinueOnError)
	f.SetOutput(errOut)
	dir := f.String("dir", "", "new project directory")
	name := f.String("template", "keynote-product", "authored template name")
	style := f.String("style", "", "override style preset")
	if code := parseFlags(f, args); code >= 0 {
		return code
	}
	if *dir == "" || !identifier.MatchString(*name) {
		return commandError(errOut, fmt.Errorf("--dir and valid --template required"))
	}
	target, err := filepath.Abs(*dir)
	if err != nil {
		return commandError(errOut, err)
	}
	if _, err = os.Lstat(target); !os.IsNotExist(err) {
		return commandError(errOut, fmt.Errorf("init requires a new directory: %s", target))
	}
	raw, err := bundled.ReadFile("assets/templates/" + *name + ".json")
	if err != nil {
		return commandError(errOut, err)
	}
	var p Project
	if err = json.Unmarshal(raw, &p); err != nil {
		return commandError(errOut, err)
	}
	if *style != "" {
		p.Style = *style
	}
	parent := filepath.Dir(target)
	if err = os.MkdirAll(parent, 0755); err != nil {
		return commandError(errOut, err)
	}
	stage, err := os.MkdirTemp(parent, ".web2video-init-")
	if err != nil {
		return commandError(errOut, err)
	}
	defer os.RemoveAll(stage)
	sources := map[string]string{"assets/product.png": "assets/shared/product.png", "assets/world.png": "assets/shared/world.png", "assets/character.png": "assets/character/astral-knight.png"}
	for _, s := range p.Scenes {
		source, ok := sources[s.Image]
		if !ok {
			return commandError(errOut, fmt.Errorf("unknown embedded image"))
		}
		data, e := bundled.ReadFile(source)
		if e != nil {
			return commandError(errOut, e)
		}
		if e = writeFileAtomic(filepath.Join(stage, filepath.FromSlash(s.Image)), data); e != nil {
			return commandError(errOut, e)
		}
	}
	raw, _ = json.MarshalIndent(p, "", "  ")
	if err = writeFileAtomic(filepath.Join(stage, "project.json"), append(raw, '\n')); err != nil {
		return commandError(errOut, err)
	}
	if err = writeFileAtomic(filepath.Join(stage, ".gitignore"), []byte(".web2video/\n.hyperframes/\nnode_modules/\nrenders/\n*.[mM][pP]4\n")); err != nil {
		return commandError(errOut, err)
	}
	if err = buildProject(stage); err != nil {
		return commandError(errOut, err)
	}
	if err = os.Rename(stage, target); err != nil {
		return commandError(errOut, err)
	}
	fmt.Fprintln(out, "Created:", target)
	return 0
}
func runBuild(args []string, out, errOut io.Writer) int {
	f := flag.NewFlagSet("build", flag.ContinueOnError)
	f.SetOutput(errOut)
	root := f.String("project", ".", "project directory")
	if code := parseFlags(f, args); code >= 0 {
		return code
	}
	if err := buildProject(*root); err != nil {
		return commandError(errOut, err)
	}
	fmt.Fprintln(out, "Built native HyperFrames composition:", filepath.Join(*root, "index.html"))
	return 0
}
func buildProject(root string) error {
	p, err := readProject(root)
	if err != nil {
		return err
	}
	styleRaw, err := bundled.ReadFile("assets/styles/" + p.Style + ".json")
	if err != nil {
		return err
	}
	var style struct {
		Palette struct {
			Accent string `json:"accent"`
		}
	}
	if err = json.Unmarshal(styleRaw, &style); err != nil {
		return err
	}
	p.Looks = map[string]json.RawMessage{p.Style: styleRaw}
	for _, scene := range p.Scenes {
		if scene.Style != "" {
			if !identifier.MatchString(scene.Style) {
				return fmt.Errorf("invalid scene style")
			}
			look, e := bundled.ReadFile("assets/styles/" + scene.Style + ".json")
			if e != nil {
				return e
			}
			p.Looks[scene.Style] = look
		}
	}
	raw, _ := json.Marshal(p)
	source, _ := bundled.ReadFile("assets/runtime/index.html")
	tpl, err := template.New("index").Parse(string(source))
	if err != nil {
		return err
	}
	var html bytes.Buffer
	view := struct {
		Project
		Accent    string
		JSON      template.JS
		Script    template.JS
		StyleJSON template.JS
	}{p, style.Palette.Accent, template.JS(raw), "", template.JS(styleRaw)}
	js, err := bundled.ReadFile("assets/runtime/composition.js")
	if err != nil {
		return err
	}
	view.Script = template.JS(js)
	if err = tpl.Execute(&html, view); err != nil {
		return err
	}
	files := map[string][]byte{"index.html": stableElementIDs(html.Bytes()), "style.json": styleRaw}
	for _, name := range []string{"composition.js", "composition.css"} {
		files[name], err = bundled.ReadFile("assets/runtime/" + name)
		if err != nil {
			return err
		}
	}
	for _, name := range []string{"catalog.wgsl", "library.wgsl", "webgpu-runtime.js"} {
		files[name], err = bundled.ReadFile("assets/shaders/" + name)
		if err != nil {
			return err
		}
	}
	for _, name := range []string{"manrope.ttf", "cinzel.ttf", "MANROPE-OFL.txt", "FONT-OFL.txt"} {
		files[name], err = bundled.ReadFile("assets/shared/" + name)
		if err != nil {
			return err
		}
	}
	runtimeDir, err := runtimeRoot()
	if err != nil {
		return err
	}
	files["gsap.min.js"], err = os.ReadFile(filepath.Join(runtimeDir, "node_modules", "gsap", "dist", "gsap.min.js"))
	if err != nil {
		return fmt.Errorf("run web2video setup first: %w", err)
	}
	for name, data := range files {
		old, e := os.ReadFile(filepath.Join(root, name))
		if e == nil && bytes.Equal(old, data) {
			continue
		}
		if err = writeFileAtomic(filepath.Join(root, name), data); err != nil {
			return err
		}
	}
	return nil
}

// Studio otherwise stamps random IDs into the source on first preview.
// Stamp only markup, before the first script, so JS comparisons stay untouched.
func stableElementIDs(raw []byte) []byte {
	end := bytes.Index(raw, []byte("<script"))
	if end < 0 {
		end = len(raw)
	}
	counter := 0
	tags := regexp.MustCompile(`<([A-Za-z][A-Za-z0-9:-]*)(\s[^<>]*?)?>`)
	prefix := tags.ReplaceAllFunc(raw[:end], func(tag []byte) []byte {
		if bytes.Contains(tag, []byte("data-hf-id=")) {
			return tag
		}
		counter++
		space := bytes.IndexAny(tag, " \t\r\n>")
		id := []byte(` data-hf-id="hf-w2v-` + strconv.Itoa(counter) + `"`)
		result := append([]byte(nil), tag[:space]...)
		result = append(result, id...)
		return append(result, tag[space:]...)
	})
	return append(prefix, raw[end:]...)
}

// Static policy is an early gate; the browser guard enforces canvas API use at runtime.
func validateComposition(root string) error {
	raw, err := os.ReadFile(filepath.Join(root, "index.html"))
	if err != nil {
		return fmt.Errorf("missing index.html; run web2video build: %w", err)
	}
	if !bytes.Contains(raw, []byte("data-composition-id=")) {
		return fmt.Errorf("index.html is not a HyperFrames composition")
	}
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			if path != root && ownedDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not accepted in render sources: %s", path)
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".js" && ext != ".html" {
			return nil
		}
		if d.Name() == "composition.js" || d.Name() == "gsap.min.js" {
			return nil
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		for _, bad := range []string{"getContext('webgl", "getContext(\"webgl", "shader-transition", "page-side-compositing"} {
			if bytes.Contains(b, []byte(bad)) {
				return fmt.Errorf("WebGL component forbidden: %s", path)
			}
		}
		return nil
	})
}
func ownedDir(name string) bool {
	return contains([]string{".git", ".web2video", ".hyperframes", "node_modules", "renders", "out", "deliverables", "qa"}, name)
}
func sourceFingerprint(root string) (string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			if path != root && ownedDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("source symlink: %s", path)
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths)
	var b strings.Builder
	for _, path := range paths {
		rel, _ := filepath.Rel(root, path)
		sum, e := fileSHA256(path)
		if e != nil {
			return "", e
		}
		fmt.Fprintf(&b, "%s %s\n", filepath.ToSlash(rel), sum)
	}
	return hashBytes([]byte(b.String())), nil
}

func validateCompiledProject(root string, p Project) error {
	raw, err := os.ReadFile(filepath.Join(root, "index.html"))
	if err != nil {
		return err
	}
	match := regexp.MustCompile(`(?s)<script\b[^>]*id="project-data"[^>]*>(.*?)</script>`).FindSubmatch(raw)
	if len(match) != 2 {
		return fmt.Errorf("project.json exists but compiled project-data is missing; run web2video build")
	}
	var compiled Project
	if err = json.Unmarshal(match[1], &compiled); err != nil {
		return err
	}
	compiled.Looks = nil
	p.Looks = nil
	actual, _ := json.Marshal(compiled)
	expected, _ := json.Marshal(p)
	if !bytes.Equal(actual, expected) {
		return fmt.Errorf("project.json differs from compiled HTML; run web2video build before rendering")
	}
	return nil
}
