package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// CLI evidence is explicitly exported by our wrapper. Native revision recording
// makes UUID artifact/history folders on every snapshot/check; opt out locally.
func applyGuardedBridge(path, originalHash string, before, after []byte) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if bytes.Count(raw, after) == 1 && hashBytes(bytes.Replace(raw, after, before, 1)) == originalHash {
		return nil
	}
	if hashBytes(raw) != originalHash || bytes.Count(raw, before) != 1 {
		return fmt.Errorf("refusing changed HyperFrames bridge input: %s", filepath.Base(path))
	}
	return writeFileAtomic(path, bytes.Replace(raw, before, after, 1))
}
func ensureArtifactBridge(root string) error {
	path := filepath.Join(root, "node_modules", "hyperframes", "dist", "chunk-KQCWIE76.js")
	if err := applyGuardedBridge(path, "5386ae18017dc92373a98f4e19bd4173cd977041fdb7f57f6e9f0d888b56a8b9", []byte("async function captureArtifactRevision(projectDir) {"), []byte("async function captureArtifactRevision(projectDir) {\n  if (process.env.WEB2VIDEO_EPHEMERAL_ARTIFACTS === \"1\") return void 0; // web2video: explicit owned evidence")); err != nil {
		return err
	}
	path = filepath.Join(root, "node_modules", "hyperframes", "dist", "chunk-GMJ3ILLW.js")
	return applyGuardedBridge(path, "ee5175ba9b3b3215dbdf473d47c5bc34f6802db3681273a0e32ec9b2142f15e1", []byte("async function forgetRecordedArtifacts(projectDir, paths) {"), []byte("async function forgetRecordedArtifacts(projectDir, paths) {\n  if (process.env.WEB2VIDEO_EPHEMERAL_ARTIFACTS === \"1\") return; // web2video: no revision artifacts"))
}
