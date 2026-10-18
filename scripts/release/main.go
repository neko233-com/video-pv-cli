// Build a Windows CLI release locally. No CI service or media exports involved.
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func main() {
	if err := release(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func release() error {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		return fmt.Errorf("this release target is Windows amd64; validate other targets before packaging")
	}
	if err := os.MkdirAll("dist", 0755); err != nil {
		return err
	}
	binary := filepath.Join("dist", "web2video-windows-amd64.exe")
	build := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w", "-o", binary, ".")
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		return err
	}
	versionBytes, err := exec.Command(binary, "version").Output()
	if err != nil {
		return err
	}
	version := strings.TrimSpace(strings.TrimPrefix(string(versionBytes), "web2video "))
	if version == "" || strings.ContainsAny(version, "/\\ \r\n") {
		return fmt.Errorf("invalid CLI version")
	}
	archivePath := filepath.Join("dist", "web2video-"+version+"-windows-amd64.zip")
	entries := map[string]string{"web2video.exe": binary, "LICENSE": "LICENSE", "NOTICE": "NOTICE", "README.md": "README.md"}
	file, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(file)
	for _, name := range []string{"web2video.exe", "LICENSE", "NOTICE", "README.md"} {
		raw, e := os.ReadFile(entries[name])
		if e != nil {
			zw.Close()
			file.Close()
			return e
		}
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetModTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
		header.SetMode(0644)
		writer, e := zw.CreateHeader(header)
		if e != nil {
			zw.Close()
			file.Close()
			return e
		}
		if _, e = writer.Write(raw); e != nil {
			zw.Close()
			file.Close()
			return e
		}
	}
	if err = zw.Close(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = verifyArchive(archivePath, entries); err != nil {
		return err
	}
	var sums strings.Builder
	for _, path := range []string{binary, archivePath} {
		raw, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		hash := sha256.Sum256(raw)
		fmt.Fprintf(&sums, "%s  %s\n", hex.EncodeToString(hash[:]), filepath.Base(path))
	}
	if err = os.WriteFile(filepath.Join("dist", "SHA256SUMS.txt"), []byte(sums.String()), 0644); err != nil {
		return err
	}
	fmt.Print(sums.String())
	return nil
}
func verifyArchive(path string, expected map[string]string) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer zr.Close()
	if len(zr.File) != len(expected) {
		return fmt.Errorf("unexpected archive entry count")
	}
	seen := map[string]bool{}
	for _, entry := range zr.File {
		source, ok := expected[entry.Name]
		if !ok || seen[entry.Name] || strings.EqualFold(filepath.Ext(entry.Name), ".mp4") {
			return fmt.Errorf("unexpected release file: %s", entry.Name)
		}
		seen[entry.Name] = true
		r, e := entry.Open()
		if e != nil {
			return e
		}
		raw, e := io.ReadAll(r)
		r.Close()
		if e != nil {
			return e
		}
		want, e := os.ReadFile(source)
		if e != nil || !bytes.Equal(raw, want) {
			return fmt.Errorf("release entry differs from source: %s", entry.Name)
		}
	}
	return nil
}
