package main

import (
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadOnlyPreviewHTTP(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "index.html"), []byte(`<html><head></head><body><div data-composition-id="main" data-width="320" data-height="180"></div></body></html>`), 0600)
	os.WriteFile(filepath.Join(root, ".secret"), []byte("secret"), 0600)
	os.WriteFile(filepath.Join(root, "shader.wgsl"), []byte("shader"), 0600)
	runtime := filepath.Join(root, "runtime.js")
	os.WriteFile(runtime, []byte("runtime"), 0600)
	handler, err := previewHandler(root, runtime)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path, method, contains string
		code                   int
	}{{"/", "GET", "web2video", 200}, {"/__web2video/composition", "GET", "/__web2video/runtime.js", 200}, {"/shader.wgsl", "GET", "shader", 200}, {"/.secret", "GET", "", 404}, {"/../../outside", "GET", "", 404}, {"/", "POST", "", 405}} {
		r := httptest.NewRequest(tc.method, tc.path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		response := w.Result()
		raw, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if response.StatusCode != tc.code || !strings.Contains(string(raw), tc.contains) {
			t.Fatal(tc, response.StatusCode, string(raw))
		}
	}
}
func TestBrowserAssetIntegrity(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "chrome-headless-shell.exe")
	os.WriteFile(path, make([]byte, 80), 0600)
	if validateBrowserAssets(path) == nil {
		t.Fatal("incomplete browser accepted")
	}
	for _, name := range []string{"icudtl.dat", "libEGL.dll", "libGLESv2.dll", "v8_context_snapshot.bin", "headless_lib_data.pak", "headless_lib_strings.pak", "headless_command_resources.pak"} {
		os.WriteFile(filepath.Join(root, name), make([]byte, 80), 0600)
	}
	if err := validateBrowserAssets(path); err != nil {
		t.Fatal(err)
	}
}
