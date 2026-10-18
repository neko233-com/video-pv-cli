package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

func sharedRuntimeRoot() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "video-pv-cli", "runtime", "multiplane-0.3.10"), nil
}

func prepareSharedRuntime(root string) error {
	if err := prepareOwnedEvidence(root, "video-pv-cli/runtime/multiplane-0.3.10/1"); err != nil {
		return err
	}
	for _, name := range []string{"package.json", "package-lock.json"} {
		want, err := bundled.ReadFile("assets/starter/" + name)
		if err != nil {
			return err
		}
		path, err := projectOutputPath(root, name)
		if err != nil {
			return err
		}
		actual, err := os.ReadFile(path)
		if err == nil && !bytes.Equal(actual, want) {
			return fmt.Errorf("shared runtime %s changed; preserve this cache and select a separate local runtime", name)
		}
		if os.IsNotExist(err) {
			err = writeFileAtomic(path, want)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// Explicit dependency paths never silently resolve to a different environment.
func preparedRuntimeRoot(requested, project string) (string, error) {
	if requested != "" {
		return filepath.Abs(requested)
	}
	root, err := filepath.Abs(project)
	if err != nil {
		return "", err
	}
	if _, err := rendererPath(root); err == nil {
		return root, nil
	} else if _, statErr := os.Stat(filepath.Join(root, "node_modules", "multiplane", "package.json")); statErr == nil {
		return "", err
	}
	cache, err := sharedRuntimeRoot()
	if err == nil {
		if _, err := rendererPath(cache); err == nil {
			return cache, nil
		}
	}
	return "", fmt.Errorf("prepared Multiplane is missing; run video-pv-cli setup --shared or setup --project %q", root)
}
