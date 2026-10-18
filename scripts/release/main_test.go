package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestReleaseArchiveAllowlist(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	os.WriteFile(source, []byte("binary"), 0600)
	for _, bad := range []bool{false, true} {
		path := filepath.Join(root, "bundle.zip")
		f, _ := os.Create(path)
		zw := zip.NewWriter(f)
		name := "web2video.exe"
		if bad {
			name = "large.mp4"
		}
		w, _ := zw.Create(name)
		w.Write([]byte("binary"))
		zw.Close()
		f.Close()
		err := verifyArchive(path, map[string]string{"web2video.exe": source})
		if (err != nil) != bad {
			t.Fatal(bad, err)
		}
	}
}
