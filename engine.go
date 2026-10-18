package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const engineVersion = "0.8.171"
const gsapVersion = "3.14.2"

//go:embed assets/engine/* assets/styles/*.json assets/shared/* assets/shaders/* assets/runtime/* assets/templates/*.json assets/character/* assets/catalog/transitions/*.json
var bundled embed.FS

type Engine struct {
	Root, Node, CLI, Browser string
	Tools                    ffmpegTools
}

func runtimeRoot() (string, error) {
	if value := os.Getenv("WEB2VIDEO_RUNTIME_DIR"); value != "" {
		return filepath.Abs(value)
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "web2video", "runtime", "hyperframes-"+engineVersion), nil
}
func nodeExecutable() (string, error) {
	path, err := exec.LookPath("node")
	if err != nil {
		return "", fmt.Errorf("Node.js >=24 is required: %w", err)
	}
	raw, err := exec.Command(path, "--version").Output()
	if err != nil {
		return "", err
	}
	var major int
	if _, err = fmt.Sscanf(strings.TrimSpace(string(raw)), "v%d.", &major); err != nil || major < 24 {
		return "", fmt.Errorf("Node.js >=24 required; found %s", raw)
	}
	return filepath.Abs(path)
}
func npmScript(node string) (string, error) {
	npm, err := exec.LookPath("npm")
	if err != nil {
		return "", err
	}
	if resolved, e := filepath.EvalSymlinks(npm); e == nil && strings.HasSuffix(resolved, ".js") {
		return resolved, nil
	}
	for _, p := range []string{filepath.Join(filepath.Dir(node), "node_modules", "npm", "bin", "npm-cli.js"), filepath.Join(filepath.Dir(npm), "node_modules", "npm", "bin", "npm-cli.js")} {
		if info, e := os.Stat(p); e == nil && info.Mode().IsRegular() {
			return p, nil
		}
	}
	return "", fmt.Errorf("cannot locate npm-cli.js; install a standard Node.js distribution")
}
func engineReady(root string) error {
	for name, want := range map[string]string{"hyperframes": engineVersion, "gsap": gsapVersion} {
		raw, err := os.ReadFile(filepath.Join(root, "node_modules", name, "package.json"))
		var p struct{ Name, Version string }
		if err != nil || json.Unmarshal(raw, &p) != nil || p.Version != want || p.Name != name {
			return fmt.Errorf("missing or changed %s@%s; run web2video setup", name, want)
		}
	}
	for _, rel := range []string{"node_modules/hyperframes/bin/hyperframes.mjs", "node_modules/hyperframes/dist/hyperframe-runtime.js", "node_modules/hyperframes/dist/hyperframe.runtime.iife.js", "node_modules/gsap/dist/gsap.min.js"} {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			return fmt.Errorf("runtime file missing: %s; run web2video setup --repair", rel)
		}
	}
	return nil
}
func engineFingerprint(root string) (string, error) {
	entries := []string{"package-lock.json", "node_modules/hyperframes/package.json", "node_modules/hyperframes/bin/hyperframes.mjs", "node_modules/gsap/dist/gsap.min.js"}
	matches, err := filepath.Glob(filepath.Join(root, "node_modules", "hyperframes", "dist", "*.js"))
	if err != nil {
		return "", err
	}
	for _, path := range matches {
		rel, _ := filepath.Rel(root, path)
		entries = append(entries, rel)
	}
	sort.Strings(entries)
	var value strings.Builder
	for _, path := range entries {
		sum, e := fileSHA256(filepath.Join(root, path))
		if e != nil {
			return "", e
		}
		fmt.Fprintf(&value, "%s %s\n", filepath.ToSlash(path), sum)
	}
	return hashBytes([]byte(value.String())), nil
}
func findBrowser() string {
	if value := os.Getenv("WEB2VIDEO_BROWSER"); value != "" {
		return value
	}
	if value := os.Getenv("PRODUCER_HEADLESS_SHELL_PATH"); value != "" {
		return value
	}
	if home, err := os.UserHomeDir(); err == nil {
		pattern := filepath.Join(home, ".cache", "hyperframes", "chrome", "chrome-headless-shell", "*", "chrome-headless-shell-win64", "chrome-headless-shell.exe")
		matches, _ := filepath.Glob(pattern)
		if len(matches) > 0 {
			return matches[len(matches)-1]
		}
	}
	return ""
}
func preparedEngine() (Engine, error) {
	var result Engine
	root, err := runtimeRoot()
	if err != nil {
		return result, err
	}
	if err = engineReady(root); err != nil {
		return result, err
	}
	if err = ensureWindowsBridge(root); err != nil {
		return result, err
	}
	if err = ensureArtifactBridge(root); err != nil {
		return result, err
	}
	node, err := nodeExecutable()
	if err != nil {
		return result, err
	}
	tools, err := resolveMediaTools("ffprobe", "ffmpeg", true, true, "", false, false)
	if err != nil {
		return result, err
	}
	return Engine{root, node, filepath.Join(root, "node_modules", "hyperframes", "bin", "hyperframes.mjs"), findBrowser(), tools}, nil
}
func withEnv(env []string, key, value string) []string {
	result := make([]string, 0, len(env)+1)
	for _, s := range env {
		if !strings.EqualFold(strings.SplitN(s, "=", 2)[0], key) {
			result = append(result, s)
		}
	}
	return append(result, key+"="+value)
}
func (e Engine) environment() []string {
	env := os.Environ()
	for key, value := range map[string]string{"WEB2VIDEO_EPHEMERAL_ARTIFACTS": "1", "HYPERFRAMES_NO_TELEMETRY": "1", "DO_NOT_TRACK": "1", "PRODUCER_BROWSER_GPU_MODE": "hardware", "PRODUCER_ALLOW_SOFTWARE_WEBGPU": "false", "PRODUCER_EXPERIMENTAL_FAST_CAPTURE": "false"} {
		env = withEnv(env, key, value)
	}
	if e.Browser != "" {
		env = withEnv(env, "PRODUCER_HEADLESS_SHELL_PATH", e.Browser)
		env = withEnv(env, "HYPERFRAMES_BROWSER_PATH", e.Browser)
	}
	if e.Tools.FFmpeg != "" {
		env = withEnv(env, "PATH", filepath.Dir(e.Tools.FFmpeg)+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	return env
}
func (e Engine) run(ctx context.Context, root string, args []string, out, errOut io.Writer) (runErr error) {
	if len(args) > 0 && contains([]string{"render", "check", "snapshot", "capture"}, args[0]) {
		if err := validateBrowserAssets(e.Browser); err != nil {
			return err
		}
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	owned := filepath.Join(cache, "web2video", "jobs")
	if err = os.MkdirAll(owned, 0755); err != nil {
		return err
	}
	job, err := os.MkdirTemp(owned, "job-")
	if err != nil {
		return err
	}
	owner, _ := json.Marshal(struct {
		PID int `json:"pid"`
	}{os.Getpid()})
	if err = os.WriteFile(filepath.Join(job, "owner.json"), owner, 0600); err != nil {
		_ = os.RemoveAll(job)
		return err
	}
	defer func() {
		var cleanupErr error
		for attempt := 0; attempt < 4; attempt++ {
			cleanupErr = os.RemoveAll(job)
			if cleanupErr == nil {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		if runErr == nil {
			runErr = fmt.Errorf("could not clean owned runtime job %s: %w", job, cleanupErr)
		}
	}()
	cmd := exec.CommandContext(ctx, e.Node, append([]string{e.CLI}, args...)...)
	cmd.Dir = root
	cmd.Env = e.environment()
	for key, value := range map[string]string{"HYPERFRAMES_HISTORY_ROOT": filepath.Join(job, "history"), "TEMP": job, "TMP": job, "TMPDIR": job} {
		cmd.Env = withEnv(cmd.Env, key, value)
	}
	cmd.Stdout = out
	cmd.Stderr = errOut
	configureProcess(cmd)
	return cmd.Run()
}
func runSetup(args []string, out, errOut io.Writer) int {
	f := flag.NewFlagSet("setup", flag.ContinueOnError)
	f.SetOutput(errOut)
	repair := f.Bool("repair", false, "reinstall the pinned npm runtime before applying verified compatibility bridges")
	if code := parseFlags(f, args); code >= 0 {
		return code
	}
	root, err := runtimeRoot()
	if err != nil {
		return commandError(errOut, err)
	}
	node, err := nodeExecutable()
	if err != nil {
		return commandError(errOut, err)
	}
	if err = os.MkdirAll(root, 0755); err != nil {
		return commandError(errOut, err)
	}
	for _, name := range []string{"package.json", "package-lock.json"} {
		want, _ := bundled.ReadFile("assets/engine/" + name)
		old, e := os.ReadFile(filepath.Join(root, name))
		if e == nil && !bytes.Equal(old, want) {
			return commandError(errOut, fmt.Errorf("runtime manifest changed: %s; use a separate WEB2VIDEO_RUNTIME_DIR", name))
		}
		if os.IsNotExist(e) {
			if e = writeFileAtomic(filepath.Join(root, name), want); e != nil {
				return commandError(errOut, e)
			}
		} else if e != nil {
			return commandError(errOut, e)
		}
	}
	if *repair || engineReady(root) != nil {
		npm, e := npmScript(node)
		if e != nil {
			return commandError(errOut, e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, node, npm, "ci", "--ignore-scripts", "--no-audit", "--no-fund", "--registry=https://registry.npmjs.org", "--fetch-retries=5")
		cmd.Dir = root
		cmd.Stdout = out
		cmd.Stderr = errOut
		configureProcess(cmd)
		if e = cmd.Run(); e != nil {
			return commandError(errOut, e)
		}
	}
	if err = engineReady(root); err != nil {
		return commandError(errOut, err)
	}
	e, err := preparedEngine()
	if err != nil {
		return commandError(errOut, err)
	}
	if e.Browser == "" || validateBrowserAssets(e.Browser) != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		browserArgs := []string{"browser", "ensure"}
		if e.Browser != "" {
			browserArgs = append(browserArgs, "--force")
		}
		e.Browser = ""
		if err = e.run(ctx, root, browserArgs, out, errOut); err != nil {
			return commandError(errOut, err)
		}
		e.Browser = findBrowser()
	}
	fmt.Fprintf(out, "Ready: HyperFrames %s + GSAP %s\nRuntime: %s\nBrowser: %s\nFFmpeg: %s\n", engineVersion, gsapVersion, e.Root, e.Browser, e.Tools.FFmpeg)
	return 0
}

func validateBrowserAssets(path string) error {
	if path == "" {
		return fmt.Errorf("render browser missing; run web2video setup")
	}
	if !strings.EqualFold(filepath.Base(path), "chrome-headless-shell.exe") {
		if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("browser executable missing")
		}
		return nil
	}
	for _, name := range []string{"chrome-headless-shell.exe", "icudtl.dat", "libEGL.dll", "libGLESv2.dll", "v8_context_snapshot.bin", "headless_lib_data.pak", "headless_lib_strings.pak", "headless_command_resources.pak"} {
		info, err := os.Stat(filepath.Join(filepath.Dir(path), name))
		if err != nil || !info.Mode().IsRegular() || info.Size() < 64 {
			return fmt.Errorf("render browser is incomplete (%s); run web2video setup to repair it", name)
		}
	}
	return nil
}
func commandError(w io.Writer, err error) int { fmt.Fprintln(w, "web2video:", err); return 1 }
func splitProject(args []string) (string, []string, error) {
	root := "."
	result := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--project" {
			i++
			if i >= len(args) {
				return "", nil, fmt.Errorf("--project needs a directory")
			}
			root = args[i]
		} else if strings.HasPrefix(arg, "--project=") {
			root = strings.TrimPrefix(arg, "--project=")
		} else {
			result = append(result, arg)
		}
	}
	abs, err := filepath.Abs(root)
	return abs, result, err
}
func runNative(command string, args []string, out, errOut io.Writer) int {
	root, nativeArgs, err := splitProject(args)
	if err != nil {
		return commandError(errOut, err)
	}
	engine, err := preparedEngine()
	if err != nil {
		return commandError(errOut, err)
	}
	if command != "catalog" && command != "doctor" {
		if err = validateComposition(root); err != nil {
			return commandError(errOut, err)
		}
	}
	if command == "preview" {
		nativeArgs = append(nativeArgs, "--foreground", "--no-open")
	}
	if command == "check" {
		nativeArgs = append(nativeArgs, "--browser-gpu")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err = engine.run(ctx, root, append([]string{command}, nativeArgs...), out, errOut); err != nil {
		return commandError(errOut, err)
	}
	return 0
}
