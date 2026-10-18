package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	ffmpegReleaseChecksumURL = "https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip.sha256"
	maxFFmpegArchiveBytes    = 250 << 20
	maxFFmpegUnpackedBytes   = 1 << 30
	maxFFmpegArchiveEntries  = 10000
	ffmpegDownloadChunkBytes = 8 << 20
)

var releaseArchivePattern = regexp.MustCompile(`^ffmpeg-([0-9]+(\.[0-9]+){1,3})-essentials_build\.zip\.sha256$`)
var releaseVersionPattern = regexp.MustCompile(`^[0-9]+(\.[0-9]+){1,3}$`)

type ffmpegCacheMetadata struct {
	Version   string    `json:"version"`
	SHA256    string    `json:"sha256"`
	SourceURL string    `json:"source_url"`
	Installed time.Time `json:"installed_at"`
}

type ffmpegTools struct {
	FFmpeg  string
	FFprobe string
	Version string
}

func defaultFFmpegCacheDir() (string, error) {
	root, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("locate current-user cache: %w", err)
	}
	return filepath.Join(root, "pv-production", "ffmpeg"), nil
}

func resolveFFmpegCacheDir(override string) (string, error) {
	if strings.TrimSpace(override) != "" {
		return filepath.Abs(override)
	}
	return defaultFFmpegCacheDir()
}

func findCachedFFmpegTools(cacheDir string) (ffmpegTools, error) {
	data, err := os.ReadFile(filepath.Join(cacheDir, "current.json"))
	if err != nil {
		return ffmpegTools{}, err
	}
	var metadata ffmpegCacheMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return ffmpegTools{}, fmt.Errorf("read FFmpeg cache metadata: %w", err)
	}
	if !validReleaseVersion(metadata.Version) {
		return ffmpegTools{}, fmt.Errorf("FFmpeg cache metadata has an invalid version")
	}
	binDir := filepath.Join(cacheDir, "gyan-win64", metadata.Version, "bin")
	tools := ffmpegTools{
		FFmpeg:  filepath.Join(binDir, "ffmpeg.exe"),
		FFprobe: filepath.Join(binDir, "ffprobe.exe"),
		Version: metadata.Version,
	}
	for _, executable := range []string{tools.FFmpeg, tools.FFprobe} {
		info, err := os.Stat(executable)
		if err != nil || !info.Mode().IsRegular() {
			return ffmpegTools{}, fmt.Errorf("cached FFmpeg tool is missing: %s", executable)
		}
	}
	return tools, nil
}

func resolveMediaTools(ffprobeName, ffmpegName string, needDecode, autoInstall bool, toolsDir string, explicitFFprobe, explicitFFmpeg bool) (ffmpegTools, error) {
	var tools ffmpegTools
	var probeErr, decodeErr error
	tools.FFprobe, probeErr = resolveExecutable(ffprobeName)
	if needDecode {
		tools.FFmpeg, decodeErr = resolveExecutable(ffmpegName)
	}
	if probeErr == nil && (!needDecode || decodeErr == nil) {
		return tools, nil
	}

	cacheDir, cacheErr := resolveFFmpegCacheDir(toolsDir)
	if cacheErr == nil {
		cached, err := findCachedFFmpegTools(cacheDir)
		if err == nil {
			if probeErr != nil && !explicitFFprobe {
				tools.FFprobe = cached.FFprobe
				probeErr = nil
			}
			if needDecode && decodeErr != nil && !explicitFFmpeg {
				tools.FFmpeg = cached.FFmpeg
				decodeErr = nil
			}
			tools.Version = cached.Version
		}
	}
	if probeErr == nil && (!needDecode || decodeErr == nil) {
		return tools, nil
	}
	if !autoInstall {
		return ffmpegTools{}, missingMediaToolsError(probeErr, decodeErr, needDecode, cacheErr)
	}
	if cacheErr != nil {
		return ffmpegTools{}, fmt.Errorf("prepare FFmpeg cache location: %w", cacheErr)
	}
	if explicitFFprobe && probeErr != nil {
		return ffmpegTools{}, fmt.Errorf("explicit --ffprobe path is unavailable: %w", probeErr)
	}
	if explicitFFmpeg && needDecode && decodeErr != nil {
		return ffmpegTools{}, fmt.Errorf("explicit --ffmpeg path is unavailable: %w", decodeErr)
	}
	installed, err := installFFmpeg(cacheDir)
	if err != nil {
		return ffmpegTools{}, fmt.Errorf("prepare FFmpeg in the current-user cache: %w", err)
	}
	if probeErr != nil {
		tools.FFprobe = installed.FFprobe
	}
	if needDecode && decodeErr != nil {
		tools.FFmpeg = installed.FFmpeg
	}
	tools.Version = installed.Version
	return tools, nil
}

func resolveExecutable(name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", errors.New("executable name is empty")
	}
	resolved, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s was not found on PATH", name)
	}
	return filepath.Abs(resolved)
}

func missingMediaToolsError(probeErr, decodeErr error, needDecode bool, cacheErr error) error {
	var missing []string
	if probeErr != nil {
		missing = append(missing, "ffprobe")
	}
	if needDecode && decodeErr != nil {
		missing = append(missing, "ffmpeg")
	}
	message := "required media tools are unavailable: " + strings.Join(missing, ", ")
	if cacheErr != nil {
		message += "; " + cacheErr.Error()
	}
	if runtime.GOOS == "windows" && runtime.GOARCH == "amd64" {
		message += "; rerun with --auto-install=true to prepare a verified copy in the current-user cache"
	} else {
		message += "; install FFmpeg from https://ffmpeg.org/download.html and make ffmpeg/ffprobe available on PATH (automatic cache setup currently supports Windows x64)"
	}
	return errors.New(message)
}

func installFFmpeg(cacheDir string) (ffmpegTools, error) {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		return ffmpegTools{}, fmt.Errorf("automatic cache setup currently supports Windows x64; see https://ffmpeg.org/download.html for other platforms")
	}
	cacheDir, err := filepath.Abs(cacheDir)
	if err != nil {
		return ffmpegTools{}, fmt.Errorf("resolve cache directory: %w", err)
	}
	if cached, err := findCachedFFmpegTools(cacheDir); err == nil {
		if _, err := verifyFFmpegPair(cached.FFmpeg, cached.FFprobe, cached.Version); err == nil {
			return cached, nil
		}
	}
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return ffmpegTools{}, fmt.Errorf("create user cache directory: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	client := &http.Client{Timeout: 20 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many redirects from FFmpeg download host")
		}
		if req.URL.Scheme != "https" || req.URL.Hostname() != "www.gyan.dev" {
			return fmt.Errorf("refusing FFmpeg download redirect to %s", req.URL.Host)
		}
		return nil
	}}

	fmt.Fprintf(os.Stderr, "video-pv-cli: preparing FFmpeg in the current-user cache; downloading from %s\n", ffmpegReleaseChecksumURL)
	checksumURL, _ := url.Parse(ffmpegReleaseChecksumURL)
	checksumBody, finalURL, err := getSmallHTTPBody(ctx, client, checksumURL.String(), 4096)
	if err != nil {
		return ffmpegTools{}, fmt.Errorf("fetch upstream SHA-256: %w", err)
	}
	archiveName := path.Base(finalURL.Path)
	match := releaseArchivePattern.FindStringSubmatch(archiveName)
	if len(match) < 2 || !validReleaseVersion(match[1]) {
		return ffmpegTools{}, fmt.Errorf("unexpected checksum URL returned by upstream: %s", finalURL)
	}
	version := match[1]
	checksum, err := parseSHA256(string(checksumBody))
	if err != nil {
		return ffmpegTools{}, fmt.Errorf("parse upstream SHA-256: %w", err)
	}
	archiveURL := *finalURL
	archiveURL.Path = strings.TrimSuffix(archiveURL.Path, ".sha256")
	archiveURL.RawQuery = ""
	archiveURL.Fragment = ""
	versionDir := filepath.Join(cacheDir, "gyan-win64", version)
	finalBinDir := filepath.Join(versionDir, "bin")
	if _, err := os.Stat(versionDir); err == nil {
		ffmpegPath := filepath.Join(finalBinDir, "ffmpeg.exe")
		ffprobePath := filepath.Join(finalBinDir, "ffprobe.exe")
		if _, verifyErr := verifyFFmpegPair(ffmpegPath, ffprobePath, version); verifyErr != nil {
			return ffmpegTools{}, fmt.Errorf("cache version directory already exists but is not valid: %s: %w", versionDir, verifyErr)
		}
		metadata := ffmpegCacheMetadata{Version: version, SHA256: checksum, SourceURL: archiveURL.String(), Installed: time.Now().UTC()}
		if err := writeCacheMetadata(cacheDir, metadata); err != nil {
			return ffmpegTools{}, fmt.Errorf("repair cache metadata: %w", err)
		}
		return ffmpegTools{FFmpeg: ffmpegPath, FFprobe: ffprobePath, Version: version}, nil
	} else if !os.IsNotExist(err) {
		return ffmpegTools{}, fmt.Errorf("inspect FFmpeg version cache: %w", err)
	}
	archivePath, err := downloadArchive(ctx, client, archiveURL.String(), cacheDir, checksum, os.Stderr)
	if err != nil {
		return ffmpegTools{}, err
	}
	defer os.Remove(archivePath)

	stage, err := os.MkdirTemp(cacheDir, ".ffmpeg-stage-")
	if err != nil {
		return ffmpegTools{}, fmt.Errorf("create staging directory: %w", err)
	}
	defer os.RemoveAll(stage)
	if err := extractFFmpegArchive(archivePath, stage); err != nil {
		return ffmpegTools{}, fmt.Errorf("extract verified FFmpeg archive: %w", err)
	}
	stageBin := filepath.Join(stage, "bin")
	ffmpegPath := filepath.Join(stageBin, "ffmpeg.exe")
	ffprobePath := filepath.Join(stageBin, "ffprobe.exe")
	if _, err := verifyFFmpegPair(ffmpegPath, ffprobePath, version); err != nil {
		return ffmpegTools{}, fmt.Errorf("verify extracted FFmpeg binaries: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(versionDir), 0o700); err != nil {
		return ffmpegTools{}, fmt.Errorf("create version cache directory: %w", err)
	}
	if err := os.Rename(stage, versionDir); err != nil {
		return ffmpegTools{}, fmt.Errorf("commit FFmpeg cache directory: %w", err)
	}
	metadata := ffmpegCacheMetadata{Version: version, SHA256: checksum, SourceURL: archiveURL.String(), Installed: time.Now().UTC()}
	if err := writeCacheMetadata(cacheDir, metadata); err != nil {
		return ffmpegTools{}, fmt.Errorf("write cache metadata: %w", err)
	}
	return ffmpegTools{
		FFmpeg:  filepath.Join(finalBinDir, "ffmpeg.exe"),
		FFprobe: filepath.Join(finalBinDir, "ffprobe.exe"),
		Version: version,
	}, nil
}

func getSmallHTTPBody(ctx context.Context, client *http.Client, address string, maxBytes int64) ([]byte, *url.URL, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, nil, err
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("upstream returned HTTP %s", response.Status)
	}
	if response.Request == nil || response.Request.URL == nil || response.Request.URL.Scheme != "https" || response.Request.URL.Hostname() != "www.gyan.dev" {
		return nil, nil, errors.New("upstream response did not come from the approved HTTPS host")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return nil, nil, err
	}
	if int64(len(body)) > maxBytes {
		return nil, nil, errors.New("upstream response exceeded size limit")
	}
	return body, response.Request.URL, nil
}

func parseSHA256(value string) (string, error) {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) == 0 {
		return "", errors.New("empty checksum response")
	}
	checksum := strings.ToLower(fields[0])
	if len(checksum) != sha256.Size*2 {
		return "", errors.New("checksum must contain 64 hexadecimal characters")
	}
	if _, err := hex.DecodeString(checksum); err != nil {
		return "", errors.New("checksum is not hexadecimal")
	}
	return checksum, nil
}

func downloadArchive(ctx context.Context, client *http.Client, address, cacheDir, expectedSHA string, progress io.Writer) (string, error) {
	parsed, err := url.Parse(address)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() != "www.gyan.dev" {
		return "", errors.New("refusing archive URL outside approved HTTPS host")
	}
	first, err := requestArchiveRange(ctx, client, address, 0, 0)
	if err != nil {
		return "", err
	}
	firstStart, firstEnd, total, err := parseContentRange(first.Header.Get("Content-Range"))
	if err != nil {
		first.Body.Close()
		return "", fmt.Errorf("read FFmpeg archive size: %w", err)
	}
	if firstStart != 0 || firstEnd != 0 {
		first.Body.Close()
		return "", errors.New("upstream returned an inconsistent initial Content-Range")
	}
	if total <= 0 || total > maxFFmpegArchiveBytes {
		first.Body.Close()
		return "", fmt.Errorf("FFmpeg archive size %d is outside the allowed range", total)
	}
	temp, err := os.CreateTemp(cacheDir, ".ffmpeg-archive-*.zip")
	if err != nil {
		first.Body.Close()
		return "", fmt.Errorf("create download file: %w", err)
	}
	tempPath := temp.Name()
	copyErr := func() error {
		hasher := sha256.New()
		progressWriter := &transferProgressWriter{target: progress, next: 16 << 20}
		var written int64
		writeRange := func(response *http.Response, start, end int64) error {
			defer response.Body.Close()
			expected := end - start + 1
			n, err := io.Copy(io.MultiWriter(temp, hasher, progressWriter), io.LimitReader(response.Body, expected+1))
			if err != nil {
				return err
			}
			if n != expected {
				return fmt.Errorf("FFmpeg archive range %d-%d had %d bytes; expected %d", start, end, n, expected)
			}
			written += n
			return nil
		}
		if err := writeRange(first, 0, 0); err != nil {
			return err
		}
		for start := int64(1); start < total; {
			end := start + ffmpegDownloadChunkBytes - 1
			if end >= total {
				end = total - 1
			}
			response, err := requestArchiveRange(ctx, client, address, start, end)
			if err != nil {
				return err
			}
			responseStart, responseEnd, responseTotal, err := parseContentRange(response.Header.Get("Content-Range"))
			if err != nil || responseStart != start || responseEnd != end || responseTotal != total {
				response.Body.Close()
				return fmt.Errorf("upstream returned an inconsistent Content-Range for bytes %d-%d", start, end)
			}
			if err := writeRange(response, start, end); err != nil {
				return err
			}
			start = end + 1
		}
		if written != total {
			return fmt.Errorf("FFmpeg archive had %d bytes; expected %d", written, total)
		}
		if err := temp.Sync(); err != nil {
			return err
		}
		actual := hex.EncodeToString(hasher.Sum(nil))
		if actual != expectedSHA {
			return fmt.Errorf("FFmpeg archive SHA-256 mismatch: expected %s, got %s", expectedSHA, actual)
		}
		if progress != nil {
			fmt.Fprintf(progress, "\rDownloaded %.1f MiB; SHA-256 verified.\n", float64(written)/(1<<20))
		}
		return nil
	}()
	closeErr := temp.Close()
	if copyErr != nil {
		os.Remove(tempPath)
		return "", copyErr
	}
	if closeErr != nil {
		os.Remove(tempPath)
		return "", closeErr
	}
	return tempPath, nil
}

func requestArchiveRange(ctx context.Context, client *http.Client, address string, start, end int64) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	req.Header.Set("Accept-Encoding", "identity")
	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download FFmpeg archive range %d-%d: %w", start, end, err)
	}
	if response.StatusCode != http.StatusPartialContent {
		response.Body.Close()
		return nil, fmt.Errorf("upstream did not honor FFmpeg archive byte range %d-%d (HTTP %s)", start, end, response.Status)
	}
	if response.Request == nil || response.Request.URL == nil || response.Request.URL.Scheme != "https" || response.Request.URL.Hostname() != "www.gyan.dev" {
		response.Body.Close()
		return nil, errors.New("refusing archive response outside approved HTTPS host")
	}
	return response, nil
}

func parseContentRange(value string) (int64, int64, int64, error) {
	if !strings.HasPrefix(value, "bytes ") {
		return 0, 0, 0, errors.New("missing or invalid Content-Range header")
	}
	parts := strings.Split(strings.TrimPrefix(value, "bytes "), "/")
	if len(parts) != 2 {
		return 0, 0, 0, errors.New("invalid Content-Range header")
	}
	span := strings.Split(parts[0], "-")
	if len(span) != 2 {
		return 0, 0, 0, errors.New("invalid Content-Range span")
	}
	start, err1 := strconv.ParseInt(span[0], 10, 64)
	end, err2 := strconv.ParseInt(span[1], 10, 64)
	total, err3 := strconv.ParseInt(parts[1], 10, 64)
	if err1 != nil || err2 != nil || err3 != nil || start < 0 || end < start || total <= end {
		return 0, 0, 0, errors.New("invalid Content-Range values")
	}
	return start, end, total, nil
}

type transferProgressWriter struct {
	target io.Writer
	next   int64
	total  int64
}

func (w *transferProgressWriter) Write(value []byte) (int, error) {
	if w.target == nil {
		return len(value), nil
	}
	w.total += int64(len(value))
	if w.total >= w.next {
		if _, err := fmt.Fprintf(w.target, "\rDownloaded %.1f MiB...", float64(w.total)/(1<<20)); err != nil {
			return 0, err
		}
		w.next += 16 << 20
	}
	return len(value), nil
}

func extractFFmpegArchive(archivePath, destination string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	if len(reader.File) == 0 || len(reader.File) > maxFFmpegArchiveEntries {
		return fmt.Errorf("archive entry count is outside the allowed range")
	}
	const executableSuffix = "/bin/ffmpeg.exe"
	prefix := ""
	for _, entry := range reader.File {
		name := entry.Name
		if strings.HasSuffix(name, executableSuffix) {
			prefix = strings.TrimSuffix(name, executableSuffix)
			break
		}
	}
	if prefix == "" || strings.Contains(prefix, "\\") || path.IsAbs(prefix) || path.Clean(prefix) != prefix || prefix == "." || strings.Contains(prefix, "..") {
		return errors.New("archive does not contain a safe FFmpeg package root")
	}
	seen := make(map[string]bool)
	var total uint64
	foundFFmpeg, foundFFprobe := false, false
	for _, entry := range reader.File {
		name := entry.Name
		if strings.Contains(name, "\\") || path.IsAbs(name) {
			return fmt.Errorf("archive contains an unsafe path: %q", name)
		}
		directoryEntry := strings.HasSuffix(name, "/")
		nameWithoutSlash := strings.TrimSuffix(name, "/")
		clean := path.Clean(nameWithoutSlash)
		if clean != nameWithoutSlash || clean == "." || strings.HasPrefix(clean, "../") {
			return fmt.Errorf("archive contains an unsafe path: %q", name)
		}
		if clean == prefix || clean == prefix+"/" {
			continue
		}
		if !strings.HasPrefix(clean, prefix+"/") {
			return fmt.Errorf("archive contains a path outside package root: %q", name)
		}
		rel := strings.TrimPrefix(clean, prefix+"/")
		if rel == "" || seen[rel] {
			return fmt.Errorf("archive contains an empty or duplicate path: %q", name)
		}
		seen[rel] = true
		if directoryEntry || entry.FileInfo().IsDir() {
			if err := os.MkdirAll(filepath.Join(destination, filepath.FromSlash(rel)), 0o700); err != nil {
				return err
			}
			continue
		}
		mode := entry.Mode()
		if mode&os.ModeSymlink != 0 || (mode.Type() != 0 && !mode.IsRegular()) {
			return fmt.Errorf("archive contains a non-regular file: %q", name)
		}
		total += entry.UncompressedSize64
		if total > maxFFmpegUnpackedBytes {
			return fmt.Errorf("archive exceeds %d MiB extraction limit", maxFFmpegUnpackedBytes>>20)
		}
		target := filepath.Join(destination, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		source, err := entry.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			source.Close()
			return err
		}
		written, copyErr := io.Copy(out, io.LimitReader(source, int64(entry.UncompressedSize64)+1))
		closeSourceErr := source.Close()
		closeOutErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeSourceErr != nil {
			return closeSourceErr
		}
		if closeOutErr != nil {
			return closeOutErr
		}
		if uint64(written) != entry.UncompressedSize64 {
			return fmt.Errorf("archive entry size mismatch: %q", name)
		}
		if rel == "bin/ffmpeg.exe" {
			foundFFmpeg = true
		}
		if rel == "bin/ffprobe.exe" {
			foundFFprobe = true
		}
	}
	if !foundFFmpeg || !foundFFprobe {
		return errors.New("archive is missing bin/ffmpeg.exe or bin/ffprobe.exe")
	}
	return nil
}

func verifyFFmpegPair(ffmpegPath, ffprobePath, expectedVersion string) (string, error) {
	version, err := executableVersion(ffmpegPath)
	if err != nil {
		return "", fmt.Errorf("run ffmpeg -version: %w", err)
	}
	if expectedVersion != "" && !strings.Contains(version, "version "+expectedVersion) {
		return "", fmt.Errorf("ffmpeg reports an unexpected version: %s", version)
	}
	probeVersion, err := executableVersion(ffprobePath)
	if err != nil {
		return "", fmt.Errorf("run ffprobe -version: %w", err)
	}
	if expectedVersion != "" && !strings.Contains(probeVersion, "version "+expectedVersion) {
		return "", fmt.Errorf("ffprobe reports an unexpected version: %s", probeVersion)
	}
	return version, nil
}

func executableVersion(executable string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-version")
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("version check timed out")
		}
		return "", err
	}
	lines := strings.SplitN(strings.TrimSpace(string(output)), "\n", 2)
	if len(lines) == 0 || !strings.Contains(lines[0], " version ") {
		return "", errors.New("unrecognized FFmpeg version output")
	}
	return strings.TrimSpace(lines[0]), nil
}

func validReleaseVersion(version string) bool {
	return releaseVersionPattern.MatchString(version)
}

func writeCacheMetadata(cacheDir string, metadata ffmpegCacheMetadata) error {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	temp, err := os.CreateTemp(cacheDir, ".current-*.json")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		os.Remove(tempPath)
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		os.Remove(tempPath)
		return err
	}
	if err := temp.Close(); err != nil {
		os.Remove(tempPath)
		return err
	}
	if err := os.Rename(tempPath, filepath.Join(cacheDir, "current.json")); err != nil {
		os.Remove(tempPath)
		return err
	}
	return nil
}
