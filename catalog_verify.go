package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

type AtlasCase struct {
	ID         string       `json:"id"`
	Template   TemplateData `json:"template"`
	Time       float64      `json:"time"`
	Transition string       `json:"transition,omitempty"`
	Phase      float64      `json:"phase"`
}
type AtlasData struct {
	Kind   string      `json:"kind"`
	ID     string      `json:"id"`
	Width  int         `json:"width"`
	Height int         `json:"height"`
	FPS    float64     `json:"fps"`
	Cases  []AtlasCase `json:"cases"`
}
type CatalogEvidence struct {
	Schema         string            `json:"schema"`
	Passed         bool              `json:"passed"`
	Templates      int               `json:"templates"`
	Transitions    int               `json:"transitions"`
	CaseCount      int               `json:"case_count"`
	CompletedCases int               `json:"completed_cases"`
	Mode           string            `json:"mode"`
	Batches        []string          `json:"batches"`
	Error          string            `json:"error,omitempty"`
	PixelChecks    int               `json:"pixel_checks"`
	EndpointChecks int               `json:"endpoint_checks"`
	LoopChecks     int               `json:"loop_checks"`
	Limited        bool              `json:"limited"`
	Platform       string            `json:"platform"`
	GoVersion      string            `json:"go_version"`
	CatalogSHA256  string            `json:"catalog_sha256"`
	SourceSHA256   map[string]string `json:"source_sha256"`
	Backend        string            `json:"backend"`
	Raster         string            `json:"browser_raster"`
}

func catalogCases(templates map[string]TemplateData, transitions map[string]TransitionData, mode, category string) []AtlasCase {
	ids := []string{}
	for id, t := range templates {
		if category == "" || t.Category == category {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	cases := []AtlasCase{}
	for _, id := range ids {
		t := templates[id]
		if mode == "deep" {
			start := 0.0
			for _, s := range t.Scenes {
				for pi, phase := range []float64{.05, .5, .95} {
					cases = append(cases, AtlasCase{ID: fmt.Sprintf("%s/%s/%d", id, s.ID, pi), Template: t, Time: start + s.Duration*phase})
				}
				start += s.Duration - s.TransitionDuration
			}
		} else {
			for pi, phase := range []float64{.03, .5, .97} {
				cases = append(cases, AtlasCase{ID: fmt.Sprintf("%s/%d", id, pi), Template: t, Time: templateDuration(t) * phase})
			}
		}
		if t.Loop {
			cases = append(cases, AtlasCase{ID: id + "/loop-start", Template: t, Time: 0}, AtlasCase{ID: id + "/loop-end", Template: t, Time: templateDuration(t)})
		}
	}
	if category == "" {
		keys := []string{}
		for id := range transitions {
			keys = append(keys, id)
		}
		sort.Strings(keys)
		base := templates["keynote-product-porcelain"]
		base.Scenes = append([]SceneData(nil), base.Scenes[:2]...)
		base.Scenes[0].Title = "BEFORE"
		base.Scenes[1].Title = "AFTER"
		base.Scenes[0].Type = "hero-device"
		base.Scenes[1].Type = "bento"
		for _, id := range keys {
			for pi, phase := range []float64{0, .25, .5, .75, 1} {
				cases = append(cases, AtlasCase{ID: fmt.Sprintf("transition/%s/%d", id, pi), Template: base, Transition: id, Phase: phase})
			}
		}
	}
	return cases
}

func runCatalogVerify(args []string, stdout, stderr io.Writer) int {
	f := flag.NewFlagSet("catalog verify", flag.ContinueOnError)
	f.SetOutput(stderr)
	runtimeProject := f.String("runtime-project", "", "prepared project; default local dependencies or current-user runtime")
	out := f.String("out", "", "stable local evidence directory (default work/catalog-validation)")
	mode := f.String("mode", "deep", "deep: every scene at three times; smoke: three times per template")
	category := f.String("category", "", "optional template category; empty also verifies all transitions")
	batchSize := f.Int("batch-size", 32, "cases per independent MP4/contact sheet batch (1-64)")
	limit := f.Int("limit", 0, "development-only case limit; 0 verifies all cases")
	resume := f.Bool("resume", true, "revalidate and reuse matching completed evidence; preserve failed attempts")
	raster := f.String("raster", "gpu", "gpu or software browser rasterization; native WebGPU remains required")
	if code := parseFlags(f, args); code >= 0 {
		return code
	}
	if (*mode != "deep" && *mode != "smoke") || *batchSize < 1 || *batchSize > 64 || *limit < 0 {
		fmt.Fprintln(stderr, "invalid catalog verification options")
		return 2
	}
	resolvedRuntime, err := preparedRuntimeRoot(*runtimeProject, ".")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	*runtimeProject = resolvedRuntime
	templates, transitions, err := readCatalog()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	cases := catalogCases(templates, transitions, *mode, *category)
	if *limit > 0 && len(cases) > *limit {
		cases = cases[:*limit]
	}
	if len(cases) == 0 {
		fmt.Fprintln(stderr, "no verification cases")
		return 2
	}
	if *out == "" {
		*out = filepath.Join("work", "catalog-validation")
	}
	root, err := filepath.Abs(*out)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	_, statErr := os.Stat(root)
	exists := statErr == nil
	if exists && !*resume {
		fmt.Fprintln(stderr, "evidence directory already exists; choose a new --out")
		return 2
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	evidence := CatalogEvidence{Schema: "video-pv-cli/catalog-verification/1", Templates: len(templates), Transitions: len(transitions), CaseCount: len(cases), Mode: *mode, Limited: *limit > 0 || *category != "", Platform: runtime.GOOS + "/" + runtime.GOARCH, GoVersion: runtime.Version(), SourceSHA256: map[string]string{}}
	evidence.Backend = "webgpu"
	evidence.Raster = *raster
	catalogHash := sha256.New()
	err = fs.WalkDir(bundled, "assets/catalog", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			data, e := bundled.ReadFile(path)
			if e != nil {
				return e
			}
			catalogHash.Write([]byte(path + "\x00"))
			catalogHash.Write(data)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	evidence.CatalogSHA256 = hex.EncodeToString(catalogHash.Sum(nil))
	for _, path := range []string{"assets/starter/pv.html", "assets/starter/catalog-runtime.js", "assets/starter/catalog.wgsl", "assets/starter/webgpu-runtime.js", "assets/starter/library.wgsl"} {
		data, _ := bundled.ReadFile(path)
		h := sha256.Sum256(data)
		evidence.SourceSHA256[path] = hex.EncodeToString(h[:])
	}
	pixels := newCatalogPixelGate()
	if *resume && exists {
		previous, err := resumeCatalogEvidence(root, evidence, cases, pixels, stderr)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		evidence = previous
		evidence.Passed = false
		evidence.Error = ""
		fmt.Fprintf(stdout, "Revalidated %d completed cases; continuing locally.\n", evidence.CompletedCases)
	}
	var saveError error
	save := func() {
		data, _ := json.MarshalIndent(evidence, "", "  ")
		if err := writeFileAtomic(filepath.Join(root, "catalog-report.json"), append(data, '\n')); err != nil {
			saveError = err
			fmt.Fprintln(stderr, err)
		}
	}
	save()
	if saveError != nil {
		return 1
	}
	for start := evidence.CompletedCases; start < len(cases); start += *batchSize {
		end := start + *batchSize
		if end > len(cases) {
			end = len(cases)
		}
		batch := cases[start:end]
		baseName := fmt.Sprintf("batch-%03d", len(evidence.Batches)+1)
		name, project := baseName, filepath.Join(root, baseName)
		for attempt := 2; ; attempt++ {
			if _, err := os.Lstat(project); os.IsNotExist(err) {
				break
			} else if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			name = fmt.Sprintf("%s-attempt-%03d", baseName, attempt)
			project = filepath.Join(root, name)
		}
		// A regular initialized project provides the same shared source files/assets.
		seed := templates["character-reveal-midnight"]
		if err := initializeDataProject(project, seed, transitions, seed.ProjectTitle); err != nil {
			evidence.Error = err.Error()
			save()
			fmt.Fprintln(stderr, err)
			return 1
		}
		atlas := AtlasData{Kind: "atlas", ID: name, Width: 1280, Height: 720, FPS: 6, Cases: batch}
		data, _ := json.MarshalIndent(atlas, "", "  ")
		if err := os.WriteFile(filepath.Join(project, "template.json"), append(data, '\n'), 0644); err != nil {
			evidence.Error = err.Error()
			save()
			return 1
		}
		manifest := manifestForTemplate(seed)
		manifest.Project.Title = "Catalog verification " + name
		manifest.Project.Width = 1280
		manifest.Project.Height = 720
		manifest.Project.FPS = 6
		manifest.Project.DurationSeconds = float64(len(batch)) / 6
		manifest.Shots = nil
		assetIDs := []string{}
		for i := range manifest.Assets {
			assetIDs = append(assetIDs, manifest.Assets[i].ID)
			if manifest.Assets[i].ID == "template" {
				manifest.Assets[i].Role = "atlas-data"
			}
		}
		for i, c := range batch {
			manifest.Shots = append(manifest.Shots, Shot{ID: fmt.Sprintf("case-%04d", i), StartSeconds: float64(i) / 6, EndSeconds: float64(i+1) / 6, Beat: c.ID, Role: "test-case", VisualDescription: "Explicit Windows preview/test case; not a production film or approval", AssetIDs: assetIDs, Status: "draft"})
		}
		data, _ = json.MarshalIndent(manifest, "", "  ")
		if err := os.WriteFile(filepath.Join(project, "pv-project.json"), append(data, '\n'), 0644); err != nil {
			evidence.Error = err.Error()
			save()
			return 1
		}
		at := []string{}
		for i := range batch {
			at = append(at, strconv.FormatFloat(float64(i)/6, 'f', 9, 64))
		}
		fmt.Fprintf(stdout, "%s: cases %d-%d / %d\n", name, start+1, end, len(cases))
		code := runRender([]string{"--project", project, "--renderer-project", *runtimeProject, "--raster", *raster, "--verify-at", strings.Join(at, ","), "--stills-every", "0.166666667", "--auto-install=false", "--timeout", "15m"}, stdout, stderr)
		if code != 0 {
			evidence.Error = "failed " + name
			save()
			return code
		}
		if err := pixels.checkBatch(project, batch); err != nil {
			evidence.Error = name + ": " + err.Error()
			save()
			fmt.Fprintln(stderr, evidence.Error)
			return 1
		}
		evidence.PixelChecks = pixels.frames
		evidence.EndpointChecks = pixels.endpoints
		evidence.LoopChecks = pixels.loops
		evidence.CompletedCases = end
		evidence.Batches = append(evidence.Batches, name)
		save()
		if saveError != nil {
			return 1
		}
	}
	evidence.Passed = true
	save()
	if saveError != nil {
		return 1
	}
	fmt.Fprintf(stdout, "Verified %d cases across %d local MP4 batches. Evidence: %s\n", len(cases), len(evidence.Batches), root)
	return 0
}
