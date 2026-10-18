package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestFullTimelineVerificationAndExplicitSampling(t *testing.T) {
	p := ProjectInfo{DurationSeconds: 16.8, FPS: 30}
	times, full, err := verificationTimes(p, "")
	if err != nil || !full || len(times) != 504 || times[0] != "0.000000000000" || times[503] != "16.766666666667" {
		t.Fatalf("not full coverage: %d %v %v", len(times), full, err)
	}
	for _, input := range []string{"NaN", "-1", "16.8", "0,0", "0.01", "Inf", ""} {
		if input == "" {
			continue
		}
		if _, _, err := verificationTimes(p, input); err == nil {
			t.Fatalf("invalid times accepted: %s", input)
		}
	}
	times, full, err = verificationTimes(p, "0,1,2")
	if err != nil || full || len(times) != 3 {
		t.Fatal("partial coverage was misreported")
	}
}

func TestCaptureBridgeIsBoundedIdempotentAndRejectsUnknownSources(t *testing.T) {
	original := []byte(captureCallback + "\n" + captureRender)
	hash := sha256.Sum256(original)
	expected := hex.EncodeToString(hash[:])
	patched, err := bridgeCaptureSource(original, expected)
	if err != nil || !bytes.Contains(patched, []byte("await globalThis.__PV_WEBGPU_FLUSH()")) {
		t.Fatal("missing GPU barrier", err)
	}
	again, err := bridgeCaptureSource(patched, expected)
	if err != nil || !bytes.Equal(patched, again) {
		t.Fatal("bridge is not idempotent", err)
	}
	if _, err := bridgeCaptureSource(append(patched, 'x'), expected); err == nil {
		t.Fatal("modified renderer accepted")
	}
	if _, err := bridgeCaptureSource([]byte("unrecognized"), expected); err == nil {
		t.Fatal("unknown renderer accepted")
	}
}

func TestInitIsSelfContainedAndPreservesExistingProjects(t *testing.T) {
	target := filepath.Join(t.TempDir(), "project with spaces & symbols")
	if err := initializeProject(target, "character", "Character PV"); err != nil {
		t.Fatal(err)
	}
	p, r := validateManifest(filepath.Join(target, "pv-project.json"), false)
	if reportHasFailure(r) || p.Project.Type != "character" || p.Project.Title != "Character PV" {
		t.Fatalf("bad initialized manifest: %+v", r)
	}
	for _, path := range []string{"package-lock.json", "catalog.wgsl", "library.wgsl", "webgpu-runtime.js", "cinzel.ttf", "FONT-OFL.txt"} {
		if _, err := os.Stat(filepath.Join(target, path)); err != nil {
			t.Fatal(err)
		}
	}
	if err := initializeProject(target, "product", "Replacement"); err == nil {
		t.Fatal("overwrote existing project")
	}
	p, _ = validateManifest(filepath.Join(target, "pv-project.json"), false)
	if p.Project.Title != "Character PV" {
		t.Fatal("existing source was changed")
	}
}

func TestSkillInstallAndArchiveAreUsableModules(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "pv-production")
	var output bytes.Buffer
	if err := installSkill(target, false, &output); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"go.mod", "main.go", "pipeline.go", "workspace.go", "SKILL.md", "references/shader-production.md", "assets/starter/package-lock.json"} {
		if _, err := os.Stat(filepath.Join(target, path)); err != nil {
			t.Fatal(err)
		}
	}
	marker := filepath.Join(target, "custom-note")
	_ = os.WriteFile(marker, []byte("keep"), 0600)
	if err := installSkill(target, false, &output); err == nil {
		t.Fatal("existing install was overwritten")
	}
	if err := installSkill(target, true, &output); err != nil {
		t.Fatal(err)
	}
	backups, _ := filepath.Glob(filepath.Join(root, ".pv-production.backup-*"))
	if len(backups) != 1 {
		t.Fatalf("expected one hidden backup: %v", backups)
	}
	if data, err := os.ReadFile(filepath.Join(backups[0], "custom-note")); err != nil || string(data) != "keep" {
		t.Fatal("custom data not preserved")
	}
	archive := filepath.Join(root, "skill.zip")
	if err := packSkill(archive); err != nil {
		t.Fatal(err)
	}
	if err := packSkill(archive); err == nil {
		t.Fatal("existing archive overwritten")
	}
	reader, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	found := map[string]bool{}
	for _, file := range reader.File {
		found[file.Name] = true
		if strings.Contains(file.Name, "node_modules") || strings.Contains(file.Name, "work/") {
			t.Fatal("unexpected development artifacts in package")
		}
	}
	if !found["pv-production/SKILL.md"] || !found["pv-production/assets/starter/catalog.wgsl"] {
		t.Fatal("archive missing skill or shader")
	}
}

func TestRendererProtocolFailsClosed(t *testing.T) {
	good := `{"schema":"multiplane/1","type":"result","command":"render","ok":true,"data":{}}`
	for name, input := range map[string]string{"invalid": "not json", "missing": "", "wrong-command": strings.Replace(good, `"render"`, `"verify"`, 1), "wrong-schema": strings.Replace(good, "multiplane/1", "multiplane/2", 1), "after-result": good + "\n" + good} {
		t.Run(name, func(t *testing.T) {
			var step RenderStep
			if err := parseRenderEvents([]byte(input), "render", &step); err == nil {
				t.Fatal("invalid protocol accepted")
			}
		})
	}
	var step RenderStep
	if err := parseRenderEvents([]byte(`{"schema":"multiplane/1","type":"warning","code":"gpu_raster_jitter"}`+"\n"+good), "render", &step); err != nil || !step.Result.OK || len(step.Notices) != 1 {
		t.Fatalf("valid protocol: %+v %v", step, err)
	}
}

func TestInspectSpecMustMatchDeclaredMaster(t *testing.T) {
	p := validManifest().Project
	good := fmt.Sprintf(`{"stage":{"width":%d,"height":%d},"duration":%g,"fps":%g,"meta":{"gpu":{"backend":"webgpu","shaderLanguage":"wgsl"}}}`, p.Width, p.Height, p.DurationSeconds, p.FPS)
	if err := compareShotSpec([]byte(good), p); err != nil {
		t.Fatal(err)
	}
	if err := compareShotSpec([]byte(strings.Replace(good, `"duration":5`, `"duration":1`, 1)), p); err == nil {
		t.Fatal("duration mismatch accepted")
	}
	if err := compareShotSpec([]byte(`{}`), p); err == nil {
		t.Fatal("missing spec accepted")
	}
	if err := compareShotSpec([]byte(strings.Replace(good, "webgpu", "unsupported", 1)), p); err == nil {
		t.Fatal("unsupported backend accepted")
	}
}

func TestReportsCannotOverwriteInputs(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "source.txt"), []byte("source"), 0600)
	path := writeTestManifest(t, root, validManifest())
	before, _ := os.ReadFile(path)
	var stdout, stderr bytes.Buffer
	if code := realMain([]string{"check", "--manifest", path, "--report", path}, &stdout, &stderr); code != 2 {
		t.Fatalf("should reject overwrite: %d %s", code, stderr.String())
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("manifest changed")
	}
	alias := filepath.Join(root, "hardlink.json")
	if err := os.Link(path, alias); err == nil {
		if err := protectOutput(alias, path); err == nil {
			t.Fatal("hardlink overwrite accepted")
		}
	}
}

func TestDuplicateJSONKeysRejected(t *testing.T) {
	for _, data := range []string{`{"project":{"title":"a","title":"b"}}`, `{"shots":[{"id":"a","id":"b"}]}`} {
		if err := rejectDuplicateKeys([]byte(data)); err == nil {
			t.Fatal("duplicate accepted")
		}
	}
	if err := rejectDuplicateKeys([]byte(`{"a":{"x":1},"b":{"x":2}}`)); err != nil {
		t.Fatal(err)
	}
}

func TestCaptionBlockGrammar(t *testing.T) {
	valid := "\ufeffWEBVTT\n\nNOTE producer note\n00:00.000 --> 00:01.000\nThis is not a cue\n\ncaption-a\n00:01.000 --> 00:02.000 align:start\nHello\n"
	cues, err := parseCaptions(valid, ".vtt")
	if err != nil || len(cues) != 1 || cues[0].Start != 1 {
		t.Fatalf("VTT notes not handled: %+v %v", cues, err)
	}
	for _, data := range []string{"no header\n\n00:00.000 --> 00:01.000\ntext\n", "WEBVTT\n\n00:00,000 --> 00:01,000\ntext\n", "WEBVTT\n\ngarbage\n", "WEBVTT\n\n00:00.000 --> 00:01.000\ntext\n00:02.000 --> 00:03.000\ntext\n"} {
		if _, err := parseCaptions(data, ".vtt"); err == nil {
			t.Fatalf("bad captions accepted: %q", data)
		}
	}
	if _, ok := parseRatio("0:9"); ok {
		t.Fatal("zero ratio accepted")
	}
}

func TestMediaVideoDurationAndPixelFormatAreIndependent(t *testing.T) {
	p := validManifest()
	p.Delivery.PixelFormat = "yuv420p"
	m := MediaSummary{Width: 1920, Height: 1080, FPS: 30, DurationSeconds: 5, VideoDurationSeconds: 1, Frames: 30, PixelFormat: "yuv444p"}
	var report Report
	compareMedia(&report, p, m)
	if !hasCheck(report, "media.spec", "fail") {
		t.Fatal("short video masked by container duration")
	}
}

func TestIntegrationShaderMP4AndFailureGates(t *testing.T) {
	runtimeProject := os.Getenv("PV_INTEGRATION_RUNTIME")
	if runtimeProject == "" {
		runtimeProject = os.Getenv("PV_INTEGRATION_PROJECT")
	}
	if runtimeProject == "" {
		t.Skip("set PV_INTEGRATION_RUNTIME to a prepared renderer; fixtures clean themselves")
	}
	runtimeProject, err := filepath.Abs(runtimeProject)
	if err != nil {
		t.Fatal(err)
	}
	templates, transitions, err := readCatalog()
	if err != nil {
		t.Fatal(err)
	}
	fixture := templates["character-reveal-midnight"]
	fixture.ID = "integration-fixture"
	fixture.Scenes = append([]SceneData(nil), fixture.Scenes[:1]...)
	fixture.Scenes[0].Duration = 1
	fixture.Scenes[0].TransitionDuration = 0
	root := filepath.Join(t.TempDir(), "isolated-pv")
	if err := initializeDataProject(root, fixture, transitions, fixture.ProjectTitle); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	runCLI := func(args []string) int {
		if len(args) > 0 && args[0] == "render" {
			args = append(args, "--renderer-project", runtimeProject)
		}
		return realMain(args, &stdout, &stderr)
	}
	tools, err := resolveMediaTools("ffprobe", "ffmpeg", true, false, "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, "pv-project.json")
	originalManifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	defer os.WriteFile(manifestPath, originalManifest, 0644)
	var manifest ProjectFile
	if err := json.Unmarshal(originalManifest, &manifest); err != nil {
		t.Fatal(err)
	}
	tone := filepath.Join(root, "test-tone.wav")
	duration := strconv.FormatFloat(manifest.Project.DurationSeconds, 'f', 3, 64)
	if out, err := exec.Command(tools.FFmpeg, "-hide_banner", "-v", "error", "-y", "-f", "lavfi", "-i", "sine=frequency=440:duration="+duration, tone).CombinedOutput(); err != nil {
		t.Fatalf("generate audio fixture: %v %s", err, out)
	}
	defer os.Remove(tone)
	manifest.Assets = append(manifest.Assets, Asset{ID: "tone", Path: "test-tone.wav", Role: "music", Source: "Synthetic sine wave used only as an automated audio fixture", RightsStatus: "cleared", Status: "draft"})
	updated, _ := json.Marshal(manifest)
	if err := os.WriteFile(manifestPath, updated, 0644); err != nil {
		t.Fatal(err)
	}
	suffix := "fixture"
	out := "deliverables/test-" + suffix + ".mp4"
	raster := os.Getenv("PV_INTEGRATION_RASTER")
	if raster == "" {
		raster = "gpu"
	}
	args := []string{"render", "--project", root, "--raster", raster, "--out", out, "--report", "qa/test-" + suffix + ".json", "--auto-install=false", "--audio", "test-tone.wav"}
	if code := runCLI(args); code != 0 {
		t.Fatalf("real shader render: %d %s", code, stderr.String())
	}
	reportData, err := os.ReadFile(filepath.Join(root, "qa", "test-"+suffix+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var report BuildReport
	if err := json.Unmarshal(reportData, &report); err != nil {
		t.Fatal(err)
	}
	if !report.Passed || len(report.Steps) < 5 || report.Validation == nil || !hasCheck(*report.Validation, "media.decode", "pass") || !report.VerificationFullTimeline || report.VerificationSamples != int(math.Round(manifest.Project.DurationSeconds*manifest.Project.FPS)) || report.CaptureBridge != captureBridgeID {
		t.Fatalf("incomplete pipeline evidence: %+v", report)
	}
	media := report.Validation.Media
	if media == nil || media.Frames != int64(math.Round(manifest.Project.DurationSeconds*manifest.Project.FPS)) || media.PixelFormat != "yuv420p" || media.AudioCodec != "aac" {
		t.Fatalf("unexpected actual MP4: %+v", media)
	}
	stderr.Reset()
	if code := runCLI([]string{"check", "--manifest", filepath.Join(root, "pv-project.json"), "--media", filepath.Join(root, out), "--release", "--auto-install=false"}); code != 1 {
		t.Fatal("draft unexpectedly released")
	}
	// Verify a shader compiler error cannot yield a successful MP4.
	shaderName := "catalog.wgsl"
	for _, asset := range manifest.Assets {
		if asset.ID == "shader" {
			shaderName = asset.Path
		}
	}
	shaderPath := filepath.Join(root, shaderName)
	original, err := os.ReadFile(shaderPath)
	if err != nil {
		t.Fatal(err)
	}
	defer os.WriteFile(shaderPath, original, 0644)
	_ = os.WriteFile(shaderPath, []byte("THIS IS NOT WGSL"), 0644)
	stderr.Reset()
	badOut := "deliverables/invalid-" + suffix + ".mp4"
	if code := runCLI([]string{"render", "--project", root, "--out", badOut, "--report", "qa/invalid-" + suffix + ".json", "--auto-install=false"}); code == 0 {
		t.Fatal("broken shader succeeded")
	}
	if _, err := os.Stat(filepath.Join(root, badOut)); !os.IsNotExist(err) {
		t.Fatal("broken shader produced output")
	}
	if err := os.WriteFile(shaderPath, original, 0644); err != nil {
		t.Fatal(err)
	}
	htmlPath := filepath.Join(root, "pv.html")
	html, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatal(err)
	}
	defer os.WriteFile(htmlPath, html, 0644)
	changed := strings.Replace(string(html), "Shot.define(", "let pvCounter=0;\nShot.define(", 1)
	marker := "player.render(t);"
	if !strings.Contains(changed, marker) {
		marker = "shader.render(t);"
	}
	if !strings.Contains(changed, marker) {
		t.Fatal("nondeterminism fixture failed to modify source")
	}
	changed = strings.Replace(changed, marker, marker+" document.getElementById('sky').style.opacity=(++pvCounter%2)?'0.15':'1';", 1)
	_ = os.WriteFile(htmlPath, []byte(changed), 0644)
	stderr.Reset()
	badOut = "deliverables/nondeterministic-" + suffix + ".mp4"
	badReport := "qa/nondeterministic-" + suffix + ".json"
	if code := runCLI([]string{"render", "--project", root, "--out", badOut, "--report", badReport, "--raster", raster, "--auto-install=false"}); code == 0 {
		t.Fatal("stateful animation succeeded")
	}
	data, _ := os.ReadFile(filepath.Join(root, badReport))
	if !bytes.Contains(data, []byte("nondeterministic_output")) {
		t.Fatalf("did not exercise determinism gate: %s %s", data, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, badOut)); !os.IsNotExist(err) {
		t.Fatal("nondeterministic shader produced output")
	}
	for name, injection := range map[string]string{
		"webgpu-unavailable": "Object.defineProperty(navigator,'gpu',{value:undefined});",
		"webgpu-device-lost": "const originalRequestDevice=GPUAdapter.prototype.requestDevice;GPUAdapter.prototype.requestDevice=async function(...args){const device=await originalRequestDevice.apply(this,args);device.destroy();return device;};",
	} {
		t.Run(name, func(t *testing.T) {
			changed := strings.Replace(string(html), "Shot.define(", injection+"\nShot.define(", 1)
			if changed == string(html) {
				t.Fatal("fixture marker missing")
			}
			if err := os.WriteFile(htmlPath, []byte(changed), 0644); err != nil {
				t.Fatal(err)
			}
			defer os.WriteFile(htmlPath, html, 0644)
			badOut := "deliverables/" + name + "-" + suffix + ".mp4"
			stderr.Reset()
			if code := runCLI([]string{"render", "--project", root, "--out", badOut, "--report", "qa/" + name + "-" + suffix + ".json", "--auto-install=false"}); code == 0 {
				t.Fatal("unusable WebGPU device accepted")
			}
			if _, err := os.Stat(filepath.Join(root, badOut)); !os.IsNotExist(err) {
				t.Fatal("unusable WebGPU produced output")
			}
		})
	}
}
