package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	files, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		raw, err := os.ReadFile(filepath.Join(root, f.Name()))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		out[f.Name()] = hex.EncodeToString(sum[:])
	}
	return out
}
func TestGalleryUsesActualPixelsAndIsIdempotent(t *testing.T) {
	root := t.TempDir()
	evidence := filepath.Join(root, "evidence")
	batch := filepath.Join(evidence, "batch-001")
	_ = os.MkdirAll(filepath.Join(batch, "qa"), 0755)
	img := image.NewNRGBA(image.Rect(0, 0, 16, 9))
	img.Set(0, 0, color.NRGBA{R: 200, A: 255})
	var pngData bytes.Buffer
	_ = png.Encode(&pngData, img)
	_ = os.WriteFile(filepath.Join(batch, "frame.png"), pngData.Bytes(), 0600)
	sum := sha256.Sum256(pngData.Bytes())
	hash := hex.EncodeToString(sum[:])
	cases := []map[string]any{}
	pixels := []map[string]any{}
	for i := 0; i < 100; i++ {
		id := fmt.Sprintf("style-%03d", i)
		caseID := "style/" + id + "/0"
		cases = append(cases, map[string]any{"id": caseID, "template": map[string]any{"style": preset{ID: id, Name: "<safe>", Family: "keynote", Keywords: []string{"rim light"}}}})
		pixels = append(pixels, map[string]any{"case": caseID, "png": "frame.png", "sha256": hash})
	}
	write := func(path string, v any) {
		raw, _ := json.Marshal(v)
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(batch, "template.json"), map[string]any{"cases": cases})
	write(filepath.Join(batch, "qa/pixel-report.json"), pixels)
	write(filepath.Join(evidence, "catalog-report.json"), map[string]any{"passed": true, "styles": 100, "batches": []string{"batch-001"}})
	out := filepath.Join(root, "gallery")
	unowned := filepath.Join(root, "unowned")
	_ = os.Mkdir(unowned, 0755)
	_ = os.WriteFile(filepath.Join(unowned, "user-data"), []byte("keep"), 0600)
	if build(evidence, unowned) == nil {
		t.Fatal("unowned output accepted")
	}
	if err := build(evidence, out); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(out, "user-note"), []byte("keep"), 0600)
	before := snapshot(t, out)
	if err := build(evidence, out); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, snapshot(t, out)) {
		t.Fatal("repeat gallery changed files")
	}
	html, _ := os.ReadFile(filepath.Join(out, "index.html"))
	if bytes.Contains(html, []byte("<safe>")) || !bytes.Contains(html, []byte("&lt;safe&gt;")) {
		t.Fatal("unescaped label")
	}
	_ = os.WriteFile(filepath.Join(batch, "frame.png"), []byte("tampered"), 0600)
	if build(evidence, out) == nil {
		t.Fatal("tampered evidence accepted")
	}
	if !reflect.DeepEqual(before, snapshot(t, out)) {
		t.Fatal("failure changed gallery")
	}
}
func TestGalleryRejectsIncompleteEvidenceAndTraversal(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "catalog-report.json"), []byte(`{"passed":false,"styles":100}`), 0600)
	if build(root, filepath.Join(root, "out")) == nil {
		t.Fatal("failed evidence accepted")
	}
	if _, err := confined(root, "../outside"); err == nil {
		t.Fatal("traversal accepted")
	}
}
