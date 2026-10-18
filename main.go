package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const version = "0.3.1"

func main() { os.Exit(realMain(os.Args[1:], os.Stdout, os.Stderr)) }

func runCheck(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("video-pv-cli", flag.ContinueOnError)
	flags.SetOutput(stderr)
	manifest := flags.String("manifest", "pv-project.json", "path to the PV project JSON manifest")
	media := flags.String("media", "", "media file to inspect (overrides delivery.video_path)")
	ffprobe := flags.String("ffprobe", "ffprobe", "ffprobe executable name or path")
	ffmpeg := flags.String("ffmpeg", "ffmpeg", "ffmpeg executable name or path for a full decode check")
	autoInstall := flags.Bool("auto-install", true, "prepare missing FFmpeg tools in the current-user cache (Windows x64)")
	toolsDir := flags.String("tools-dir", "", "FFmpeg cache directory override (defaults to the current-user cache)")
	installTools := flags.Bool("install-tools", false, "download and verify FFmpeg into the current-user cache, then exit")
	doctor := flags.Bool("doctor", false, "report FFmpeg availability on PATH and in the current-user cache")
	decode := flags.Bool("decode", false, "decode every video and audio frame; also enabled automatically with --release")
	reportPath := flags.String("report", "", "write a JSON report to this path")
	jsonOutput := flags.Bool("json", false, "write the report as JSON to standard output")
	release := flags.Bool("release", false, "apply release checks, including approvals and final media")
	showVersion := flags.Bool("version", false, "print version and exit")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: video-pv-cli check [options]")
		fmt.Fprintln(stderr, "Validate a PV project manifest and, when available, its final media. Missing FFmpeg tools are prepared in the current-user cache by default on Windows x64.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "video-pv-cli: positional arguments are not supported; use --manifest")
		return 2
	}
	if *showVersion {
		fmt.Fprintf(stdout, "video-pv-cli %s\n", version)
		return 0
	}
	if *installTools && *doctor {
		fmt.Fprintln(stderr, "video-pv-cli: --install-tools and --doctor cannot be used together")
		return 2
	}
	if *doctor {
		return runDoctor(stdout, stderr, *toolsDir)
	}
	if *installTools {
		cacheDir, err := resolveFFmpegCacheDir(*toolsDir)
		if err != nil {
			fmt.Fprintf(stderr, "video-pv-cli: resolve FFmpeg cache: %v\n", err)
			return 2
		}
		installed, err := installFFmpeg(cacheDir)
		if err != nil {
			fmt.Fprintf(stderr, "video-pv-cli: install FFmpeg: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "FFmpeg %s is ready in the current-user cache\nffmpeg: %s\nffprobe: %s\n", installed.Version, installed.FFmpeg, installed.FFprobe)
		return 0
	}

	visitedFlags := make(map[string]bool)
	flags.Visit(func(value *flag.Flag) { visitedFlags[value.Name] = true })

	manifestPath, err := filepath.Abs(*manifest)
	if err != nil {
		fmt.Fprintf(stderr, "video-pv-cli: resolve manifest path: %v\n", err)
		return 2
	}
	project, report := validateManifest(manifestPath, *release)
	if *reportPath != "" {
		protected := []string{manifestPath, *media}
		for _, asset := range project.Assets {
			if asset.Path != "" {
				protected = append(protected, filepath.Join(filepath.Dir(manifestPath), filepath.FromSlash(asset.Path)))
			}
		}
		if project.Delivery.VideoPath != "" {
			protected = append(protected, filepath.Join(filepath.Dir(manifestPath), filepath.FromSlash(project.Delivery.VideoPath)))
		}
		if err := protectOutput(*reportPath, protected...); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
	}
	mediaPath := *media
	mediaWasExplicit := mediaPath != ""
	if mediaPath == "" && project.Delivery.VideoPath != "" {
		mediaPath = filepath.Join(filepath.Dir(manifestPath), filepath.FromSlash(project.Delivery.VideoPath))
	}
	if mediaPath == "" && *release {
		report.add("media.required", "fail", "release mode requires a final media file; set delivery.video_path or pass --media")
	} else if mediaPath != "" {
		if !filepath.IsAbs(mediaPath) {
			mediaPath, err = filepath.Abs(mediaPath)
			if err != nil {
				report.add("media.path", "fail", "could not resolve media path: "+err.Error())
			}
		}
		if err == nil {
			info, statErr := os.Stat(mediaPath)
			switch {
			case statErr != nil:
				status := "warning"
				if *release || mediaWasExplicit {
					status = "fail"
				}
				report.add("media.missing", status, "media file not found: "+mediaPath)
			case info.IsDir():
				report.add("media.not_file", "fail", "media path points to a directory: "+mediaPath)
			default:
				tools, toolsErr := resolveMediaTools(*ffprobe, *ffmpeg, *decode || *release, *autoInstall, *toolsDir, visitedFlags["ffprobe"], visitedFlags["ffmpeg"])
				if toolsErr != nil {
					report.add("media.tools", "fail", toolsErr.Error())
					break
				}
				report.add("media.tools", "pass", "ffprobe and requested ffmpeg tools are available")
				mediaSummary, probeErr := inspectMedia(mediaPath, tools.FFprobe)
				if probeErr != nil {
					report.add("media.probe", "fail", probeErr.Error())
				} else {
					report.Media = mediaSummary
					compareMedia(&report, project, *mediaSummary)
					if *decode || *release {
						if strings.EqualFold(filepath.Ext(mediaPath), ".mp4") {
							var integrityErr error
							report.MP4, integrityErr = inspectMP4Integrity(mediaPath, tools.FFprobe, *mediaSummary)
							if integrityErr != nil {
								report.add("media.mp4", "fail", integrityErr.Error())
							} else {
								report.add("media.mp4", "pass", "MP4 atoms, frame timestamps, cadence and decoded frame count are valid")
							}
						}
						if decodeErr := decodeMedia(mediaPath, tools.FFmpeg); decodeErr != nil {
							report.add("media.decode", "fail", decodeErr.Error())
						} else {
							report.add("media.decode", "pass", "full video and audio decode completed without errors")
						}
					}
				}
			}
		}
	} else {
		report.add("media.skipped", "warning", "no final media supplied; codec, duration, frame rate, and audio checks were skipped")
	}

	report.finish(*release)
	if *reportPath != "" {
		if err := writeJSONReport(*reportPath, report); err != nil {
			fmt.Fprintf(stderr, "video-pv-cli: write report: %v\n", err)
			return 2
		}
	}
	if *jsonOutput {
		if err := json.NewEncoder(stdout).Encode(report); err != nil {
			fmt.Fprintf(stderr, "video-pv-cli: write JSON output: %v\n", err)
			return 2
		}
	} else {
		printReport(stdout, report)
	}
	if !report.Passed {
		return 1
	}
	return 0
}

func runDoctor(stdout, stderr io.Writer, toolsDir string) int {
	cacheDir, cacheErr := resolveFFmpegCacheDir(toolsDir)
	cacheTools, cachedErr := ffmpegTools{}, errors.New("not installed in current-user cache")
	if cacheErr == nil {
		cacheTools, cachedErr = findCachedFFmpegTools(cacheDir)
		if cachedErr == nil {
			_, cachedErr = verifyFFmpegPair(cacheTools.FFmpeg, cacheTools.FFprobe, cacheTools.Version)
		}
	}
	pathFFmpeg, pathFFmpegVersion, pathFFmpegErr := resolveAndVerifyExecutable("ffmpeg")
	pathFFprobe, pathFFprobeVersion, pathFFprobeErr := resolveAndVerifyExecutable("ffprobe")
	if pathFFmpegErr == nil {
		fmt.Fprintf(stdout, "PATH ffmpeg: %s (%s)\n", pathFFmpeg, pathFFmpegVersion)
	} else {
		fmt.Fprintf(stdout, "PATH ffmpeg: unavailable (%v)\n", pathFFmpegErr)
	}
	if pathFFprobeErr == nil {
		fmt.Fprintf(stdout, "PATH ffprobe: %s (%s)\n", pathFFprobe, pathFFprobeVersion)
	} else {
		fmt.Fprintf(stdout, "PATH ffprobe: unavailable (%v)\n", pathFFprobeErr)
	}
	if cachedErr == nil {
		fmt.Fprintf(stdout, "User cache FFmpeg: %s (%s)\n", cacheTools.Version, cacheDir)
	} else {
		fmt.Fprintf(stdout, "User cache FFmpeg: unavailable (%v)\n", cachedErr)
	}
	if pathFFmpegErr == nil && pathFFprobeErr == nil {
		return 0
	}
	if cachedErr == nil {
		return 0
	}
	if cacheErr != nil {
		fmt.Fprintf(stderr, "video-pv-cli: %v\n", cacheErr)
	}
	return 1
}

func resolveAndVerifyExecutable(name string) (string, string, error) {
	executable, err := resolveExecutable(name)
	if err != nil {
		return "", "", err
	}
	version, err := executableVersion(executable)
	if err != nil {
		return executable, "", err
	}
	return executable, version, nil
}

func writeJSONReport(path string, report Report) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeFileAtomic(path, data)
}

func printReport(w io.Writer, report Report) {
	state := "PASS"
	if !report.Passed {
		state = "FAIL"
	}
	fmt.Fprintf(w, "%s: %s\n", state, report.ProjectTitle)
	for _, check := range report.Checks {
		if check.Status == "pass" {
			continue
		}
		fmt.Fprintf(w, "  %s [%s] %s\n", strings.ToUpper(check.Status), check.Code, check.Message)
	}
	if report.ReleaseMode {
		fmt.Fprintf(w, "Release gate: %t\n", report.ReleaseReady)
	}
	if report.Media != nil {
		fmt.Fprintf(w, "Media: %dx%d %.3g fps, %.3g s, container=%s, video=%s, audio=%s\n", report.Media.Width, report.Media.Height, report.Media.FPS, report.Media.DurationSeconds, displayValue(report.Media.Container), report.Media.VideoCodec, displayValue(report.Media.AudioCodec))
	}
}

func displayValue(value string) string {
	if value == "" {
		return "none"
	}
	return value
}

func isReleaseTimestamp(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	_, err := time.Parse(time.RFC3339, value)
	return err == nil
}
