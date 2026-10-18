package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func snapshotFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		hash, err := fileSHA256(path)
		files[filepath.ToSlash(rel)] = hash
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return files
}

func TestEvidenceOwnershipIsStableAndPreservesUnknownFiles(t *testing.T) {
	root := t.TempDir()
	if err := prepareOwnedEvidence(root, "test/1"); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(root, "user-note"), []byte("keep"), 0600)
	before := snapshotFiles(t, root)
	if err := prepareOwnedEvidence(root, "test/1"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, snapshotFiles(t, root)) {
		t.Fatal("repeat preparation changed files")
	}
	if err := prepareOwnedEvidence(root, "other/1"); err == nil {
		t.Fatal("wrong owner accepted")
	}
	unowned := t.TempDir()
	_ = os.WriteFile(filepath.Join(unowned, "user-data"), []byte("keep"), 0600)
	if err := prepareOwnedEvidence(unowned, "test/1"); err == nil {
		t.Fatal("unowned directory accepted")
	}
}

func TestBrowserAtlasCoversAllPresetsAndRepeats(t *testing.T) {
	templates, transitions, err := readCatalog()
	if err != nil {
		t.Fatal(err)
	}
	atlas, rows := browserAtlas(templates, transitions)
	if len(rows) != 968 || len(atlas.Cases) != len(rows) {
		t.Fatal("incomplete browser atlas")
	}
	counts, repeats := map[string]int{}, map[string]int{}
	for i, row := range rows {
		if row.Frame != i {
			t.Fatal("case/frame mismatch")
		}
		key := row.Kind + "/" + row.ID
		counts[key]++
		if row.Repeat {
			repeats[key]++
		}
		if row.AllowUniform && !templateUniformTransition(templates[row.ID], row.Time, transitions) {
			t.Fatal("blank exemption outside an intentional exposure transition")
		}
	}
	for id := range templates {
		if counts["template/"+id] != 4 || repeats["template/"+id] != 1 {
			t.Fatal(id)
		}
	}
	for id := range transitions {
		if counts["transition/"+id] != 6 || repeats["transition/"+id] != 1 {
			t.Fatal(id)
		}
	}
}

func TestPreviewRestrictsOriginsMethodsAndPaths(t *testing.T) {
	templates, transitions, _ := readCatalog()
	root := t.TempDir()
	const origin = "http://127.0.0.1:7335"
	handler := newPreviewHandler(root, root, origin, templates, transitions)
	for _, tc := range []struct {
		method, path, host, requestOrigin string
		code                              int
	}{
		{"GET", "/", "127.0.0.1:7335", "", 200},
		{"GET", "/api/browser-cases", "127.0.0.1:7335", "", 200},
		{"GET", "/atlas/all/catalog-runtime.js", "127.0.0.1:7335", "", 200},
		{"GET", "/multiplane/runtime/../../package.json", "127.0.0.1:7335", "", 404},
		{"GET", "/", "untrusted.test", "", 403},
		{"POST", "/api/report", "127.0.0.1:7335", "https://untrusted.test", 403},
		{"POST", "/api/report", "127.0.0.1:7335", origin, 200},
		{"DELETE", "/", "127.0.0.1:7335", "", 405},
	} {
		r := httptest.NewRequest(tc.method, origin+tc.path, strings.NewReader(`{"passed":true}`))
		r.Host = tc.host
		r.Header.Set("Origin", tc.requestOrigin)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Fatalf("%s %s: %d, want %d", tc.method, tc.path, w.Code, tc.code)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "edge-report.json")); err != nil {
		t.Fatal(err)
	}
}

func TestWebsiteAndUnityDeliveryContracts(t *testing.T) {
	data, _ := bundled.ReadFile("assets/delivery/background.html")
	for _, required := range []string{"autoplay muted loop playsinline", `tabindex="-1"`, `aria-hidden="true" inert`, "pointer-events:none", "prefers-reduced-motion", "visibilitychange", "poster.png", "video.controls=false"} {
		if !bytes.Contains(data, []byte(required)) {
			t.Fatal("background missing", required)
		}
	}
	data, _ = bundled.ReadFile("assets/delivery/PVVideoSurface.cs")
	for _, required := range []string{"raycastTarget = false", "player.Prepare()", "prepareCompleted -= OnPrepared", "texture.Release()", "Application.streamingAssetsPath", "player.isLooping = loop"} {
		if !bytes.Contains(data, []byte(required)) {
			t.Fatal("Unity helper missing", required)
		}
	}
}

func TestIntegrationBundleIsIdempotentAndCleansStaging(t *testing.T) {
	if os.Getenv("PV_MEDIA_INTEGRATION") != "1" {
		t.Skip("set PV_MEDIA_INTEGRATION=1 for real MP4 remux/cache tests")
	}
	tools, err := resolveMediaTools("ffprobe", "ffmpeg", true, false, "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	templates, transitions, _ := readCatalog()
	fixture := templates["website-atmosphere-midnight"]
	fixture.Width, fixture.Height, fixture.FPS = 640, 360, 10
	fixture.Scenes = append([]SceneData(nil), fixture.Scenes...)
	fixture.Scenes[0].Duration = 1
	root := filepath.Join(t.TempDir(), "project")
	if err := initializeDataProject(root, fixture, transitions, `A < B & "Title"`); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "deliverables"), 0755); err != nil {
		t.Fatal(err)
	}
	video := filepath.Join(root, "deliverables", "master.mp4")
	if output, err := exec.Command(tools.FFmpeg, "-nostdin", "-v", "error", "-n", "-f", "lavfi", "-i", "color=c=navy:size=640x360:rate=10:duration=1", "-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", "-shortest", video).CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v %s", err, output)
	}
	var out, stderr bytes.Buffer
	for _, target := range []string{"website", "unity"} {
		args := []string{"bundle", "--project", root, "--target", target}
		if code := realMain(args, &out, &stderr); code != 0 {
			t.Fatalf("bundle: %d %s", code, stderr.String())
		}
		destination := filepath.Join(root, "deliverables", target)
		_ = os.WriteFile(filepath.Join(destination, "user-note"), []byte("keep"), 0600)
		before := snapshotFiles(t, destination)
		out.Reset()
		if code := realMain(args, &out, &stderr); code != 0 || !strings.Contains(out.String(), "Bundle is current") {
			t.Fatalf("repeat: %d %s %s", code, out.String(), stderr.String())
		}
		if !reflect.DeepEqual(before, snapshotFiles(t, destination)) {
			t.Fatal("repeat bundle changed file set/content")
		}
		if target == "website" {
			data, _ := os.ReadFile(filepath.Join(destination, "background.html"))
			if !bytes.Contains(data, []byte("A &lt; B &amp; &#34;Title&#34;")) {
				t.Fatal("unescaped website title")
			}
			var report DeliveryBundle
			raw, _ := os.ReadFile(filepath.Join(destination, "bundle-report.json"))
			_ = json.Unmarshal(raw, &report)
			if report.Media.AudioCodec != "" {
				t.Fatal("website retained audio")
			}
			handler, err := websiteDeliveryHandler(destination, "http://127.0.0.1:7335", http.NotFoundHandler())
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest("GET", "http://127.0.0.1:7335/delivery/background.mp4", nil)
			r.Header.Set("Range", "bytes=0-31")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != 206 || w.Body.Len() != 32 {
				t.Fatal("MP4 range seeking broken", w.Code)
			}
			_ = os.WriteFile(filepath.Join(destination, "poster.png"), []byte("tampered"), 0600)
			if _, err := websiteDeliveryHandler(destination, "http://127.0.0.1:7335", http.NotFoundHandler()); err == nil {
				t.Fatal("modified delivery accepted")
			}
			if code := realMain(args, &out, &stderr); code != 0 {
				t.Fatal("could not repair owned bundle", stderr.String())
			}
		}
	}
	staging, _ := filepath.Glob(filepath.Join(root, "deliverables", ".pv-bundle-*"))
	if len(staging) != 0 {
		t.Fatal("bundle staging leaked", staging)
	}
}
