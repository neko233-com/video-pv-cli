package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// HyperFrames #5775: its Windows headless shell omits the DXC DLLs.
// Select Dawn's native shader compiler path; rendering remains WebGPU/WGSL.
// Only these exact locked upstream bytes may be changed; never patch by guess.
const bridgeOriginalSHA256 = "c22676e894b4ceceb80d91c6c68a308d5b9bd5e171bb75c688f321a1bb54c728"

var bridgeBefore = []byte("    chromeArgs.push(WEBGPU_FLAG);")
var bridgeAfterPrevious = []byte("    chromeArgs.push(WEBGPU_FLAG);\n    if (platform === \"win32\") chromeArgs.push(\"--disable-dawn-features=use_dxc\"); // web2video: HyperFrames #5775")
var bridgeAfter = []byte(string(bridgeAfterPrevious) + "\n    chromeArgs.push(\"--disable-webgl\", \"--disable-webgl2\"); // web2video: native WebGPU canvas policy")

func bridgeEngineBytes(raw []byte) ([]byte, error) {
	if bytes.Count(raw, bridgeAfter) == 1 {
		original := bytes.Replace(raw, bridgeAfter, bridgeBefore, 1)
		if hashBytes(original) == bridgeOriginalSHA256 {
			return raw, nil
		}
	}
	if bytes.Count(raw, bridgeAfterPrevious) == 1 {
		original := bytes.Replace(raw, bridgeAfterPrevious, bridgeBefore, 1)
		if hashBytes(original) == bridgeOriginalSHA256 {
			raw = original
		}
	}
	if hashBytes(raw) != bridgeOriginalSHA256 || bytes.Count(raw, bridgeBefore) != 1 {
		return nil, fmt.Errorf("HyperFrames Windows bridge refused changed engine bytes; run setup in a fresh runtime directory")
	}
	return bytes.Replace(raw, bridgeBefore, bridgeAfter, 1), nil
}
func ensureWindowsBridge(root string) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	path := filepath.Join(root, "node_modules", "hyperframes", "dist", "chunk-VKFYI7AQ.js")
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	patched, err := bridgeEngineBytes(raw)
	if err != nil {
		return err
	}
	if bytes.Equal(raw, patched) {
		return nil
	}
	return writeFileAtomic(path, patched)
}
