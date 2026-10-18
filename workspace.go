package main

import (
	"archive/zip"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

//go:embed SKILL.md agents/*.yaml references/*.md assets/starter/* assets/character/* *.go go.mod README.md LICENSE NOTICE examples/*
var bundled embed.FS

func parseFlags(f *flag.FlagSet, args []string) int {
	if err := f.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if f.NArg() != 0 {
		fmt.Fprintln(f.Output(), "unexpected positional arguments")
		return 2
	}
	return -1
}

func runInit(args []string, stdout, stderr io.Writer) int {
	f := flag.NewFlagSet("init", flag.ContinueOnError)
	f.SetOutput(stderr)
	dir := f.String("dir", "my-pv", "new project directory (must not exist)")
	kind := f.String("type", "game", "game, character, or product")
	title := f.String("title", "Stellar Echo", "project title")
	if code := parseFlags(f, args); code >= 0 {
		return code
	}
	if *kind != "game" && *kind != "character" && *kind != "product" {
		fmt.Fprintln(stderr, "invalid --type")
		return 2
	}
	target, err := filepath.Abs(*dir)
	if err == nil {
		err = initializeProject(target, *kind, *title)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	fmt.Fprintf(stdout, "Created %s\nNext: video-pv-cli setup --project %q\nThen: video-pv-cli render --project %q\n", target, target, target)
	return 0
}

func initializeProject(target, kind, title string) error {
	if strings.TrimSpace(title) == "" {
		return errors.New("project title must not be empty")
	}
	if _, err := os.Lstat(target); err == nil {
		return fmt.Errorf("project already exists: %s", target)
	} else if !os.IsNotExist(err) {
		return err
	}
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(parent, ".video-pv-init-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := copyBundled("assets/starter", stage); err != nil {
		return err
	}
	if kind == "character" {
		if err := copyBundled("assets/character", stage); err != nil {
			return err
		}
	}
	if title != "Stellar Echo" {
		path := filepath.Join(stage, "pv.html")
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		source = []byte(strings.Replace(string(source), "STELLAR<span>ECHO</span>", html.EscapeString(title), 1))
		if err := os.WriteFile(path, source, 0644); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(filepath.Join(stage, "pv-project.json"))
	if err != nil {
		return err
	}
	var p ProjectFile
	if err = json.Unmarshal(data, &p); err != nil {
		return err
	}
	p.Project.Type = kind
	p.Project.Title = title
	data, err = json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(stage, "pv-project.json"), append(data, '\n'), 0644); err != nil {
		return err
	}
	return os.Rename(stage, target)
}

func copyBundled(prefix, target string) error {
	return fs.WalkDir(bundled, prefix, func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(name, prefix+"/")
		path := filepath.Join(target, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		data, err := bundled.ReadFile(name)
		if err != nil {
			return err
		}
		return os.WriteFile(path, data, 0644)
	})
}

func runSetup(args []string, stdout, stderr io.Writer) int {
	f := flag.NewFlagSet("setup", flag.ContinueOnError)
	f.SetOutput(stderr)
	project := f.String("project", ".", "PV project directory")
	skip := f.Bool("skip-browser", false, "skip Chromium installation")
	timeout := f.Duration("timeout", 15*time.Minute, "setup timeout")
	if code := parseFlags(f, args); code >= 0 {
		return code
	}
	root, err := filepath.Abs(*project)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if *timeout <= 0 {
		fmt.Fprintln(stderr, "--timeout must be positive")
		return 2
	}
	node, err := nodeExecutable()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	npm, err := npmScript()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	// Project-owned package manifests are explicit input, never downloaded via npx.
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	var pkg struct {
		Dependencies map[string]string `json:"dependencies"`
	}
	if err = json.Unmarshal(data, &pkg); err != nil || pkg.Dependencies["multiplane"] != "0.3.10" {
		fmt.Fprintln(stderr, "setup requires the starter's pinned multiplane 0.3.10 dependency; use init or install an intentionally selected renderer yourself")
		return 2
	}
	base, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	ctx, stop := context.WithTimeout(base, *timeout)
	defer stop()
	install := "install"
	if _, err := os.Stat(filepath.Join(root, "package-lock.json")); err == nil {
		install = "ci"
	}
	if err := runNodeCommand(ctx, root, node, []string{npm, install, "--ignore-scripts", "--no-audit", "--no-fund"}, stdout, stderr); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if !*skip {
		if err := runNodeCommand(ctx, root, node, []string{filepath.Join(root, "node_modules", "playwright", "cli.js"), "install", "chromium"}, stdout, stderr); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	fmt.Fprintln(stdout, "Renderer dependencies are ready. FFmpeg will be resolved by render (Windows x64 can use the user cache).")
	return 0
}

func nodeExecutable() (string, error) {
	node, err := exec.LookPath("node")
	if err != nil {
		return "", fmt.Errorf("Node.js >=24 is required: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, node, "--version").Output()
	if err != nil {
		return "", err
	}
	major, _ := strconv.Atoi(strings.Split(strings.TrimPrefix(strings.TrimSpace(string(output)), "v"), ".")[0])
	if major < 24 {
		return "", fmt.Errorf("Node.js >=24 required, found %s", strings.TrimSpace(string(output)))
	}
	return filepath.Abs(node)
}

func npmScript() (string, error) {
	npm, err := exec.LookPath("npm")
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(npm)
	if err != nil {
		resolved = npm
	}
	candidates := []string{filepath.Join(filepath.Dir(npm), "node_modules", "npm", "bin", "npm-cli.js"), resolved}
	for _, candidate := range candidates {
		if filepath.Ext(candidate) == ".js" {
			if _, err := os.Stat(candidate); err == nil {
				return filepath.Abs(candidate)
			}
		}
	}
	return "", fmt.Errorf("cannot locate npm-cli.js beside %s; install a standard Node.js distribution", npm)
}

func runNodeCommand(ctx context.Context, root, node string, args []string, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, node, args...)
	cmd.Dir = root
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	configureProcess(cmd)
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	return nil
}

func rendererPath(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "node_modules", "multiplane", "package.json"))
	if err != nil {
		return "", fmt.Errorf("local Multiplane is missing; run video-pv-cli setup --project %q", root)
	}
	var pkg struct {
		Name string            `json:"name"`
		Bin  map[string]string `json:"bin"`
	}
	if err = json.Unmarshal(data, &pkg); err != nil {
		return "", err
	}
	if pkg.Name != "multiplane" || pkg.Bin["multiplane"] == "" {
		return "", errors.New("invalid local Multiplane package")
	}
	return confinedPath(filepath.Join(root, "node_modules", "multiplane"), pkg.Bin["multiplane"])
}

func runSkill(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(stderr, "Usage: video-pv-cli skill install [--dir path] [--force] | skill pack --out skill.zip")
		if len(args) > 0 {
			return 0
		}
		return 2
	}
	action := args[0]
	f := flag.NewFlagSet("skill "+action, flag.ContinueOnError)
	f.SetOutput(stderr)
	dir := f.String("dir", "", "skill installation folder (default: CODEX_HOME/skills/pv-production)")
	out := f.String("out", "pv-production.zip", "skill archive path")
	force := f.Bool("force", false, "back up and replace an existing installation")
	if code := parseFlags(f, args[1:]); code >= 0 {
		return code
	}
	var err error
	switch action {
	case "install":
		target := *dir
		if target == "" {
			base := os.Getenv("CODEX_HOME")
			if base == "" {
				var homeErr error
				base, homeErr = os.UserHomeDir()
				if homeErr != nil {
					fmt.Fprintln(stderr, homeErr)
					return 2
				}
				base = filepath.Join(base, ".codex")
			}
			target = filepath.Join(base, "skills", "pv-production")
		}
		target, err = filepath.Abs(target)
		if err == nil {
			err = installSkill(target, *force, stdout)
		}
	case "pack":
		err = packSkill(*out)
	default:
		err = fmt.Errorf("unknown skill action %q", action)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return 0
}

func installSkill(target string, force bool, stdout io.Writer) error {
	if info, err := os.Lstat(target); err == nil {
		if !force {
			return fmt.Errorf("skill already exists: %s; use --force to create a backup and replace", target)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("refusing to replace a non-directory or symlink")
		}
		marker, readErr := os.ReadFile(filepath.Join(target, "SKILL.md"))
		if readErr != nil || !strings.Contains(string(marker), "name: pv-production") {
			return errors.New("refusing to replace an unrelated directory without a pv-production skill marker")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(target), ".pv-skill-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := copyBundled(".", stage); err != nil {
		return err
	}
	backup := ""
	if _, err := os.Stat(target); err == nil {
		backup = filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+".backup-"+strconv.FormatInt(time.Now().UnixNano(), 10))
		if err := os.Rename(target, backup); err != nil {
			return err
		}
	}
	if err := os.Rename(stage, target); err != nil {
		if backup != "" {
			_ = os.Rename(backup, target)
		}
		return err
	}
	fmt.Fprintf(stdout, "Installed pv-production at %s (%s/%s)\n", target, runtime.GOOS, runtime.GOARCH)
	if backup != "" {
		fmt.Fprintln(stdout, "Backup: "+backup)
	}
	return nil
}

func packSkill(out string) error {
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return err
	}
	file, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	w := zip.NewWriter(file)
	err = fs.WalkDir(bundled, ".", func(name string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		data, err := bundled.ReadFile(name)
		if err != nil {
			return err
		}
		entry, err := w.Create("pv-production/" + name)
		if err != nil {
			return err
		}
		_, err = entry.Write(data)
		return err
	})
	zipErr := w.Close()
	closeErr := file.Close()
	if err == nil {
		err = zipErr
	}
	if err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(out)
	}
	return err
}
