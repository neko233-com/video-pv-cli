package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGPUExportAndFailureCleanup(t *testing.T) {
	if os.Getenv("WEB2VIDEO_GPU_INTEGRATION") != "1" {
		t.Skip("enable WEB2VIDEO_GPU_INTEGRATION=1 for real native WebGPU capture")
	}
	root := filepath.Join(t.TempDir(), "gpu-project")
	var out, errOut bytes.Buffer
	if runInit([]string{"--dir", root}, &out, &errOut) != 0 {
		t.Fatal(errOut.String())
	}
	p, err := readProject(root)
	if err != nil {
		t.Fatal(err)
	}
	p.Width = 320
	p.Height = 180
	p.FPS = 6
	p.Scenes = p.Scenes[:1]
	p.Scenes[0].Duration = 1.5
	raw, _ := json.Marshal(p)
	os.WriteFile(filepath.Join(root, "project.json"), raw, 0600)
	if err = buildProject(root); err != nil {
		t.Fatal(err)
	}
	args := []string{"--project", root, "--quality", "draft"}
	if code := runRender(args, &out, &errOut); code != 0 {
		t.Fatal(code, out.String(), errOut.String())
	}
	movie := filepath.Join(root, "renders", "master.mp4")
	validShader, _ := os.ReadFile(filepath.Join(root, "catalog.wgsl"))
	original, _ := fileSHA256(movie)
	before := snapshotFiles(t, root)
	out.Reset()
	errOut.Reset()
	if runRender(args, &out, &errOut) != 0 || !strings.Contains(out.String(), "Reused and fully verified") || !reflect.DeepEqual(before, snapshotFiles(t, root)) {
		t.Fatal("render reuse not idempotent", out.String(), errOut.String())
	}
	os.WriteFile(filepath.Join(root, "catalog.wgsl"), []byte("this is invalid WGSL"), 0600)
	out.Reset()
	errOut.Reset()
	if runRender(append(args, "--force"), &out, &errOut) == 0 {
		t.Fatal("invalid WGSL succeeded")
	}
	after, _ := fileSHA256(movie)
	if after != original {
		t.Fatal("failed render replaced good output")
	}
	os.WriteFile(filepath.Join(root, "catalog.wgsl"), validShader, 0600)
	runtimePath := filepath.Join(root, "webgpu-runtime.js")
	runtimeSource, _ := os.ReadFile(runtimePath)
	runtimeSource = bytes.Replace(runtimeSource, []byte("await navigator.gpu.requestAdapter()"), []byte("null"), 1)
	os.WriteFile(runtimePath, runtimeSource, 0600)
	out.Reset()
	errOut.Reset()
	if runRender([]string{"--project", root, "--out", "renders/no-adapter.mp4", "--quality", "draft"}, &out, &errOut) == 0 {
		t.Fatal("missing WebGPU adapter succeeded")
	}
	if _, err = os.Stat(filepath.Join(root, "renders", "no-adapter.mp4")); !os.IsNotExist(err) {
		t.Fatal("adapter failure published an MP4")
	}
	entries, _ := os.ReadDir(filepath.Join(root, ".web2video"))
	for _, entry := range entries {
		if entry.Name() == "job.lock" || entry.IsDir() && strings.HasPrefix(entry.Name(), "render-") {
			t.Fatal("failed render left temporary work", entry.Name())
		}
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	jobsRoot := filepath.Join(cache, "web2video", "jobs")
	engineEntries, _ := os.ReadDir(jobsRoot)
	for _, entry := range engineEntries {
		raw, _ := os.ReadFile(filepath.Join(jobsRoot, entry.Name(), "owner.json"))
		var owner struct {
			PID int `json:"pid"`
		}
		_ = json.Unmarshal(raw, &owner)
		if owner.PID == os.Getpid() {
			t.Fatal("owned engine profiles/history leaked", entry.Name())
		}
	}
}
