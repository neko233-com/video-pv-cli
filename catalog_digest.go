package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
)

func catalogDataSHA256() (string, error) {
	hash := sha256.New()
	for _, prefix := range []string{"assets/catalog", "assets/styles"} {
		if err := fs.WalkDir(bundled, prefix, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			data, err := bundled.ReadFile(path)
			if err != nil {
				return err
			}
			hash.Write([]byte(path + "\x00"))
			hash.Write(data)
			return nil
		}); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func previewSourceDigests() map[string]string {
	digest, _ := catalogDataSHA256()
	out := map[string]string{"catalog": digest}
	for _, path := range []string{"assets/starter/pv.html", "assets/starter/catalog-runtime.js", "assets/starter/catalog.wgsl", "assets/starter/webgpu-runtime.js", "assets/starter/library.wgsl", "assets/preview/index.html"} {
		raw, _ := bundled.ReadFile(path)
		sum := sha256.Sum256(raw)
		out[path] = hex.EncodeToString(sum[:])
	}
	return out
}
