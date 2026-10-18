package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

const captureBridgeID = "multiplane-0.3.10-webgpu-barrier-v1"
const upstreamCaptureSHA256 = "c7e69d6623903a0ea6dbc238aa49bdbfb7042036bd7ac622be3e3e7fef5beb88"
const captureCallback = "page.evaluate(({ t, motionBlur, motionBlurBase, motionProbeTime, dofMotionSamples, dofMotionIndex, alpha }) => {"
const bridgedCallback = "page.evaluate(async ({ t, motionBlur, motionBlurBase, motionProbeTime, dofMotionSamples, dofMotionIndex, alpha }) => {"
const captureRender = "try { SHOT.render(t); } catch (error)"
const bridgedRender = "// video-pv-cli WebGPU queue barrier v1\n          try { SHOT.render(t); if (typeof globalThis.__PV_WEBGPU_FLUSH !== 'function') throw new Error('PV_WEBGPU_SYNC_REQUIRED'); await globalThis.__PV_WEBGPU_FLUSH(); } catch (error)"

func bridgeCaptureSource(data []byte, expectedSHA string) ([]byte, error) {
	original := data
	if bytes.Contains(data, []byte("// video-pv-cli WebGPU queue barrier v1")) {
		original = bytes.Replace(data, []byte(bridgedCallback), []byte(captureCallback), 1)
		original = bytes.Replace(original, []byte(bridgedRender), []byte(captureRender), 1)
	}
	h := sha256.Sum256(original)
	if hex.EncodeToString(h[:]) != expectedSHA {
		return nil, fmt.Errorf("unrecognized Multiplane capture source: refusing an unsafe WebGPU synchronization patch")
	}
	if bytes.Count(original, []byte(captureCallback)) != 1 || bytes.Count(original, []byte(captureRender)) != 1 {
		return nil, fmt.Errorf("Multiplane capture contract changed")
	}
	patched := bytes.Replace(original, []byte(captureCallback), []byte(bridgedCallback), 1)
	patched = bytes.Replace(patched, []byte(captureRender), []byte(bridgedRender), 1)
	return patched, nil
}

// The pinned upstream capture calls synchronous render(t), then screenshots.
// Await the native WebGPU queue before every screenshot without changing render(t).
func ensureWebGPUCaptureBridge(project string) (string, error) {
	path := filepath.Join(project, "node_modules", "multiplane", "src", "capture.mjs")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	patched, err := bridgeCaptureSource(data, upstreamCaptureSHA256)
	if err != nil {
		return "", err
	}
	if !bytes.Equal(data, patched) {
		if err := writeFileAtomic(path, patched); err != nil {
			return "", err
		}
	}
	h := sha256.Sum256(patched)
	return hex.EncodeToString(h[:]), nil
}
