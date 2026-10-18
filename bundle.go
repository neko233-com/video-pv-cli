package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type DeliveryBundle struct {
	Schema         string            `json:"schema"`
	Passed         bool              `json:"passed"`
	Target         string            `json:"target"`
	SourceSHA256   string            `json:"source_sha256"`
	ManifestSHA256 string            `json:"manifest_sha256"`
	RecipeSHA256   string            `json:"recipe_sha256"`
	Files          map[string]string `json:"files"`
	Loop           bool              `json:"loop"`
	Clickable      bool              `json:"clickable"`
	Media          *MediaSummary     `json:"media"`
	MP4            *MP4Integrity     `json:"mp4"`
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func runBundle(args []string, stdout, stderr io.Writer) int {
	f := flag.NewFlagSet("bundle", flag.ContinueOnError)
	f.SetOutput(stderr)
	project := f.String("project", ".", "source PV project")
	target := f.String("target", "mp4", "mp4 or website; engine-independent media delivery")
	mediaOverride := f.String("media", "", "source MP4 override")
	out := f.String("out", "", "owned output directory (default project/deliverables/TARGET)")
	if code := parseFlags(f, args); code >= 0 {
		return code
	}
	if *target != "website" && *target != "mp4" {
		fmt.Fprintln(stderr, "target must be mp4 or website")
		return 2
	}
	root, err := filepath.Abs(*project)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	manifestPath := filepath.Join(root, "pv-project.json")
	p, report := validateManifest(manifestPath, false)
	if reportHasFailures(report) {
		printReport(stderr, report)
		return 1
	}
	source, err := projectFilePath(root, p.Delivery.VideoPath)
	if *mediaOverride != "" {
		source = *mediaOverride
		if !filepath.IsAbs(source) {
			source, err = projectFilePath(root, source)
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if *target == "website" {
		data, e := os.ReadFile(filepath.Join(root, "template.json"))
		var t TemplateData
		if e != nil || decodeStrict(data, &t) != nil || !t.Loop {
			fmt.Fprintln(stderr, "website background bundles require a verified periodic website template")
			return 1
		}
	}
	tools, err := resolveMediaTools("ffprobe", "ffmpeg", true, true, "", false, false)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	input, err := inspectMedia(source, tools.FFprobe)
	if err == nil {
		compareMedia(&report, p, *input)
		_, err = inspectMP4Integrity(source, tools.FFprobe, *input)
		if err == nil {
			err = decodeMedia(source, tools.FFmpeg)
		}
	}
	if err != nil || reportHasFailures(report) {
		fmt.Fprintln(stderr, "source MP4 failed media validation", err)
		return 1
	}
	if input.VideoCodec != "h264" || input.PixelFormat != "yuv420p" {
		fmt.Fprintln(stderr, "bundle requires H.264/yuv420p MP4")
		return 1
	}
	if *out == "" {
		*out = filepath.Join(root, "deliverables", *target)
	}
	destination, err := filepath.Abs(*out)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if err := prepareOwnedEvidence(destination, "video-pv-cli/delivery/"+*target+"/1"); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	sourceHash, err := fileSHA256(source)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	manifestHash, err := fileSHA256(manifestPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	recipe := sha256.New()
	_, _ = io.WriteString(recipe, version+"/delivery/1/"+*target)
	for _, name := range []string{"background.html"} {
		data, _ := bundled.ReadFile("assets/delivery/" + name)
		_, _ = recipe.Write(data)
	}
	recipeHash := hex.EncodeToString(recipe.Sum(nil))
	priorData, _ := os.ReadFile(filepath.Join(destination, "bundle-report.json"))
	var prior DeliveryBundle
	if json.Unmarshal(priorData, &prior) == nil && prior.Passed && prior.Target == *target && prior.SourceSHA256 == sourceHash && prior.ManifestSHA256 == manifestHash && prior.RecipeSHA256 == recipeHash {
		unchanged := len(prior.Files) > 0
		for rel, want := range prior.Files {
			path, e := projectFilePath(destination, rel)
			if e != nil {
				unchanged = false
				break
			}
			got, e := fileSHA256(path)
			if e != nil || got != want {
				unchanged = false
				break
			}
		}
		if unchanged {
			fmt.Fprintln(stdout, "Bundle is current; validated inputs and reused existing files:", destination)
			return 0
		}
	}
	stage, err := os.MkdirTemp(filepath.Dir(destination), ".pv-bundle-")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer os.RemoveAll(stage)
	videoName := "background.mp4"
	if *target == "mp4" {
		videoName = "video.mp4"
	}
	videoPath := filepath.Join(stage, filepath.FromSlash(videoName))
	if err := os.MkdirAll(filepath.Dir(videoPath), 0755); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	run := func(args ...string) error {
		cmd := exec.CommandContext(ctx, tools.FFmpeg, args...)
		configureProcess(cmd)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("FFmpeg delivery: %v %s", err, output)
		}
		return nil
	}
	ffArgs := []string{"-nostdin", "-v", "error", "-n", "-i", source, "-map", "0:v:0", "-c:v", "copy"}
	if *target == "website" {
		ffArgs = append(ffArgs, "-an")
	} else {
		ffArgs = append(ffArgs, "-map", "0:a?", "-c:a", "copy")
	}
	ffArgs = append(ffArgs, "-movflags", "+faststart", videoPath)
	if err := run(ffArgs...); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := run("-nostdin", "-v", "error", "-n", "-i", videoPath, "-frames:v", "1", filepath.Join(stage, "poster.png")); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	files := []string{videoName, "poster.png"}
	if *target == "website" {
		data, _ := bundled.ReadFile("assets/delivery/background.html")
		data = []byte(strings.ReplaceAll(string(data), "{{TITLE}}", html.EscapeString(p.Project.Title)))
		if err := os.WriteFile(filepath.Join(stage, "background.html"), data, 0644); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		files = append(files, "background.html")
	}
	result, err := inspectMedia(videoPath, tools.FFprobe)
	var integrity *MP4Integrity
	if err == nil {
		integrity, err = inspectMP4Integrity(videoPath, tools.FFprobe, *result)
		if err == nil {
			err = decodeMedia(videoPath, tools.FFmpeg)
		}
		if err == nil && !integrity.FastStart {
			err = fmt.Errorf("delivery MP4 lacks fast-start")
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	var outputReport Report
	expected := p
	if *target == "website" {
		expected.Project.AudioRequired = false
		expected.Delivery.AudioCodec = ""
	}
	compareMedia(&outputReport, expected, *result)
	if reportHasFailures(outputReport) {
		printReport(stderr, outputReport)
		return 1
	}
	if *target == "website" && result.AudioCodec != "" {
		fmt.Fprintln(stderr, "background audio was not removed")
		return 1
	}
	bundle := DeliveryBundle{Schema: "video-pv-cli/delivery/1", Passed: true, Target: *target, SourceSHA256: sourceHash, ManifestSHA256: manifestHash, RecipeSHA256: recipeHash, Files: map[string]string{}, Loop: *target == "website", Clickable: false, Media: result}
	bundle.MP4 = integrity
	for _, rel := range files {
		path := filepath.Join(stage, filepath.FromSlash(rel))
		dest, e := projectOutputPath(destination, rel)
		if e != nil {
			fmt.Fprintln(stderr, e)
			return 1
		}
		if e := protectOutput(dest, source, manifestPath); e != nil {
			fmt.Fprintln(stderr, e)
			return 1
		}
		data, e := os.ReadFile(path)
		if e == nil {
			e = writeFileAtomic(dest, data)
		}
		if e != nil {
			fmt.Fprintln(stderr, e)
			return 1
		}
		bundle.Files[rel], _ = fileSHA256(dest)
	}
	bundle.Media.Path = filepath.Join(destination, filepath.FromSlash(videoName))
	data, _ := json.MarshalIndent(bundle, "", "  ")
	if err := writeFileAtomic(filepath.Join(destination, "bundle-report.json"), append(data, '\n')); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, "Validated delivery bundle:", destination)
	return 0
}
