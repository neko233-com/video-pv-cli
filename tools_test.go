package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSHA256AndReleaseVersion(t *testing.T) {
	want := strings.Repeat("a", sha256.Size*2)
	got, err := parseSHA256(want + "  ffmpeg.zip")
	if err != nil || got != want {
		t.Fatalf("parse SHA-256: got %q, err=%v", got, err)
	}
	if _, err := parseSHA256("not-a-checksum"); err == nil {
		t.Fatal("invalid checksum was accepted")
	}
	match := releaseArchivePattern.FindStringSubmatch("ffmpeg-9.0.2-essentials_build.zip.sha256")
	if len(match) < 2 || match[1] != "9.0.2" || !validReleaseVersion(match[1]) {
		t.Fatalf("release archive version was not parsed: %v", match)
	}
	if validReleaseVersion("../9.0.2") || validReleaseVersion("9.0.2-preview") {
		t.Fatal("unsafe release version was accepted")
	}
}

func TestParseContentRange(t *testing.T) {
	start, end, total, err := parseContentRange("bytes 8388608-16777215/114768076")
	if err != nil || start != 8388608 || end != 16777215 || total != 114768076 {
		t.Fatalf("parse Content-Range: %d-%d/%d, err=%v", start, end, total, err)
	}
	if _, _, _, err := parseContentRange("bytes 0-0/*"); err == nil {
		t.Fatal("unknown content length was accepted")
	}
}

func TestDownloadArchiveUsesRangesAndVerifiesChecksum(t *testing.T) {
	payload := bytes.Repeat([]byte("web2video-range-download\n"), 22000)
	digest := sha256.Sum256(payload)
	wantSHA := hex.EncodeToString(digest[:])
	var ranges []string
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Scheme != "https" || request.URL.Host != "www.gyan.dev" {
			return nil, fmt.Errorf("unexpected test request URL: %s", request.URL)
		}
		rangeValue := request.Header.Get("Range")
		var start, end int
		if _, err := fmt.Sscanf(rangeValue, "bytes=%d-%d", &start, &end); err != nil || start < 0 || end < start || end >= len(payload) {
			return nil, fmt.Errorf("invalid test range: %q", rangeValue)
		}
		ranges = append(ranges, rangeValue)
		body := io.NopCloser(bytes.NewReader(payload[start : end+1]))
		header := make(http.Header)
		header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(payload)))
		return &http.Response{Status: "206 Partial Content", StatusCode: http.StatusPartialContent, Header: header, Body: body, Request: request, ContentLength: int64(end - start + 1)}, nil
	})
	client := &http.Client{Transport: transport}
	cache := t.TempDir()
	archivePath, err := downloadArchive(context.Background(), client, "https://www.gyan.dev/ffmpeg/test.zip", cache, wantSHA, nil)
	if err != nil {
		t.Fatalf("download ranged archive: %v", err)
	}
	defer os.Remove(archivePath)
	got, err := os.ReadFile(archivePath)
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("downloaded archive mismatch: bytes=%d, err=%v", len(got), err)
	}
	if len(ranges) != 2 || ranges[0] != "bytes=0-0" || ranges[1] != fmt.Sprintf("bytes=1-%d", len(payload)-1) {
		t.Fatalf("unexpected download ranges: %v", ranges)
	}
	if _, err := downloadArchive(context.Background(), client, "https://www.gyan.dev/ffmpeg/test.zip", cache, strings.Repeat("0", 64), nil); err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Fatalf("bad checksum should reject the archive, got %v", err)
	}
	entries, err := os.ReadDir(cache)
	if err != nil || len(entries) != 1 || entries[0].Name() != filepath.Base(archivePath) {
		t.Fatalf("failed checksum created an unexpected cache file: entries=%v, err=%v", entries, err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func TestExtractFFmpegArchiveKeepsRuntimeDependencies(t *testing.T) {
	archivePath := makeTestFFmpegArchive(t, map[string][]byte{
		"ffmpeg-9.0.2-essentials_build/bin/ffmpeg.exe":  []byte("ffmpeg"),
		"ffmpeg-9.0.2-essentials_build/bin/ffprobe.exe": []byte("ffprobe"),
		"ffmpeg-9.0.2-essentials_build/bin/avcodec.dll": []byte("runtime"),
		"ffmpeg-9.0.2-essentials_build/LICENSE.txt":     []byte("license"),
	})
	destination := t.TempDir()
	if err := extractFFmpegArchive(archivePath, destination); err != nil {
		t.Fatalf("extract safe package: %v", err)
	}
	for name, want := range map[string]string{
		"bin/ffmpeg.exe":  "ffmpeg",
		"bin/ffprobe.exe": "ffprobe",
		"bin/avcodec.dll": "runtime",
		"LICENSE.txt":     "license",
	} {
		data, err := os.ReadFile(filepath.Join(destination, filepath.FromSlash(name)))
		if err != nil || string(data) != want {
			t.Errorf("extracted %s: got %q, err=%v", name, data, err)
		}
	}
}

func TestExtractFFmpegArchiveRejectsPathTraversal(t *testing.T) {
	archivePath := makeTestFFmpegArchive(t, map[string][]byte{
		"ffmpeg-9.0.2-essentials_build/bin/ffmpeg.exe":   []byte("ffmpeg"),
		"ffmpeg-9.0.2-essentials_build/bin/ffprobe.exe":  []byte("ffprobe"),
		"ffmpeg-9.0.2-essentials_build/../../escape.txt": []byte("escape"),
	})
	destination := t.TempDir()
	if err := extractFFmpegArchive(archivePath, destination); err == nil {
		t.Fatal("archive path traversal was accepted")
	}
}

func TestDefaultFFmpegCacheUsesUserCacheAndOverride(t *testing.T) {
	defaultDir, err := defaultFFmpegCacheDir()
	if err != nil || !strings.Contains(filepath.ToSlash(defaultDir), "web2video/ffmpeg") {
		t.Fatalf("unexpected default user cache: %q, %v", defaultDir, err)
	}
	want, err := filepath.Abs(filepath.Join(t.TempDir(), "tools"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := resolveFFmpegCacheDir(filepath.Dir(want))
	if err != nil || got != filepath.Dir(want) {
		t.Fatalf("cache override mismatch: got %q, want %q, err=%v", got, filepath.Dir(want), err)
	}
}

func TestMissingFFmpegCanBeRejectedWithoutNetworkWhenAutoInstallIsDisabled(t *testing.T) {
	missingProbe := filepath.Join(t.TempDir(), "missing-ffprobe")
	missingFFmpeg := filepath.Join(t.TempDir(), "missing-ffmpeg")
	_, err := resolveMediaTools(missingProbe, missingFFmpeg, true, false, t.TempDir(), true, true)
	if err == nil || !strings.Contains(err.Error(), "required media tools are unavailable") {
		t.Fatalf("expected disabled auto-install guidance, got %v", err)
	}
}

func makeTestFFmpegArchive(t *testing.T, entries map[string][]byte) string {
	t.Helper()
	archivePath := filepath.Join(t.TempDir(), "ffmpeg.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for name, contents := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			file.Close()
			t.Fatal(err)
		}
		if _, err := entry.Write(contents); err != nil {
			file.Close()
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return archivePath
}
