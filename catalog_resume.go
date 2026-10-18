package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
)

var catalogBatchName = regexp.MustCompile(`^batch-[0-9]+(-attempt-[0-9]+)?$`)

func resumeCatalogEvidence(root string, expected CatalogEvidence, cases []AtlasCase, pixels *catalogPixelGate, stderr io.Writer) (CatalogEvidence, error) {
	var prior CatalogEvidence
	data, err := os.ReadFile(filepath.Join(root, "catalog-report.json"))
	if err != nil {
		return prior, err
	}
	if err := decodeStrict(data, &prior); err != nil {
		return prior, err
	}
	if prior.Schema != expected.Schema || prior.CaseCount != expected.CaseCount || prior.Mode != expected.Mode || prior.Limited != expected.Limited || prior.Platform != expected.Platform || prior.Backend != expected.Backend || prior.Raster != expected.Raster || prior.CatalogSHA256 != expected.CatalogSHA256 || !reflect.DeepEqual(prior.SourceSHA256, expected.SourceSHA256) {
		return prior, fmt.Errorf("resume requires identical catalog, renderer sources, platform and sampling options")
	}
	completed := 0
	for _, name := range prior.Batches {
		if !catalogBatchName.MatchString(name) {
			return prior, fmt.Errorf("invalid cached batch name")
		}
		project := filepath.Join(root, name)
		data, err := os.ReadFile(filepath.Join(project, "template.json"))
		if err != nil {
			return prior, err
		}
		var atlas AtlasData
		if err := decodeStrict(data, &atlas); err != nil {
			return prior, err
		}
		end := completed + len(atlas.Cases)
		if len(atlas.Cases) == 0 || end > len(cases) {
			return prior, fmt.Errorf("invalid cached case count")
		}
		actual, _ := json.Marshal(atlas.Cases)
		wanted, _ := json.Marshal(cases[completed:end])
		if !bytes.Equal(actual, wanted) {
			return prior, fmt.Errorf("cached case sequence changed: %s", name)
		}
		data, err = os.ReadFile(filepath.Join(project, "qa", "render-report.json"))
		if err != nil {
			return prior, err
		}
		var build BuildReport
		if err := json.Unmarshal(data, &build); err != nil {
			return prior, err
		}
		if !build.Passed || !build.VerificationFullTimeline || build.CaptureBridge != captureBridgeID {
			return prior, fmt.Errorf("cached batch was not fully verified with the current GPU barrier: %s", name)
		}
		for rel, wanted := range build.InputSHA256 {
			path, err := projectFilePath(project, rel)
			if err != nil {
				return prior, err
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return prior, err
			}
			h := sha256.Sum256(raw)
			if hex.EncodeToString(h[:]) != wanted {
				return prior, fmt.Errorf("cached input changed: %s/%s", name, rel)
			}
		}
		data, err = os.ReadFile(filepath.Join(project, "qa", "pixel-report.json"))
		if err != nil {
			return prior, err
		}
		var recorded []pixelEvidence
		if err := json.Unmarshal(data, &recorded); err != nil {
			return prior, err
		}
		if len(recorded) != len(atlas.Cases) {
			return prior, fmt.Errorf("cached pixel coverage mismatch")
		}
		for i, row := range recorded {
			if row.Case != atlas.Cases[i].ID {
				return prior, fmt.Errorf("cached pixel case mismatch")
			}
			path, err := projectFilePath(project, row.PNG)
			if err != nil {
				return prior, err
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return prior, err
			}
			h := sha256.Sum256(raw)
			if hex.EncodeToString(h[:]) != row.SHA256 {
				return prior, fmt.Errorf("cached PNG changed: %s", row.PNG)
			}
		}
		// Recheck the actual media, including complete decode; never trust a stale pass flag.
		if code := runCheck([]string{"--manifest", filepath.Join(project, "pv-project.json"), "--media", build.Output, "--decode", "--auto-install=false", "--json"}, io.Discard, stderr); code != 0 {
			return prior, fmt.Errorf("cached MP4 failed revalidation: %s", name)
		}
		if err := pixels.checkBatch(project, atlas.Cases); err != nil {
			return prior, err
		}
		completed = end
	}
	if completed != prior.CompletedCases {
		return prior, fmt.Errorf("cached aggregate count differs from completed batches")
	}
	prior.PixelChecks = pixels.frames
	prior.EndpointChecks = pixels.endpoints
	prior.LoopChecks = pixels.loops
	return prior, nil
}
