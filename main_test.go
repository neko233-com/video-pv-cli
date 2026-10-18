package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateDraftManifestAllowsPlannedAssetWithoutFile(t *testing.T) {
	root := t.TempDir()
	manifest := ProjectFile{
		SchemaVersion: "1.0",
		Project:       ProjectInfo{Title: "Draft", Type: "character", DurationSeconds: 5, Width: 1920, Height: 1080, FPS: 30},
		Assets:        []Asset{{ID: "future-frame", Role: "keyframe", Source: "Planned image generation", RightsStatus: "unknown", Status: "planned"}},
		Shots:         []Shot{{ID: "s1", StartSeconds: 0, EndSeconds: 5, Beat: "Reveal", Role: "hook", VisualDescription: "Silhouette", AssetIDs: []string{"future-frame"}, Status: "planned"}},
	}
	path := writeTestManifest(t, root, manifest)
	_, report := validateManifest(path, false)
	if reportHasFailure(report) {
		t.Fatalf("draft plan with a planned asset should remain structurally valid; got checks: %+v", report.Checks)
	}
}

func TestValidateClaimRequiresMatchingEvidenceInShot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "proof.txt"), []byte("proof"), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := validManifest()
	manifest.Assets = append(manifest.Assets, Asset{ID: "other", Path: "proof.txt", Role: "reference", Source: "approved source", RightsStatus: "cleared", Status: "approved"})
	manifest.Claims = []Claim{{ID: "claim-1", Text: "A supported claim", EvidenceAssetIDs: []string{"source"}}}
	manifest.Shots[0].AssetIDs = []string{"other"}
	manifest.Shots[0].ClaimIDs = []string{"claim-1"}
	path := writeTestManifest(t, root, manifest)
	_, report := validateManifest(path, false)
	if !hasCheck(report, "claim.shot_evidence_mismatch", "fail") {
		t.Fatalf("expected claim/evidence mismatch failure, checks: %+v", report.Checks)
	}
}

func TestRejectsAssetPathTraversal(t *testing.T) {
	root := t.TempDir()
	manifest := validManifest()
	manifest.Assets[0].Path = "../outside.txt"
	path := writeTestManifest(t, root, manifest)
	_, report := validateManifest(path, false)
	if !hasCheck(report, "asset.path", "fail") {
		t.Fatalf("expected path traversal failure, checks: %+v", report.Checks)
	}
}

func TestReleaseRequiresClearedApprovedAssetsAndHumanAttestation(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "source.txt"), []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := validManifest()
	path := writeTestManifest(t, root, manifest)
	_, report := validateManifest(path, true)
	if !hasCheck(report, "asset.not_approved", "fail") || !hasCheck(report, "asset.rights_uncleared", "fail") || !hasCheck(report, "review.attestation", "fail") {
		t.Fatalf("release should reject unapproved, uncleared, unreviewed plan; checks: %+v", report.Checks)
	}
}

func TestParseCaptionsValidatesSRTAndVTT(t *testing.T) {
	validSRT := "1\n00:00:00,000 --> 00:00:01,200\nHello\n"
	cues, err := parseCaptions(validSRT, ".srt")
	if err != nil || len(cues) != 1 || cues[0].End != 1.2 {
		t.Fatalf("unexpected SRT parse: %+v, %v", cues, err)
	}
	invalid := "1\n00:00:01,000 --> 00:00:01,000\nHello\n"
	if _, err := parseCaptions(invalid, ".srt"); err == nil {
		t.Fatal("expected invalid cue duration error")
	}
	validVTT := "WEBVTT\n\n00:00:00.000 --> 00:00:01.000\nHello\n"
	if _, err := parseCaptions(validVTT, ".vtt"); err != nil {
		t.Fatalf("valid VTT rejected: %v", err)
	}
	shortVTT := "WEBVTT\n\n00:00.000 --> 00:01.000\nHello\n"
	if _, err := parseCaptions(shortVTT, ".vtt"); err != nil {
		t.Fatalf("valid short VTT timestamp rejected: %v", err)
	}
}

func TestParseHelpersAndFrameRates(t *testing.T) {
	if ratio, ok := parseRatio("16:9"); !ok || ratio < 1.77 || ratio > 1.78 {
		t.Fatalf("bad ratio: %f, %t", ratio, ok)
	}
	if _, ok := parseRatio("wide"); ok {
		t.Fatal("invalid ratio accepted")
	}
	if got := parseFrameRate("30000/1001"); got < 29.96 || got > 29.98 {
		t.Fatalf("bad rational frame rate: %f", got)
	}
	if got := parseFrameRate("30"); got != 30 {
		t.Fatalf("bad decimal frame rate: %f", got)
	}
}

func TestCompareMediaAcceptsMP4FamilyAndFindsSpecMismatch(t *testing.T) {
	project := validManifest()
	project.Delivery.Container = "mp4"
	project.Delivery.VideoCodec = "h264"
	project.Delivery.AudioCodec = "aac"
	project.Project.AudioRequired = true
	media := MediaSummary{Container: "mov,mp4,m4a,3gp,3g2,mj2", VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080, FPS: 29.97, DurationSeconds: 5}
	var report Report
	compareMedia(&report, project, media)
	if !hasCheck(report, "media.spec", "pass") {
		t.Fatalf("valid MP4 export rejected: %+v", report.Checks)
	}
	media.Width = 1280
	media.AudioCodec = ""
	compareMedia(&report, project, media)
	if !hasCheck(report, "media.spec", "fail") {
		t.Fatalf("invalid export not detected: %+v", report.Checks)
	}
}

func TestCLIWritesJSONReportAndAcceptsDraftExample(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "source.txt"), []byte("brief"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := writeTestManifest(t, root, validManifest())
	var stdout, stderr bytes.Buffer
	code := realMain([]string{"--manifest", path, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected successful draft check, exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	var report Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON report: %v", err)
	}
	if !report.Passed || report.ReleaseReady {
		t.Fatalf("unexpected report state: %+v", report)
	}
}

func TestCLIReleaseGateReturnsFailureForDraftProject(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "source.txt"), []byte("brief"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := writeTestManifest(t, root, validManifest())
	var stdout, stderr bytes.Buffer
	code := realMain([]string{"--manifest", path, "--release", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected release gate to reject a draft manifest, exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	var report Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("invalid release report: %v", err)
	}
	if report.Passed || report.ReleaseReady || !report.ReleaseMode {
		t.Fatalf("draft project incorrectly passed release gate: %+v", report)
	}
}

func TestManifestRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pv-project.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":"1.0","unexpected":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, report := validateManifest(path, false)
	if !hasCheck(report, "manifest.json", "fail") {
		t.Fatalf("unknown manifest field should fail strict decoding: %+v", report.Checks)
	}
}

func TestMissingFFprobeReturnsUsefulError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fake.mp4")
	if err := os.WriteFile(path, []byte("not a video"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := inspectMedia(path, filepath.Join(t.TempDir(), "missing-ffprobe"))
	if err == nil || !strings.Contains(err.Error(), "ffprobe executable") {
		t.Fatalf("expected missing ffprobe guidance, got %v", err)
	}
}

func TestMissingFFmpegReturnsUsefulError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fake.mp4")
	if err := os.WriteFile(path, []byte("not a video"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := decodeMedia(path, filepath.Join(t.TempDir(), "missing-ffmpeg"))
	if err == nil || !strings.Contains(err.Error(), "ffmpeg executable") {
		t.Fatalf("expected missing ffmpeg guidance, got %v", err)
	}
}

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
	payload := bytes.Repeat([]byte("video-pv-cli-range-download\n"), 22000)
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
	if err != nil || !strings.Contains(filepath.ToSlash(defaultDir), "pv-production/ffmpeg") {
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

func TestSkillEntrypointReferencesAndSampleManifestArePresent(t *testing.T) {
	root := "."
	skillPath := filepath.Join(root, "SKILL.md")
	skillBytes, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read skill entrypoint: %v", err)
	}
	skill := string(skillBytes)
	if !strings.HasPrefix(skill, "---\nname: pv-production\n") {
		t.Fatal("skill entrypoint must declare the pv-production name in YAML frontmatter")
	}
	for _, required := range []string{"references/production-playbook.md", "references/project-schema.md", "references/validation.md"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(required))); err != nil {
			t.Errorf("required skill reference %s is missing: %v", required, err)
		}
		if !strings.Contains(skill, required) {
			t.Errorf("SKILL.md does not route to %s", required)
		}
	}
	manifestPath := filepath.Join(root, "examples", "pv-project.json")
	_, report := validateManifest(manifestPath, false)
	if reportHasFailure(report) {
		t.Fatalf("checked-in sample manifest is invalid: %+v", report.Checks)
	}
}

func TestWriteReportOverwritesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONReport(path, Report{ProjectTitle: "updated", Passed: true}); err != nil {
		t.Fatalf("rewrite report: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"project_title": "updated"`) {
		t.Fatalf("report was not replaced: %s", data)
	}
}

func validManifest() ProjectFile {
	return ProjectFile{
		SchemaVersion: "1.0",
		Project:       ProjectInfo{Title: "Test PV", Type: "product", DurationSeconds: 5, Width: 1920, Height: 1080, FPS: 30, Platforms: []string{"web"}, Language: "en"},
		Assets:        []Asset{{ID: "source", Path: "source.txt", Role: "product-proof", Source: "Approved capture", RightsStatus: "pending", Status: "draft"}},
		Shots:         []Shot{{ID: "s1", StartSeconds: 0, EndSeconds: 5, Beat: "Proof", Role: "proof", VisualDescription: "Real product screen", AssetIDs: []string{"source"}, Status: "draft"}},
		Delivery:      Delivery{VideoPath: "deliverables/master.mp4"},
	}
}

func writeTestManifest(t *testing.T, root string, manifest ProjectFile) string {
	t.Helper()
	path := filepath.Join(root, "pv-project.json")
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
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

func reportHasFailure(report Report) bool {
	for _, check := range report.Checks {
		if check.Status == "fail" {
			return true
		}
	}
	return false
}

func hasCheck(report Report, code, status string) bool {
	for _, check := range report.Checks {
		if check.Code == code && check.Status == status {
			return true
		}
	}
	return false
}
