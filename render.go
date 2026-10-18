package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"time"
)

type RenderReceipt struct {
	Schema       string        `json:"schema"`
	Fingerprint  string        `json:"fingerprint"`
	OutputSHA256 string        `json:"output_sha256"`
	Media        *MediaSummary `json:"media"`
	MP4          *MP4Integrity `json:"mp4"`
}

func verifyRendered(path string, e Engine, p *Project) (*MediaSummary, *MP4Integrity, error) {
	before, err := fileSHA256(path)
	if err != nil {
		return nil, nil, err
	}
	media, err := inspectMedia(path, e.Tools.FFprobe)
	if err != nil {
		return nil, nil, err
	}
	integrity, err := inspectMP4Integrity(path, e.Tools.FFprobe, *media)
	if err != nil {
		return nil, nil, err
	}
	if !integrity.FastStart {
		return nil, nil, fmt.Errorf("MP4 is not fast-start")
	}
	if p != nil {
		if media.Width != p.Width || media.Height != p.Height || math.Abs(media.FPS-float64(p.FPS)) > .001 || math.Abs(media.DurationSeconds-p.Duration) > 1/float64(p.FPS)+.01 {
			return nil, nil, fmt.Errorf("render differs from project dimensions, fps or duration")
		}
		if p.Loop && media.AudioStreams != 0 {
			return nil, nil, fmt.Errorf("background loop must be silent")
		}
	}
	if err = decodeMedia(path, e.Tools.FFmpeg); err != nil {
		return nil, nil, err
	}
	if p != nil {
		for _, scene := range p.Scenes {
			pixels, e := sampleLuma(path, e.Tools.FFmpeg, scene.Start+scene.Duration*.5)
			if e != nil {
				return nil, nil, e
			}
			if !hasFrameContent(pixels) {
				return nil, nil, fmt.Errorf("scene %s is blank/flat at its midpoint", scene.ID)
			}
		}
	}
	after, err := fileSHA256(path)
	if err != nil || before != after {
		return nil, nil, fmt.Errorf("MP4 changed during verification")
	}
	return media, integrity, nil
}

func sampleLuma(path, ffmpeg string, at float64) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-ss", fmt.Sprintf("%.9f", at), "-i", path, "-frames:v", "1", "-vf", "scale=96:54,format=gray", "-f", "rawvideo", "-")
	configureProcess(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	raw, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("pixel sample failed: %v %s", err, stderr.String())
	}
	if len(raw) != 96*54 {
		return nil, fmt.Errorf("pixel sample has no full frame")
	}
	return raw, nil
}
func hasFrameContent(pixels []byte) bool {
	if len(pixels) == 0 {
		return false
	}
	low, high := byte(255), byte(0)
	for _, v := range pixels {
		if v < low {
			low = v
		}
		if v > high {
			high = v
		}
	}
	return int(high)-int(low) >= 16
}
func acquireJob(root string) (func(), error) {
	lockPath, err := confinedPath(root, ".web2video/job.lock")
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(lockPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "job.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("project already has an active job (%s); after a forced termination, remove the lock only after checking that its process has exited", path)
	}
	fmt.Fprintf(f, "pid=%d\n", os.Getpid())
	f.Close()
	return func() { _ = os.Remove(path) }, nil
}
func runRender(args []string, out, errOut io.Writer) int {
	f := flag.NewFlagSet("render", flag.ContinueOnError)
	f.SetOutput(errOut)
	dir := f.String("project", ".", "project directory")
	output := f.String("out", "renders/master.mp4", "project-relative MP4 under renders/")
	quality := f.String("quality", "looks", "draft, looks, delivery")
	force := f.Bool("force", false, "replace a stale output after a successful staged render")
	if code := parseFlags(f, args); code >= 0 {
		return code
	}
	if !contains([]string{"draft", "looks", "delivery"}, *quality) {
		return commandError(errOut, fmt.Errorf("invalid quality"))
	}
	root, err := filepath.Abs(*dir)
	if err != nil {
		return commandError(errOut, err)
	}
	if !strings.HasPrefix(filepath.ToSlash(filepath.Clean(*output)), "renders/") || !strings.EqualFold(filepath.Ext(*output), ".mp4") {
		return commandError(errOut, fmt.Errorf("--out must be a project-relative renders/*.mp4 path"))
	}
	target, err := projectOutputPath(root, *output)
	if err != nil {
		return commandError(errOut, err)
	}
	unlock, err := acquireJob(root)
	if err != nil {
		return commandError(errOut, err)
	}
	defer unlock()
	if err = validateComposition(root); err != nil {
		return commandError(errOut, err)
	}
	e, err := preparedEngine()
	if err != nil {
		return commandError(errOut, err)
	}
	source, err := sourceFingerprint(root)
	if err != nil {
		return commandError(errOut, err)
	}
	engineHash, err := engineFingerprint(e.Root)
	if err != nil {
		return commandError(errOut, err)
	}
	browserHash := ""
	if e.Browser != "" {
		browserHash, err = fileSHA256(e.Browser)
		if err != nil {
			return commandError(errOut, err)
		}
	}
	fingerprint := hashBytes([]byte(version + source + engineHash + browserHash + *quality + *output))
	receiptPath := filepath.Join(root, ".web2video", "render-"+hashBytes([]byte(*output))[:16]+".json")
	var project *Project
	if _, err = os.Stat(filepath.Join(root, "project.json")); err == nil {
		p, e := readProject(root)
		if e != nil {
			return commandError(errOut, e)
		}
		project = &p
		if err = validateCompiledProject(root, p); err != nil {
			return commandError(errOut, err)
		}
	}
	if _, err = os.Stat(target); err == nil {
		raw, _ := os.ReadFile(receiptPath)
		var receipt RenderReceipt
		_ = json.Unmarshal(raw, &receipt)
		sum, _ := fileSHA256(target)
		if receipt.Fingerprint == fingerprint && receipt.OutputSHA256 == sum {
			if _, _, err = verifyRendered(target, e, project); err != nil {
				return commandError(errOut, err)
			}
			fmt.Fprintln(out, "Reused and fully verified:", target)
			return 0
		}
		if !*force {
			return commandError(errOut, fmt.Errorf("output exists with changed inputs; choose another --out or explicitly use --force"))
		}
	} else if !os.IsNotExist(err) {
		return commandError(errOut, err)
	}
	if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return commandError(errOut, err)
	}
	stage, err := os.MkdirTemp(filepath.Join(root, ".web2video"), "render-")
	if err != nil {
		return commandError(errOut, err)
	}
	defer os.RemoveAll(stage)
	movie := filepath.Join(stage, "master.mp4")
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	ctx, timeout := context.WithTimeout(ctx, 60*time.Minute)
	defer timeout()
	nativeArgs := []string{"render", "--output", movie, "--format", "mp4", "--quality", *quality, "--strict", "--no-best-effort", "--browser-gpu", "--no-page-side-compositing", "--frames-cache-dir", "off", "--low-memory-mode"}
	if err = e.run(ctx, root, nativeArgs, out, errOut); err != nil {
		return commandError(errOut, err)
	}
	media, integrity, err := verifyRendered(movie, e, project)
	if err != nil {
		return commandError(errOut, err)
	}
	after, err := sourceFingerprint(root)
	if err != nil || after != source {
		return commandError(errOut, fmt.Errorf("sources changed during render; staged output discarded"))
	}
	sum, err := fileSHA256(movie)
	if err != nil {
		return commandError(errOut, err)
	}
	if err = os.Rename(movie, target); err != nil {
		return commandError(errOut, err)
	}
	media.Path = target
	receipt := RenderReceipt{"web2video/render/1", fingerprint, sum, media, integrity}
	raw, _ := json.MarshalIndent(receipt, "", "  ")
	if err = writeFileAtomic(receiptPath, append(raw, '\n')); err != nil {
		return commandError(errOut, err)
	}
	fmt.Fprintf(out, "Verified: %s (%d frames, H.264/yuv420p, full decode)\n", target, integrity.DecodedFrames)
	return 0
}

func runSnapshot(args []string, out, errOut io.Writer) int {
	f := flag.NewFlagSet("snapshot", flag.ContinueOnError)
	f.SetOutput(errOut)
	dir := f.String("project", ".", "project directory")
	at := f.String("at", "1,4.5,7.5,10.5", "comma-separated seconds")
	if code := parseFlags(f, args); code >= 0 {
		return code
	}
	root, err := filepath.Abs(*dir)
	if err != nil {
		return commandError(errOut, err)
	}
	unlock, err := acquireJob(root)
	if err != nil {
		return commandError(errOut, err)
	}
	defer unlock()
	if err = validateComposition(root); err != nil {
		return commandError(errOut, err)
	}
	e, err := preparedEngine()
	if err != nil {
		return commandError(errOut, err)
	}
	stage, err := os.MkdirTemp(filepath.Join(root, ".web2video"), "snapshots-")
	if err != nil {
		return commandError(errOut, err)
	}
	defer os.RemoveAll(stage)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err = e.run(ctx, root, []string{"snapshot", "--output", stage, "--at", *at, "--no-end", "--keep", "--describe=false", "--browser-gpu"}, out, errOut); err != nil {
		return commandError(errOut, err)
	}
	target := filepath.Join(root, ".web2video", "snapshots")
	if err = os.RemoveAll(target); err != nil {
		return commandError(errOut, err)
	}
	if err = os.Rename(stage, target); err != nil {
		return commandError(errOut, err)
	}
	fmt.Fprintln(out, "Snapshots:", target)
	return 0
}
func runQA(args []string, out, errOut io.Writer) int {
	f := flag.NewFlagSet("qa", flag.ContinueOnError)
	f.SetOutput(errOut)
	integration := f.Bool("media", false, "include FFmpeg positive and negative fixtures")
	gpu := f.Bool("gpu", false, "include real WebGPU render, pixels, idempotence and shader failure fixtures")
	catalog := f.Bool("catalog", false, "capture all 100 style and 76 transition presets, plus exact loop endpoints")
	if code := parseFlags(f, args); code >= 0 {
		return code
	}
	if err := validateQACheckout("."); err != nil {
		return commandError(errOut, err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	cmd := newQACommand(ctx)
	cmd.Env = os.Environ()
	if *integration {
		cmd.Env = withEnv(cmd.Env, "PV_MEDIA_INTEGRATION", "1")
	}
	if *gpu {
		cmd.Env = withEnv(cmd.Env, "WEB2VIDEO_GPU_INTEGRATION", "1")
	}
	if *catalog {
		cmd.Env = withEnv(cmd.Env, "WEB2VIDEO_CATALOG_INTEGRATION", "1")
	}
	cmd.Stdout = out
	cmd.Stderr = errOut
	configureProcess(cmd)
	if err := cmd.Run(); err != nil {
		return commandError(errOut, err)
	}
	return 0
}

func newQACommand(ctx context.Context) *exec.Cmd {
	return exec.CommandContext(ctx, "go", "test", "-count=1", "./...")
}

func validateQACheckout(root string) error {
	raw, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return fmt.Errorf("qa requires the web2video source checkout; use check and verify for video projects")
	}
	tokens := strings.Fields(string(raw))
	if len(tokens) < 2 || tokens[0] != "module" || tokens[1] != "github.com/neko233-com/web2video" {
		return fmt.Errorf("qa only runs tests in the web2video source checkout")
	}
	return nil
}
