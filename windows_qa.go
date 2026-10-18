package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type QAStep struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Log      string `json:"log"`
	ExitCode int    `json:"exit_code"`
}
type QAReport struct {
	Schema             string         `json:"schema"`
	Passed             bool           `json:"passed"`
	Version            string         `json:"version"`
	Platform           string         `json:"platform"`
	Count              int            `json:"test_repetitions"`
	GPU                bool           `json:"gpu_integration"`
	Deep               bool           `json:"deep_catalog"`
	TestPassEvents     int            `json:"test_pass_events"`
	TestSkipEvents     int            `json:"test_skip_events"`
	TestFailEvents     int            `json:"test_fail_events"`
	RequiredTestPasses map[string]int `json:"required_test_passes"`
	Steps              []QAStep       `json:"steps"`
}

func countGoTestEvents(reader io.Reader) (pass, skip, fail int, err error) {
	scan := bufio.NewScanner(reader)
	scan.Buffer(make([]byte, 4096), 8<<20)
	for scan.Scan() {
		var event struct{ Action, Test string }
		if e := json.Unmarshal(scan.Bytes(), &event); e != nil {
			return 0, 0, 0, e
		}
		if event.Test == "" {
			continue
		}
		switch event.Action {
		case "pass":
			pass++
		case "skip":
			skip++
		case "fail":
			fail++
		}
	}
	return pass, skip, fail, scan.Err()
}

func runQA(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "windows" {
		fmt.Fprintln(stderr, "Usage: video-pv-cli qa windows --source . [--deep] [--gpu=false] [--samples a.mp4,b.mp4]")
		return 2
	}
	f := flag.NewFlagSet("qa windows", flag.ContinueOnError)
	f.SetOutput(stderr)
	source := f.String("source", ".", "repository/installed skill with Go sources")
	out := f.String("out", "work/windows-qa", "fixed owned report/log directory")
	gpu := f.Bool("gpu", true, "run actual WebGPU render and failure injection fixtures")
	deep := f.Bool("deep", false, "also run/revalidate all 2640 catalog MP4 cases")
	catalogOut := f.String("catalog-out", "work/catalog-validation", "fixed catalog cache, relative to source")
	runtimeProject := f.String("runtime-project", "", "prepared renderer; defaults to current-user cache")
	count := f.Int("count", 2, "repeat all Go tests, 1-5")
	samples := f.String("samples", "", "comma-separated standalone MP4 paths")
	if code := parseFlags(f, args[1:]); code >= 0 {
		return code
	}
	if runtime.GOOS != "windows" || *count < 1 || *count > 5 {
		fmt.Fprintln(stderr, "Windows and a count from 1 to 5 are required")
		return 2
	}
	root, err := filepath.Abs(*source)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	module, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil || !strings.HasPrefix(strings.ReplaceAll(string(module), "\r\n", "\n"), "module github.com/neko233-com/video-pv-cli\n") {
		fmt.Fprintln(stderr, "source must be this repository or its installed skill")
		return 2
	}
	output, err := filepath.Abs(*out)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if err := prepareOwnedEvidence(output, "video-pv-cli/windows-qa/1"); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	environment := []string{}
	for _, value := range os.Environ() {
		key := strings.ToUpper(strings.SplitN(value, "=", 2)[0])
		if key != "PV_MEDIA_INTEGRATION" && key != "PV_INTEGRATION_RUNTIME" && key != "PV_INTEGRATION_RASTER" {
			environment = append(environment, value)
		}
	}
	environment = append(environment, "PV_MEDIA_INTEGRATION=1", "PV_INTEGRATION_RASTER=gpu")
	renderer := ""
	if *gpu || *deep {
		renderer, err = preparedRuntimeRoot(*runtimeProject, root)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	if *gpu {
		environment = append(environment, "PV_INTEGRATION_RUNTIME="+renderer)
	}
	report := QAReport{Schema: "video-pv-cli/windows-qa/1", Version: version, Platform: runtime.GOOS + "/" + runtime.GOARCH, Count: *count, GPU: *gpu, Deep: *deep}
	save := func() error {
		raw, _ := json.MarshalIndent(report, "", "  ")
		return writeFileAtomic(filepath.Join(output, "windows-qa-report.json"), append(raw, '\n'))
	}
	step := func(name, program string, argv ...string) bool {
		fmt.Fprintln(stdout, "QA:", name)
		logPath, err := projectOutputPath(output, name+".log")
		if err != nil {
			fmt.Fprintln(stderr, err)
			return false
		}
		var log, errorLog bytes.Buffer
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
		defer cancel()
		cmd := exec.CommandContext(ctx, program, argv...)
		configureProcess(cmd)
		cmd.Dir = root
		cmd.Env = environment
		cmd.Stdout = &log
		errorLogPath, e := projectOutputPath(output, name+".stderr.log")
		if e != nil {
			fmt.Fprintln(stderr, e)
			return false
		}
		cmd.Stderr = &errorLog
		err = cmd.Run()
		if e := writeFileAtomic(logPath, log.Bytes()); e != nil {
			fmt.Fprintln(stderr, e)
			return false
		}
		if e := writeFileAtomic(errorLogPath, errorLog.Bytes()); e != nil {
			fmt.Fprintln(stderr, e)
			return false
		}
		code := 0
		if err != nil {
			code = -1
			if exit, ok := err.(*exec.ExitError); ok {
				code = exit.ExitCode()
			}
			fmt.Fprintln(stderr, name, "failed:", err, "; inspect", logPath)
		}
		report.Steps = append(report.Steps, QAStep{Name: name, Passed: err == nil, Log: filepath.Base(logPath), ExitCode: code})
		if name == "go-test" {
			input, e := os.Open(logPath)
			if e == nil {
				report.TestPassEvents, report.TestSkipEvents, report.TestFailEvents, e = countGoTestEvents(input)
				_ = input.Close()
			}
			if e != nil {
				fmt.Fprintln(stderr, e)
				return false
			}
			report.RequiredTestPasses = map[string]int{}
			scan := bufio.NewScanner(bytes.NewReader(log.Bytes()))
			scan.Buffer(make([]byte, 4096), 8<<20)
			for scan.Scan() {
				var event struct{ Action, Test string }
				_ = json.Unmarshal(scan.Bytes(), &event)
				if event.Action == "pass" && !strings.Contains(event.Test, "/") && event.Test != "" {
					report.RequiredTestPasses[event.Test]++
				}
			}
			required := []string{"TestCompleteDataCatalogAndSceneCoverage", "TestStyleCatalogHas100IndependentRecipes", "TestStylePalettesMatchDirectionAndReadableForegrounds", "TestApplyStylePreservesAssetsAndIsIdempotent", "TestPreviewConcurrentReportsAreAtomicAndLeaveNoStaging", "TestCatalogDataRevalidationRejectsChangedCompletedCases", "TestSkillInstallAndArchiveAreUsableModules", "TestDemoSourcesAreSelfContainedAndDraft", "TestIntegrationBundleIsIdempotentAndCleansStaging", "TestIntegrationPortableMP4Verification", "TestIntegrationMP4RejectsAmbiguousStreamsAndAudioDrift"}
			if *gpu {
				required = append(required, "TestIntegrationShaderMP4AndFailureGates")
			}
			for _, test := range required {
				if report.RequiredTestPasses[test] != *count {
					fmt.Fprintln(stderr, "required test did not pass each repetition:", test)
					report.Steps[len(report.Steps)-1].Passed = false
					err = fmt.Errorf("missing required test coverage")
				}
			}
		}
		if e := save(); e != nil {
			fmt.Fprintln(stderr, e)
			return false
		}
		return err == nil
	}
	if !step("go-vet", "go", "vet", "./...") || !step("go-test", "go", "test", "-json", "-timeout", "30m", "-count", strconv.Itoa(*count), "./...") || !step("catalog-data", executable, "catalog", "validate") || !step("style-data", executable, "style", "validate") {
		_ = save()
		return 1
	}
	catalogPath := *catalogOut
	if !filepath.IsAbs(catalogPath) {
		catalogPath = filepath.Join(root, catalogPath)
	}
	if *deep && !step("catalog-deep", executable, "catalog", "verify", "--runtime-project", renderer, "--out", catalogPath, "--mode", "deep", "--batch-size", "32") {
		return 1
	}
	for i, path := range strings.Split(*samples, ",") {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		if !step(fmt.Sprintf("mp4-%03d", i+1), executable, "mp4", "verify", "--input", path, "--auto-install=false") {
			return 1
		}
	}
	report.Passed = true
	if err := save(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "Windows QA passed: %d test pass events, %d skips. %s\n", report.TestPassEvents, report.TestSkipEvents, filepath.Join(output, "windows-qa-report.json"))
	return 0
}
