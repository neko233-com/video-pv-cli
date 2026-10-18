package main

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestSharedRuntimePreparationIsIdempotentAndProtectsChanges(t *testing.T) {
	root := t.TempDir()
	if err := prepareSharedRuntime(root); err != nil {
		t.Fatal(err)
	}
	before := snapshotFiles(t, root)
	if err := prepareSharedRuntime(root); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, snapshotFiles(t, root)) {
		t.Fatal("runtime preparation changed files")
	}
	path := filepath.Join(root, "package-lock.json")
	_ = os.WriteFile(path, []byte("custom"), 0600)
	if err := prepareSharedRuntime(root); err == nil {
		t.Fatal("modified runtime manifest accepted")
	}
	if data, _ := os.ReadFile(path); string(data) != "custom" {
		t.Fatal("modified runtime overwritten")
	}
}

func TestWindowsDefaultRuntimeUsesUserCacheThenLocalDependencies(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows cache selection")
	}
	t.Setenv("LOCALAPPDATA", t.TempDir())
	cache, err := sharedRuntimeRoot()
	if err != nil {
		t.Fatal(err)
	}
	project := t.TempDir()
	installFixture := func(root string) {
		t.Helper()
		folder := filepath.Join(root, "node_modules", "multiplane")
		if err := os.MkdirAll(folder, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(folder, "package.json"), []byte(`{"name":"multiplane","version":"0.3.10","bin":{"multiplane":"main.js"}}`), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(folder, "main.js"), []byte("// fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	installFixture(cache)
	if got, err := preparedRuntimeRoot("", project); err != nil || got != cache {
		t.Fatal("default cache unavailable", got, err)
	}
	installFixture(project)
	if got, err := preparedRuntimeRoot("", project); err != nil || got != project {
		t.Fatal("local runtime not preferred", got, err)
	}
}

func TestExplicitRuntimeAndInvalidLocalVersionNeverFallBack(t *testing.T) {
	root := t.TempDir()
	resolved, err := preparedRuntimeRoot(root, "ignored")
	if err != nil || resolved != root {
		t.Fatal("explicit dependency path changed", err)
	}
	packageRoot := filepath.Join(root, "node_modules", "multiplane")
	_ = os.MkdirAll(packageRoot, 0755)
	_ = os.WriteFile(filepath.Join(packageRoot, "package.json"), []byte(`{"name":"multiplane","version":"0.0.0","bin":{"multiplane":"main.js"}}`), 0600)
	if _, err := preparedRuntimeRoot("", root); err == nil {
		t.Fatal("wrong local package silently replaced")
	}
}
