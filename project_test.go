package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func snapshotFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			sum, e := fileSHA256(path)
			if e != nil {
				return e
			}
			rel, _ := filepath.Rel(root, path)
			result[rel] = sum
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func TestLibraryData(t *testing.T) {
	styles, _ := fs.ReadDir(bundled, "assets/styles")
	if len(styles) != 100 {
		t.Fatal(len(styles))
	}
	for _, entry := range styles {
		raw, _ := bundled.ReadFile("assets/styles/" + entry.Name())
		var s struct {
			Schema, ID string
			Keywords   []string
		}
		if rejectDuplicateKeys(raw) != nil || json.Unmarshal(raw, &s) != nil || s.Schema != "web2video/style/1" || s.ID+".json" != entry.Name() || len(s.Keywords) < 4 {
			t.Fatal(entry.Name())
		}
	}
	transitions, _ := fs.ReadDir(bundled, "assets/catalog/transitions")
	if len(transitions) != 76 {
		t.Fatal(len(transitions))
	}
	for _, entry := range transitions {
		raw, _ := bundled.ReadFile("assets/catalog/transitions/" + entry.Name())
		var v struct {
			ID        string
			Mode      int
			Direction []float64
		}
		if json.Unmarshal(raw, &v) != nil || v.ID+".json" != entry.Name() || v.Mode < 0 || v.Mode > 47 || len(v.Direction) != 2 {
			t.Fatal(entry.Name())
		}
	}
}
func TestProjectBuildIdempotent(t *testing.T) {
	if engineReadyMustMissing() {
		t.Skip("run setup for project integration")
	}
	for _, name := range []string{"keynote-product", "anime-character", "game-trailer", "website-background"} {
		t.Run(name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), name)
			var out, errOut bytes.Buffer
			if code := runInit([]string{"--dir", root, "--template", name}, &out, &errOut); code != 0 {
				t.Fatal(code, errOut.String())
			}
			first := snapshotFiles(t, root)
			if err := buildProject(root); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(first, snapshotFiles(t, root)) {
				t.Fatal("build changed matching source")
			}
			if err := validateComposition(root); err != nil {
				t.Fatal(err)
			}
			if runInit([]string{"--dir", root}, &out, &errOut) == 0 {
				t.Fatal("overwrote existing project")
			}
			p, err := readProject(root)
			if err != nil || p.Duration < 6 {
				t.Fatal(err)
			}
			rawProject, _ := json.Marshal(p)
			if !bytes.Contains(rawProject, []byte(`"start":0`)) {
				t.Fatal("first scene lost its zero start time")
			}
			if err = validateCompiledProject(root, p); err != nil {
				t.Fatal(err)
			}
			stale := p
			stale.Title += " changed"
			if validateCompiledProject(root, stale) == nil {
				t.Fatal("stale compiled title accepted")
			}
			p.Scenes[0].Image = "../escape.png"
			raw, _ := json.Marshal(p)
			os.WriteFile(filepath.Join(root, "project.json"), raw, 0600)
			if _, err = readProject(root); err == nil {
				t.Fatal("accepted escaped source")
			}
		})
	}
}

func TestPixelGate(t *testing.T) {
	if hasFrameContent(bytes.Repeat([]byte{3}, 96*54)) {
		t.Fatal("flat frame accepted")
	}
	if !hasFrameContent([]byte{0, 25, 255}) {
		t.Fatal("content rejected")
	}
}

func TestQARejectsUnrelatedCheckout(t *testing.T) {
	root := t.TempDir()
	if validateQACheckout(root) == nil {
		t.Fatal("empty project accepted")
	}
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/other\n\ngo 1.27\n"), 0600)
	if validateQACheckout(root) == nil {
		t.Fatal("unrelated Go tests accepted")
	}
	if err := validateQACheckout("."); err != nil {
		t.Fatal(err)
	}
}

func TestQAOwnsProcessCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := newQACommand(ctx)
	if cmd.Cancel == nil {
		t.Fatal("QA command must use CommandContext before process-tree cancellation is configured")
	}
	configureProcess(cmd)
	if !reflect.DeepEqual(cmd.Args, []string{"go", "test", "-count=1", "./..."}) {
		t.Fatal(cmd.Args)
	}
}
func engineReadyMustMissing() bool {
	root, err := runtimeRoot()
	return err != nil || engineReady(root) != nil
}
func TestNegativeCLIAndPaths(t *testing.T) {
	for _, args := range [][]string{{"unknown"}, {"init"}, {"render", "--quality", "wrong"}, {"styles", "--show", "../../x"}, {"build", "--unknown"}} {
		var out, errOut bytes.Buffer
		if realMain(args, &out, &errOut) == 0 {
			t.Fatal(args)
		}
	}
	root := t.TempDir()
	for _, name := range []string{"../evil", "../../x", "."} {
		if _, err := projectFilePath(root, name); err == nil {
			t.Fatal(name)
		}
	}
	if decodeStrict([]byte(`{"a":1,"a":2}`), new(any)) == nil {
		t.Fatal("duplicate JSON accepted")
	}
	if decodeStrict([]byte(`{} {}`), new(any)) == nil {
		t.Fatal("trailing JSON accepted")
	}
	if err := writeFileAtomic(filepath.Join(root, "test"), []byte("a")); err != nil {
		t.Fatal(err)
	}
	if err := protectOutput(filepath.Join(root, "test"), filepath.Join(root, "test")); err == nil {
		t.Fatal("source overwrite accepted")
	}
}
func TestSourceFingerprintIncludesFootage(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "footage.mp4"), []byte("first"), 0600)
	before, _ := sourceFingerprint(root)
	os.WriteFile(filepath.Join(root, "footage.mp4"), []byte("second"), 0600)
	after, _ := sourceFingerprint(root)
	if before == after {
		t.Fatal("input MP4 excluded from hash")
	}
	os.MkdirAll(filepath.Join(root, "renders"), 0755)
	os.WriteFile(filepath.Join(root, "renders", "movie.mp4"), []byte("output"), 0600)
	again, _ := sourceFingerprint(root)
	if after != again {
		t.Fatal("render output entered input hash")
	}
}
func TestGPUContract(t *testing.T) {
	js, _ := bundled.ReadFile("assets/runtime/composition.js")
	for _, required := range []string{"window.__timelines.main=tl", "hf-seek", "event.detail.waitUntil(job)", "gpu.flush()", "WEB2VIDEO_WEBGL_FORBIDDEN", "config.loop"} {
		if !strings.Contains(string(js), required) {
			t.Fatal(required)
		}
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "index.html"), []byte(`<div data-composition-id="main"></div><script>canvas.getContext('webgl2')</script>`), 0600)
	if validateComposition(root) == nil {
		t.Fatal("WebGL accepted")
	}
}
func TestJobLock(t *testing.T) {
	root := t.TempDir()
	unlock, err := acquireJob(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = acquireJob(root); err == nil {
		t.Fatal("parallel job accepted")
	}
	unlock()
	unlock2, err := acquireJob(root)
	if err != nil {
		t.Fatal(err)
	}
	unlock2()
	if _, err = os.Stat(filepath.Join(root, ".web2video", "job.lock")); !os.IsNotExist(err) {
		t.Fatal("lock left behind")
	}
}

func TestAllStyleBuilds(t *testing.T) {
	if engineReadyMustMissing() {
		t.Skip("run setup")
	}
	root := filepath.Join(t.TempDir(), "style-matrix")
	var out, errOut bytes.Buffer
	if runInit([]string{"--dir", root}, &out, &errOut) != 0 {
		t.Fatal(errOut.String())
	}
	p, err := readProject(root)
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := fs.ReadDir(bundled, "assets/styles")
	for _, entry := range entries {
		p.Style = strings.TrimSuffix(entry.Name(), ".json")
		raw, _ := json.Marshal(p)
		if err = os.WriteFile(filepath.Join(root, "project.json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
		if err = buildProject(root); err != nil {
			t.Fatalf("%s: %v", p.Style, err)
		}
		html, _ := os.ReadFile(filepath.Join(root, "index.html"))
		if !bytes.Contains(html, []byte(p.Style)) || !bytes.Contains(html, []byte("style-data")) {
			t.Fatal(p.Style)
		}
	}
}
func TestWindowsBridgeGuard(t *testing.T) {
	root, err := runtimeRoot()
	if err != nil {
		t.Skip(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "node_modules", "hyperframes", "dist", "chunk-VKFYI7AQ.js"))
	if err != nil {
		t.Skip(err)
	}
	patched, err := bridgeEngineBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	again, err := bridgeEngineBytes(patched)
	if err != nil || !bytes.Equal(patched, again) {
		t.Fatal("bridge not idempotent", err)
	}
	changed := append(append([]byte(nil), patched...), 10)
	if _, err = bridgeEngineBytes(changed); err == nil {
		t.Fatal("modified dependency accepted")
	}
}
