package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

func parseFlags(f *flag.FlagSet, args []string) int {
	if err := f.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if f.NArg() != 0 {
		fmt.Fprintln(f.Output(), "unexpected positional arguments:", strings.Join(f.Args(), " "))
		return 2
	}
	return -1
}
func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}
func finitePositive(v float64) bool { return v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0) }
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func hashBytes(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

func projectFilePath(root, path string) (string, error) {
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("project path must be relative: %s", path)
	}
	return confinedPath(root, path)
}
func projectOutputPath(root, path string) (string, error) { return projectFilePath(root, path) }
func confinedPath(root, path string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(path))
	if clean == "." || clean == "" {
		return "", fmt.Errorf("path must name a file")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, candidate)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes project: %s", path)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		resolvedRoot = root
	}
	for p := candidate; ; p = filepath.Dir(p) {
		resolved, e := filepath.EvalSymlinks(p)
		if e == nil {
			rel, e = filepath.Rel(resolvedRoot, resolved)
			if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return "", fmt.Errorf("path resolves outside project: %s", path)
			}
			break
		}
		if !os.IsNotExist(e) {
			return "", e
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	return candidate, nil
}

var atomicReportMu sync.Mutex

func writeFileAtomic(path string, data []byte) error {
	atomicReportMu.Lock()
	defer atomicReportMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".web2video-write-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
func protectOutput(output string, inputs ...string) error {
	canonical := func(path string) string {
		p, e := filepath.Abs(path)
		if e != nil {
			return path
		}
		if resolved, e := filepath.EvalSymlinks(p); e == nil {
			p = resolved
		}
		if runtime.GOOS == "windows" {
			p = strings.ToLower(p)
		}
		return p
	}
	out := canonical(output)
	info, _ := os.Stat(output)
	for _, input := range inputs {
		if input == "" {
			continue
		}
		same := out == canonical(input)
		if in, e := os.Stat(input); e == nil && info != nil {
			same = same || os.SameFile(info, in)
		}
		if same {
			return fmt.Errorf("output would overwrite input: %s", output)
		}
	}
	return nil
}
