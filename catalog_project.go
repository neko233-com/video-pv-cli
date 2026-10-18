package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

// Data assets are validated even in manifest-only checks, before starting Chromium.
func validateTemplateFiles(root string, project ProjectFile, report *Report) {
	var templateAsset, atlasAsset, transitionAsset *Asset
	for i := range project.Assets {
		a := &project.Assets[i]
		switch a.Role {
		case "template-data":
			if templateAsset != nil {
				report.add("template.data", "fail", "multiple template-data assets")
			}
			templateAsset = a
		case "atlas-data":
			atlasAsset = a
		case "transition-data":
			transitionAsset = a
		}
	}
	if templateAsset == nil && atlasAsset == nil {
		return
	}
	if transitionAsset == nil {
		report.add("template.data", "fail", "template needs a declared transition-data asset")
		return
	}
	load := func(a *Asset, target any) error {
		path, err := projectFilePath(root, a.Path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return decodeStrict(data, target)
	}
	transitions := map[string]TransitionData{}
	err := load(transitionAsset, &transitions)
	if err == nil && len(transitions) == 0 {
		err = fmt.Errorf("empty transition catalog")
	}
	if err == nil {
		for id, tr := range transitions {
			if err = validateTransition(id, tr); err != nil {
				break
			}
		}
	}
	if err != nil {
		report.add("template.data", "fail", err.Error())
		return
	}
	checkImages := func(t TemplateData) error {
		for _, s := range t.Scenes {
			if s.Image == "" {
				continue
			}
			path, err := projectFilePath(root, s.Image)
			if err != nil {
				return err
			}
			info, err := os.Stat(path)
			if err != nil || info.IsDir() {
				return fmt.Errorf("scene %s image is missing: %s", s.ID, s.Image)
			}
			declared := false
			for _, a := range project.Assets {
				if filepath.Clean(filepath.FromSlash(a.Path)) == filepath.Clean(filepath.FromSlash(s.Image)) {
					declared = true
					break
				}
			}
			if !declared {
				return fmt.Errorf("scene %s image is not declared as a manifest asset: %s", s.ID, s.Image)
			}
		}
		return nil
	}
	if templateAsset != nil {
		var template TemplateData
		err = load(templateAsset, &template)
		if err == nil {
			err = validateTemplate(template, transitions)
		}
		if err == nil {
			err = checkImages(template)
		}
		if err == nil && (template.Width != project.Project.Width || template.Height != project.Project.Height || math.Abs(template.FPS-project.Project.FPS) > .000001 || math.Abs(templateDuration(template)-project.Project.DurationSeconds) > .000001) {
			err = fmt.Errorf("template profile/timing differs from pv-project.json; synchronize the manifest after editing data")
		}
	} else {
		var atlas AtlasData
		err = load(atlasAsset, &atlas)
		if err == nil && (atlas.Kind != "atlas" || len(atlas.Cases) == 0 || atlas.Width != project.Project.Width || atlas.Height != project.Project.Height || atlas.FPS != project.Project.FPS || math.Abs(float64(len(atlas.Cases))/atlas.FPS-project.Project.DurationSeconds) > .000001) {
			err = fmt.Errorf("invalid atlas profile/cases")
		}
		seen := map[string]bool{}
		if err == nil {
			for _, c := range atlas.Cases {
				if c.ID == "" || seen[c.ID] || !finiteNonNegative(c.Time) || c.Time > templateDuration(c.Template) || !finiteNonNegative(c.Phase) || c.Phase > 1 {
					err = fmt.Errorf("invalid atlas case %s", c.ID)
					break
				}
				seen[c.ID] = true
				if err = validateTemplate(c.Template, transitions); err != nil {
					break
				}
				if err = checkImages(c.Template); err != nil {
					break
				}
				if c.Transition != "" {
					if _, ok := transitions[c.Transition]; !ok || len(c.Template.Scenes) < 2 {
						err = fmt.Errorf("invalid atlas transition %s", c.Transition)
						break
					}
				}
			}
		}
	}
	if err != nil {
		report.add("template.data", "fail", err.Error())
	} else {
		report.add("template.data", "pass", "strict data schema, transition references, image assets and timeline profile are valid")
	}
}

// synchronizeDataManifest updates derived timeline fields without inventing approvals.
func synchronizeDataManifest(root string) error {
	path := filepath.Join(root, "pv-project.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var existing ProjectFile
	if err := decodeStrict(data, &existing); err != nil {
		return err
	}
	data, err = os.ReadFile(filepath.Join(root, "template.json"))
	if err != nil {
		return err
	}
	var template TemplateData
	if err := decodeStrict(data, &template); err != nil {
		return err
	}
	data, err = os.ReadFile(filepath.Join(root, "transitions.json"))
	if err != nil {
		return err
	}
	var transitions map[string]TransitionData
	if err := decodeStrict(data, &transitions); err != nil {
		return err
	}
	for id, tr := range transitions {
		if err := validateTransition(id, tr); err != nil {
			return err
		}
	}
	if err := validateTemplate(template, transitions); err != nil {
		return err
	}
	derived := manifestForTemplate(template)
	existing.Project.Title = template.ProjectTitle
	existing.Project.Width, existing.Project.Height, existing.Project.FPS = template.Width, template.Height, template.FPS
	existing.Project.DurationSeconds = templateDuration(template)
	existing.Project.AspectRatio = fmt.Sprintf("%d:%d", template.Width, template.Height)
	existing.Shots = derived.Shots
	existing.Review = Review{}
	known := map[string]bool{}
	for _, a := range existing.Assets {
		known[filepath.Clean(a.Path)] = true
	}
	for _, s := range template.Scenes {
		if s.Image == "" || known[filepath.Clean(s.Image)] {
			continue
		}
		existing.Assets = append(existing.Assets, Asset{ID: fmt.Sprintf("image-%d", len(existing.Assets)+1), Path: s.Image, Role: "visual-reference", Source: "User-specified local template image; record provenance", RightsStatus: "pending", Status: "draft"})
		known[filepath.Clean(s.Image)] = true
	}
	ids := []string{}
	for _, a := range existing.Assets {
		ids = append(ids, a.ID)
	}
	for i := range existing.Shots {
		existing.Shots[i].AssetIDs = ids
	}
	var report Report
	validateTemplateFiles(root, existing, &report)
	if reportHasFailures(report) {
		return fmt.Errorf("cannot synchronize invalid data: %v", report.Checks)
	}
	data, err = json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	// Keep the previous manifest for claim/shot mapping and editorial recovery.
	backup, err := os.OpenFile(path+".before-sync", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("save/remove existing pv-project.json.before-sync before synchronizing again: %w", err)
	}
	original, err := os.ReadFile(path)
	if err == nil {
		_, err = backup.Write(original)
	}
	closeErr := backup.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return writeFileAtomic(path, append(data, '\n'))
}
