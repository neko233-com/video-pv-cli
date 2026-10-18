package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type RenderEvent struct {
	Schema    string          `json:"schema"`
	Type      string          `json:"type"`
	Command   string          `json:"command"`
	OK        bool            `json:"ok"`
	Code      string          `json:"code"`
	Message   string          `json:"message"`
	Hint      string          `json:"hint,omitempty"`
	Details   json.RawMessage `json:"details,omitempty"`
	JobID     string          `json:"jobId"`
	Artifacts []struct {
		Path string `json:"path"`
		Kind string `json:"kind"`
	} `json:"artifacts"`
	Data json.RawMessage `json:"data,omitempty"`
}
type RenderStep struct {
	Command string            `json:"command"`
	Result  RenderEvent       `json:"result"`
	Notices []json.RawMessage `json:"notices,omitempty"`
}
type BuildReport struct {
	Draft       bool              `json:"draft"`
	Schema      string            `json:"schema"`
	Passed      bool              `json:"passed"`
	Error       string            `json:"error,omitempty"`
	Started     string            `json:"started_at"`
	Node        string            `json:"node"`
	Renderer    string            `json:"renderer_version,omitempty"`
	FFmpeg      string            `json:"ffmpeg,omitempty"`
	Output      string            `json:"output"`
	InputSHA256 map[string]string `json:"input_sha256"`
	Steps       []RenderStep      `json:"steps"`
	Validation  *Report           `json:"validation,omitempty"`
}

func runRendererDoctor(args []string, stdout, stderr io.Writer) int {
	f := flag.NewFlagSet("doctor", flag.ContinueOnError)
	f.SetOutput(stderr)
	project := f.String("project", ".", "PV project directory")
	toolsDir := f.String("tools-dir", "", "FFmpeg cache override")
	if code := parseFlags(f, args); code >= 0 {
		return code
	}
	code := runDoctor(stdout, stderr, *toolsDir)
	root, err := filepath.Abs(*project)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	node, err := nodeExecutable()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, "Node: "+node)
	renderer, err := rendererPath(root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	tools, err := resolveMediaTools("ffprobe", "ffmpeg", true, false, *toolsDir, false, false)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := runNodeCommand(ctx, root, node, []string{renderer, "doctor", "--ffmpeg", tools.FFmpeg}, stdout, stderr); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return code
}

func runRender(args []string, stdout, stderr io.Writer) int {
	f := flag.NewFlagSet("render", flag.ContinueOnError)
	f.SetOutput(stderr)
	projectDir := f.String("project", ".", "PV project directory")
	manifestName := f.String("manifest", "pv-project.json", "manifest path relative to project")
	shot := f.String("shot", "pv.html", "full-timeline HTML path relative to project")
	out := f.String("out", "", "MP4 output relative to project (default delivery.video_path)")
	reportName := f.String("report", "qa/render-report.json", "JSON report relative to project")
	audio := f.String("audio", "", "local mixed soundtrack relative to project")
	workers := f.Int("workers", 2, "Chromium worker count (1-16)")
	raster := f.String("raster", "gpu", "gpu or software rasterization")
	quality := f.String("quality", "standard", "fast, standard, or high sampling quality (art direction remains source-driven)")
	blur := f.Int("motion-blur", 0, "override temporal samples (1-256); 0 uses shot defaults")
	draft := f.Bool("draft", false, "low resolution preview; never release-ready")
	release := f.Bool("release", false, "also require recorded approvals and human review")
	timeout := f.Duration("timeout", 30*time.Minute, "total render pipeline timeout")
	ffmpeg := f.String("ffmpeg", "ffmpeg", "FFmpeg executable path")
	ffprobe := f.String("ffprobe", "ffprobe", "ffprobe executable path")
	toolsDir := f.String("tools-dir", "", "FFmpeg cache override")
	auto := f.Bool("auto-install", true, "allow missing Windows x64 FFmpeg setup")
	if code := parseFlags(f, args); code >= 0 {
		return code
	}
	if *workers < 1 || *workers > 16 || (*raster != "gpu" && *raster != "software") || (*quality != "standard" && *quality != "fast" && *quality != "high") || *blur < 0 || *blur > 256 || *timeout <= 0 || (*draft && *release) {
		fmt.Fprintln(stderr, "invalid workers/raster/timeout or draft combined with release")
		return 2
	}
	root, err := filepath.Abs(*projectDir)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	manifestPath, err := projectFilePath(root, *manifestName)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	p, validation := validateManifest(manifestPath, *release)
	if reportHasFailures(validation) {
		printReport(stderr, validation)
		return 1
	}
	shotPath, err := projectFilePath(root, *shot)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if *out == "" {
		*out, err = filepath.Rel(root, filepath.Join(filepath.Dir(manifestPath), filepath.FromSlash(p.Delivery.VideoPath)))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
	}
	output, err := projectOutputPath(root, *out)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if !strings.EqualFold(filepath.Ext(output), ".mp4") {
		fmt.Fprintln(stderr, "render output must end with .mp4")
		return 2
	}
	if _, err := os.Lstat(output); err == nil {
		fmt.Fprintln(stderr, "output already exists; choose a new --out path: "+output)
		return 2
	} else if !os.IsNotExist(err) {
		fmt.Fprintln(stderr, err)
		return 2
	}
	reportPath, err := projectOutputPath(root, *reportName)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	inputs := []string{manifestPath, shotPath, filepath.Join(root, "package.json"), filepath.Join(root, "package-lock.json")}
	for _, asset := range p.Assets {
		if asset.Path != "" {
			inputs = append(inputs, filepath.Join(filepath.Dir(manifestPath), filepath.FromSlash(asset.Path)))
		}
	}
	audioPath := ""
	if *audio != "" {
		audioPath, err = projectFilePath(root, *audio)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		inputs = append(inputs, audioPath)
		declared := false
		for _, asset := range p.Assets {
			path := filepath.Join(filepath.Dir(manifestPath), filepath.FromSlash(asset.Path))
			if asset.Path != "" && protectOutput(audioPath, path) != nil {
				declared = true
				break
			}
		}
		if !declared {
			fmt.Fprintln(stderr, "--audio must be declared as an asset in the project manifest")
			return 2
		}
	}
	if p.Project.AudioRequired && audioPath == "" {
		fmt.Fprintln(stderr, "manifest requires audio; pass --audio with a declared mixed soundtrack asset")
		return 2
	}
	if err := protectOutput(output, inputs...); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if err := protectOutput(reportPath, append(inputs, output)...); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	build := BuildReport{Schema: "video-pv-cli/render/1", Started: time.Now().UTC().Format(time.RFC3339), Output: output, InputSHA256: map[string]string{}}
	build.Draft = *draft
	fail := func(e error) int {
		build.Error = e.Error()
		if reportErr := writeBuildReport(reportPath, build); reportErr != nil {
			fmt.Fprintln(stderr, "write failure report:", reportErr)
		}
		fmt.Fprintln(stderr, e)
		return 1
	}
	for _, input := range inputs {
		data, err := os.ReadFile(input)
		if err != nil {
			return fail(fmt.Errorf("read render input %s: %w", input, err))
		}
		digest := sha256.Sum256(data)
		rel, _ := filepath.Rel(root, input)
		build.InputSHA256[filepath.ToSlash(rel)] = hex.EncodeToString(digest[:])
	}
	node, err := nodeExecutable()
	if err != nil {
		return fail(err)
	}
	build.Node = node
	renderer, err := rendererPath(root)
	if err != nil {
		return fail(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "node_modules", "multiplane", "package.json"))
	var pkg struct {
		Version string `json:"version"`
	}
	_ = json.Unmarshal(data, &pkg)
	build.Renderer = pkg.Version
	visited := map[string]bool{}
	f.Visit(func(v *flag.Flag) { visited[v.Name] = true })
	tools, err := resolveMediaTools(*ffprobe, *ffmpeg, true, *auto, *toolsDir, visited["ffprobe"], visited["ffmpeg"])
	if err != nil {
		return fail(err)
	}
	build.FFmpeg, _ = executableVersion(tools.FFmpeg)
	base, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	ctx, stop := context.WithTimeout(base, *timeout)
	defer stop()
	jobRoot, err := projectOutputPath(root, filepath.ToSlash(filepath.Join("renders", "jobs", fmt.Sprintf("pv-%d", time.Now().UnixNano()), "record.json")))
	if err != nil {
		return fail(err)
	}
	jobDir := filepath.Dir(jobRoot)
	if err = os.MkdirAll(jobDir, 0755); err != nil {
		return fail(err)
	}
	run := func(command string, options ...string) error {
		fmt.Fprintln(stderr, "video-pv-cli: "+command)
		arguments := []string{command, filepath.ToSlash(*shot), "--root", root, "--json"}
		arguments = append(arguments, options...)
		step, err := runMultiplane(ctx, root, node, renderer, jobDir, command, arguments, stderr)
		build.Steps = append(build.Steps, step)
		return err
	}
	if err = run("lint"); err != nil {
		return fail(err)
	}
	if err = run("inspect"); err != nil {
		return fail(err)
	}
	if err = compareShotSpec(build.Steps[len(build.Steps)-1].Result.Data, p.Project); err != nil {
		return fail(err)
	}
	common := []string{"--raster", *raster, "--quality", *quality}
	if *blur > 0 {
		common = append(common, "--motion-blur", strconv.Itoa(*blur))
	}
	if err = run("verify", append(append([]string{}, common...), "--workers", strconv.Itoa(*workers))...); err != nil {
		return fail(err)
	}
	stillArgs := append([]string{}, common...)
	stillArgs = append(stillArgs, "--every", strconv.FormatFloat(math.Max(p.Project.DurationSeconds/8, 1/p.Project.FPS), 'f', 6, 64), "--out", filepath.Join(jobDir, "stills"))
	if err = run("stills", stillArgs...); err != nil {
		return fail(err)
	}
	renderArgs := append([]string{}, common...)
	renderArgs = append(renderArgs, "--out", output, "--workers", strconv.Itoa(*workers), "--ffmpeg", tools.FFmpeg)
	if audioPath != "" {
		renderArgs = append(renderArgs, "--audio", audioPath)
	}
	if *draft {
		renderArgs = append(renderArgs, "--draft")
	}
	if err = run("render", renderArgs...); err != nil {
		return fail(err)
	}
	// Detect inputs changed during a long render, rather than claiming reproducibility.
	for rel, want := range build.InputSHA256 {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return fail(err)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != want {
			return fail(fmt.Errorf("input changed during rendering: %s", rel))
		}
	}
	var result bytes.Buffer
	checkArgs := []string{"--manifest", manifestPath, "--media", output, "--decode", "--ffmpeg", tools.FFmpeg, "--ffprobe", tools.FFprobe, "--auto-install=false", "--json"}
	if *release {
		checkArgs = append(checkArgs, "--release")
	}
	code := runCheck(checkArgs, &result, stderr)
	var final Report
	if err = json.Unmarshal(result.Bytes(), &final); err != nil {
		return fail(fmt.Errorf("read final QA: %w", err))
	}
	build.Validation = &final
	if code != 0 {
		return fail(errors.New("rendered file failed final QA; see report"))
	}
	build.Passed = true
	if err = writeBuildReport(reportPath, build); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	fmt.Fprintf(stdout, "Rendered and validated: %s\nReport: %s\nContact sheet: %s\n", output, reportPath, filepath.Join(jobDir, "stills", "_sheet.png"))
	return 0
}

func reportHasFailures(r Report) bool {
	for _, c := range r.Checks {
		if c.Status == "fail" {
			return true
		}
	}
	return false
}
func compareShotSpec(data []byte, p ProjectInfo) error {
	var d struct {
		Stage struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"stage"`
		Duration float64 `json:"duration"`
		FPS      float64 `json:"fps"`
	}
	if err := json.Unmarshal(data, &d); err != nil {
		return err
	}
	if d.Stage.Width != p.Width || d.Stage.Height != p.Height || math.Abs(d.Duration-p.DurationSeconds) > 0.001 || math.Abs(d.FPS-p.FPS) > 0.001 {
		return fmt.Errorf("HTML shot spec %dx%d %.3f fps %.3fs differs from manifest %dx%d %.3f fps %.3fs", d.Stage.Width, d.Stage.Height, d.FPS, d.Duration, p.Width, p.Height, p.FPS, p.DurationSeconds)
	}
	return nil
}

type limitedBuffer struct {
	bytes.Buffer
	Max int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.Max {
		return 0, errors.New("renderer event output exceeds limit")
	}
	return b.Buffer.Write(p)
}
func runMultiplane(ctx context.Context, root, node, renderer, jobDir, command string, args []string, stderr io.Writer) (RenderStep, error) {
	step := RenderStep{Command: command}
	log, err := os.Create(filepath.Join(jobDir, command+".jsonl"))
	if err != nil {
		return step, err
	}
	defer log.Close()
	buffer := &limitedBuffer{Max: 32 << 20}
	cmd := exec.CommandContext(ctx, node, append([]string{renderer}, args...)...)
	cmd.Dir = root
	cmd.Stdout = io.MultiWriter(log, buffer)
	cmd.Stderr = stderr
	configureProcess(cmd)
	runErr := cmd.Run()
	parseErr := parseRenderEvents(buffer.Bytes(), command, &step)
	if ctx.Err() != nil {
		return step, ctx.Err()
	}
	if parseErr != nil {
		return step, parseErr
	}
	if runErr != nil {
		return step, fmt.Errorf("Multiplane %s: %w (%s)", command, runErr, step.Result.Code)
	}
	if !step.Result.OK {
		return step, fmt.Errorf("Multiplane %s failed: %s: %s", command, step.Result.Code, step.Result.Message)
	}
	return step, nil
}

func parseRenderEvents(data []byte, command string, step *RenderStep) error {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 4096), 8<<20)
	found := false
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		if found {
			return errors.New("renderer wrote events after its final result")
		}
		var event RenderEvent
		if err := json.Unmarshal(line, &event); err != nil {
			return fmt.Errorf("invalid renderer JSON: %w", err)
		}
		if event.Schema != "multiplane/1" || event.Type == "" {
			return errors.New("unsupported renderer event schema")
		}
		if event.Type == "result" {
			if event.Command != command {
				return errors.New("renderer result command mismatch")
			}
			step.Result = event
			found = true
		} else if event.Type == "warning" || event.Type == "finding" {
			step.Notices = append(step.Notices, append(json.RawMessage(nil), line...))
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if !found {
		return errors.New("renderer did not return a final result")
	}
	return nil
}

func writeBuildReport(path string, report BuildReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, append(data, '\n'))
}

func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".pv-report-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func protectOutput(output string, inputs ...string) error {
	canonical := func(path string) string {
		abs, err := filepath.Abs(path)
		if err != nil {
			return path
		}
		if resolved, err := filepath.EvalSymlinks(abs); err == nil {
			abs = resolved
		}
		if runtime.GOOS == "windows" {
			abs = strings.ToLower(abs)
		}
		return abs
	}
	out := canonical(output)
	outInfo, _ := os.Stat(output)
	for _, input := range inputs {
		if input == "" {
			continue
		}
		same := out == canonical(input)
		if inInfo, err := os.Stat(input); err == nil && outInfo != nil {
			same = same || os.SameFile(outInfo, inInfo)
		}
		if same {
			return fmt.Errorf("output would overwrite an input: %s", output)
		}
	}
	return nil
}
