package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type ProjectFile struct {
	SchemaVersion string      `json:"schema_version"`
	Project       ProjectInfo `json:"project"`
	Assets        []Asset     `json:"assets"`
	Claims        []Claim     `json:"claims"`
	Shots         []Shot      `json:"shots"`
	Delivery      Delivery    `json:"delivery"`
	Review        Review      `json:"review"`
}

type ProjectInfo struct {
	Title            string   `json:"title"`
	Type             string   `json:"type"`
	DurationSeconds  float64  `json:"duration_seconds"`
	Width            int      `json:"width"`
	Height           int      `json:"height"`
	FPS              float64  `json:"fps"`
	AspectRatio      string   `json:"aspect_ratio,omitempty"`
	Platforms        []string `json:"platforms"`
	Language         string   `json:"language"`
	AudioRequired    bool     `json:"audio_required"`
	CaptionsRequired bool     `json:"captions_required"`
}

type Asset struct {
	ID           string `json:"id"`
	Path         string `json:"path"`
	Role         string `json:"role"`
	Source       string `json:"source"`
	RightsStatus string `json:"rights_status"`
	Status       string `json:"status"`
	Attribution  string `json:"attribution,omitempty"`
}

type Claim struct {
	ID               string   `json:"id"`
	Text             string   `json:"text"`
	EvidenceAssetIDs []string `json:"evidence_asset_ids"`
}

type Shot struct {
	ID                string   `json:"id"`
	StartSeconds      float64  `json:"start_seconds"`
	EndSeconds        float64  `json:"end_seconds"`
	Beat              string   `json:"beat"`
	Role              string   `json:"role"`
	VisualDescription string   `json:"visual_description"`
	AssetIDs          []string `json:"asset_ids"`
	ClaimIDs          []string `json:"claim_ids"`
	Status            string   `json:"status"`
}

type Delivery struct {
	PixelFormat string `json:"pixel_format,omitempty"`
	Container   string `json:"container,omitempty"`
	VideoPath   string `json:"video_path"`
	VideoCodec  string `json:"video_codec,omitempty"`
	AudioCodec  string `json:"audio_codec,omitempty"`
}

type Review struct {
	CreativeApproved bool   `json:"creative_approved"`
	Reviewer         string `json:"reviewer"`
	ApprovedAt       string `json:"approved_at"`
}

type Check struct {
	Code    string `json:"code"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type MediaSummary struct {
	VideoStreams         int     `json:"video_streams,omitempty"`
	AudioStreams         int     `json:"audio_streams,omitempty"`
	AudioDurationSeconds float64 `json:"audio_duration_seconds,omitempty"`
	AudioStartSeconds    float64 `json:"audio_start_seconds,omitempty"`
	PixelFormat          string  `json:"pixel_format,omitempty"`
	VideoDurationSeconds float64 `json:"video_duration_seconds,omitempty"`
	Frames               int64   `json:"frames,omitempty"`
	Path                 string  `json:"path"`
	Container            string  `json:"container"`
	VideoCodec           string  `json:"video_codec"`
	AudioCodec           string  `json:"audio_codec,omitempty"`
	Width                int     `json:"width"`
	Height               int     `json:"height"`
	FPS                  float64 `json:"fps"`
	DurationSeconds      float64 `json:"duration_seconds"`
}

type Report struct {
	SourceSHA256 string        `json:"source_sha256,omitempty"`
	ProjectTitle string        `json:"project_title"`
	ReleaseMode  bool          `json:"release_mode"`
	Passed       bool          `json:"passed"`
	ReleaseReady bool          `json:"release_ready"`
	Checks       []Check       `json:"checks"`
	Media        *MediaSummary `json:"media,omitempty"`
	MP4          *MP4Integrity `json:"mp4,omitempty"`
}

func (r *Report) add(code, status, message string) {
	r.Checks = append(r.Checks, Check{Code: code, Status: status, Message: message})
}

func (r *Report) finish(release bool) {
	r.ReleaseMode = release
	r.Passed = true
	for _, check := range r.Checks {
		if check.Status == "fail" {
			r.Passed = false
			break
		}
	}
	r.ReleaseReady = release && r.Passed
}

func validateManifest(path string, release bool) (ProjectFile, Report) {
	project := ProjectFile{}
	report := Report{ProjectTitle: filepath.Base(filepath.Dir(path))}
	data, err := os.ReadFile(path)
	if err != nil {
		report.add("manifest.read", "fail", "could not read manifest: "+err.Error())
		return project, report
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := rejectDuplicateKeys(data); err != nil {
		report.add("manifest.json", "fail", err.Error())
		return project, report
	}
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&project); err != nil {
		report.add("manifest.json", "fail", "invalid manifest JSON: "+err.Error())
		return project, report
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		report.add("manifest.trailing_data", "fail", "manifest must contain exactly one JSON value")
		return project, report
	}
	if strings.TrimSpace(project.Project.Title) != "" {
		report.ProjectTitle = project.Project.Title
	}
	report.add("manifest.json", "pass", "manifest JSON is valid and contains no unknown fields")
	root, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		root, _ = filepath.Abs(filepath.Dir(path))
	}
	validateProjectFields(project, &report, release)
	validateAssets(root, project.Assets, &report, release)
	validateTemplateFiles(root, project, &report)
	validateClaimsAndShots(project, &report, release)
	validateCaptions(root, project, &report, release)
	validateReview(project.Review, &report, release)
	validateDeliveryPath(root, project.Delivery.VideoPath, &report)
	return project, report
}

func validateProjectFields(project ProjectFile, report *Report, release bool) {
	var problems []string
	if project.SchemaVersion != "1.0" {
		problems = append(problems, "schema_version must be 1.0")
	}
	if strings.TrimSpace(project.Project.Title) == "" {
		problems = append(problems, "project.title is required")
	}
	switch project.Project.Type {
	case "game", "character", "product":
	default:
		problems = append(problems, "project.type must be game, character, or product")
	}
	if !finitePositive(project.Project.DurationSeconds) {
		problems = append(problems, "project.duration_seconds must be a finite number greater than zero")
	}
	if project.Project.Width <= 0 || project.Project.Height <= 0 {
		problems = append(problems, "project.width and project.height must be greater than zero")
	}
	if !finitePositive(project.Project.FPS) || project.Project.FPS > 240 {
		problems = append(problems, "project.fps must be greater than zero and no more than 240")
	}
	if len(project.Project.Platforms) == 0 && release {
		problems = append(problems, "release mode requires at least one target platform")
	}
	if strings.TrimSpace(project.Project.Language) == "" && release {
		problems = append(problems, "release mode requires a language")
	}
	if project.Project.AspectRatio != "" {
		ratio, ok := parseRatio(project.Project.AspectRatio)
		if !ok {
			problems = append(problems, "project.aspect_ratio must use W:H form, for example 16:9")
		} else if project.Project.Width > 0 && project.Project.Height > 0 {
			actual := float64(project.Project.Width) / float64(project.Project.Height)
			if math.Abs(actual-ratio)/ratio > 0.02 {
				problems = append(problems, "project.aspect_ratio does not agree with width and height within 2 percent")
			}
		}
	}
	if len(problems) != 0 {
		report.add("project.spec", "fail", strings.Join(problems, "; "))
	} else {
		report.add("project.spec", "pass", "project type and output specifications are valid")
	}
}

func validateAssets(root string, assets []Asset, report *Report, release bool) {
	ids := make(map[string]Asset, len(assets))
	for i, asset := range assets {
		prefix := fmt.Sprintf("assets[%d]", i)
		if strings.TrimSpace(asset.ID) == "" {
			report.add("asset.id", "fail", prefix+" has an empty id")
			continue
		}
		if _, exists := ids[asset.ID]; exists {
			report.add("asset.duplicate_id", "fail", "duplicate asset id: "+asset.ID)
			continue
		}
		ids[asset.ID] = asset
		if strings.TrimSpace(asset.Source) == "" {
			report.add("asset.source", "fail", "asset "+asset.ID+" has no source/provenance note")
		}
		if strings.TrimSpace(asset.Role) == "" {
			report.add("asset.role", "fail", "asset "+asset.ID+" has no role")
		}
		switch asset.RightsStatus {
		case "cleared", "pending", "restricted", "unknown":
		default:
			report.add("asset.rights_value", "fail", "asset "+asset.ID+" has an invalid rights_status")
		}
		switch asset.Status {
		case "planned", "draft", "approved", "rejected":
		default:
			report.add("asset.status_value", "fail", "asset "+asset.ID+" has an invalid status")
		}
		if asset.Path == "" {
			if asset.Status == "approved" || release {
				report.add("asset.path", "fail", "approved/release asset "+asset.ID+" must have a local path")
			}
		} else if filePath, err := projectFilePath(root, asset.Path); err != nil {
			report.add("asset.path", "fail", "asset "+asset.ID+": "+err.Error())
		} else if info, err := os.Stat(filePath); err != nil {
			report.add("asset.missing", "fail", "asset "+asset.ID+" file not found: "+asset.Path)
		} else if info.IsDir() || info.Size() == 0 {
			report.add("asset.not_file", "fail", "asset "+asset.ID+" must point to a non-empty file")
		}
		if release {
			if asset.Status != "approved" {
				report.add("asset.not_approved", "fail", "asset "+asset.ID+" is not approved")
			}
			if asset.RightsStatus != "cleared" {
				report.add("asset.rights_uncleared", "fail", "asset "+asset.ID+" rights status is "+asset.RightsStatus)
			}
		}
	}
	if len(assets) == 0 {
		status := "pass"
		message := "no source assets are declared"
		if release {
			status, message = "fail", "release mode requires declared source assets"
		}
		report.add("assets.count", status, message)
	} else {
		report.add("assets.count", "pass", fmt.Sprintf("%d source assets declared", len(assets)))
	}
}

func validateClaimsAndShots(project ProjectFile, report *Report, release bool) {
	assetIDs := make(map[string]Asset, len(project.Assets))
	for _, asset := range project.Assets {
		assetIDs[asset.ID] = asset
	}
	claimIDs := make(map[string]Claim, len(project.Claims))
	for _, claim := range project.Claims {
		if strings.TrimSpace(claim.ID) == "" {
			report.add("claim.id", "fail", "a claim has an empty id")
			continue
		}
		if _, exists := claimIDs[claim.ID]; exists {
			report.add("claim.duplicate_id", "fail", "duplicate claim id: "+claim.ID)
			continue
		}
		claimIDs[claim.ID] = claim
		if strings.TrimSpace(claim.Text) == "" {
			report.add("claim.text", "fail", "claim "+claim.ID+" has no text")
		}
		if release && len(claim.EvidenceAssetIDs) == 0 {
			report.add("claim.evidence_required", "fail", "claim "+claim.ID+" has no evidence asset")
		}
		for _, id := range claim.EvidenceAssetIDs {
			if _, ok := assetIDs[id]; !ok {
				report.add("claim.evidence_missing", "fail", "claim "+claim.ID+" refers to unknown evidence asset "+id)
			}
		}
	}

	shotIDs := make(map[string]struct{}, len(project.Shots))
	usedAssets := make(map[string]bool, len(assetIDs))
	usedClaims := make(map[string]bool, len(claimIDs))
	var previousEnd float64
	timelineProblems := 0
	if len(project.Shots) == 0 {
		report.add("timeline.empty", "fail", "at least one shot is required")
	}
	for i, shot := range project.Shots {
		if strings.TrimSpace(shot.ID) == "" {
			report.add("shot.id", "fail", fmt.Sprintf("shots[%d] has an empty id", i))
		} else if _, exists := shotIDs[shot.ID]; exists {
			report.add("shot.duplicate_id", "fail", "duplicate shot id: "+shot.ID)
		} else {
			shotIDs[shot.ID] = struct{}{}
		}
		if !finiteNonNegative(shot.StartSeconds) || !finitePositive(shot.EndSeconds) || shot.EndSeconds <= shot.StartSeconds {
			report.add("shot.time", "fail", "shot "+shot.ID+" must have finite, positive start/end times with end after start")
			timelineProblems++
		} else {
			if i == 0 && shot.StartSeconds > 0.05 {
				status := "warning"
				if release {
					status = "fail"
				}
				report.add("timeline.leading_gap", status, "timeline starts after 0 seconds")
				timelineProblems++
			}
			if i > 0 {
				if shot.StartSeconds < previousEnd-0.001 {
					report.add("timeline.overlap", "fail", "shot "+shot.ID+" overlaps or is out of order")
					timelineProblems++
				} else if shot.StartSeconds > previousEnd+0.05 {
					status := "warning"
					if release {
						status = "fail"
					}
					report.add("timeline.gap", status, fmt.Sprintf("gap of %.3f seconds before shot %s", shot.StartSeconds-previousEnd, shot.ID))
					timelineProblems++
				}
			}
			if shot.EndSeconds > project.Project.DurationSeconds+0.05 && finitePositive(project.Project.DurationSeconds) {
				report.add("timeline.overshoot", "fail", "shot "+shot.ID+" ends after the target runtime")
				timelineProblems++
			}
			previousEnd = shot.EndSeconds
		}
		if strings.TrimSpace(shot.Beat) == "" || strings.TrimSpace(shot.VisualDescription) == "" {
			report.add("shot.direction", "fail", "shot "+shot.ID+" needs a beat and visual_description")
		}
		switch shot.Status {
		case "planned", "draft", "approved", "needs_revision":
		default:
			report.add("shot.status_value", "fail", "shot "+shot.ID+" has an invalid status")
		}
		if release && shot.Status != "approved" {
			report.add("shot.not_approved", "fail", "shot "+shot.ID+" is not approved")
		}
		if release && len(shot.AssetIDs) == 0 {
			report.add("shot.asset_required", "fail", "release shot "+shot.ID+" has no source asset")
		}
		for _, id := range shot.AssetIDs {
			if _, ok := assetIDs[id]; !ok {
				report.add("shot.asset_missing", "fail", "shot "+shot.ID+" refers to unknown asset "+id)
			} else {
				usedAssets[id] = true
			}
		}
		for _, id := range shot.ClaimIDs {
			claim, ok := claimIDs[id]
			if !ok {
				report.add("shot.claim_missing", "fail", "shot "+shot.ID+" refers to unknown claim "+id)
				continue
			}
			usedClaims[id] = true
			if len(claim.EvidenceAssetIDs) > 0 && !intersects(shot.AssetIDs, claim.EvidenceAssetIDs) {
				report.add("claim.shot_evidence_mismatch", "fail", "shot "+shot.ID+" communicates claim "+id+" but uses none of its evidence assets")
			}
		}
	}
	if len(project.Shots) > 0 && finitePositive(project.Project.DurationSeconds) {
		last := project.Shots[len(project.Shots)-1].EndSeconds
		tolerance := math.Max(0.25, 1.0/math.Max(project.Project.FPS, 1))
		if math.Abs(last-project.Project.DurationSeconds) > tolerance {
			status := "warning"
			if release {
				status = "fail"
			}
			report.add("timeline.runtime_mismatch", status, fmt.Sprintf("last shot ends at %.3f seconds; target runtime is %.3f seconds", last, project.Project.DurationSeconds))
			timelineProblems++
		}
	}
	for _, claim := range project.Claims {
		if !usedClaims[claim.ID] {
			status := "warning"
			if release {
				status = "fail"
			}
			report.add("claim.unused", status, "claim is not assigned to any shot: "+claim.ID)
		}
	}
	for id := range assetIDs {
		if !usedAssets[id] {
			report.add("asset.unused", "warning", "asset is not assigned to a shot: "+id)
		}
	}
	if timelineProblems == 0 && len(project.Shots) > 0 {
		report.add("timeline.coverage", "pass", "shot order and timeline coverage are valid")
	}
}

func validateReview(review Review, report *Report, release bool) {
	if !release {
		return
	}
	var problems []string
	if !review.CreativeApproved {
		problems = append(problems, "creative_approved must be true")
	}
	if strings.TrimSpace(review.Reviewer) == "" {
		problems = append(problems, "reviewer is required")
	}
	if !isReleaseTimestamp(review.ApprovedAt) {
		problems = append(problems, "approved_at must be an RFC 3339 timestamp")
	}
	if len(problems) > 0 {
		report.add("review.attestation", "fail", strings.Join(problems, "; "))
	} else {
		report.add("review.attestation", "pass", "human review attestation is recorded")
	}
}

func validateDeliveryPath(root, path string, report *Report) {
	if path == "" {
		report.add("delivery.path", "warning", "delivery.video_path is empty")
		return
	}
	if _, err := projectOutputPath(root, path); err != nil {
		report.add("delivery.path", "fail", err.Error())
		return
	}
	report.add("delivery.path", "pass", "delivery path is contained in the project directory")
}

func validateCaptions(root string, project ProjectFile, report *Report, release bool) {
	captionCount := 0
	for _, asset := range project.Assets {
		if asset.Role != "caption" || asset.Path == "" {
			continue
		}
		captionCount++
		path, err := projectFilePath(root, asset.Path)
		if err != nil {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		cues, err := parseCaptions(string(data), filepath.Ext(path))
		if err != nil {
			report.add("caption.syntax", "fail", "caption asset "+asset.ID+": "+err.Error())
			continue
		}
		for i, cue := range cues {
			if cue.End <= cue.Start {
				report.add("caption.duration", "fail", fmt.Sprintf("caption asset %s cue %d has non-positive duration", asset.ID, i+1))
			}
			if cue.Start < 0 || cue.End > project.Project.DurationSeconds+0.05 {
				report.add("caption.bounds", "fail", fmt.Sprintf("caption asset %s cue %d falls outside the target runtime", asset.ID, i+1))
			}
			if i > 0 && cue.Start < cues[i-1].End {
				report.add("caption.overlap", "fail", fmt.Sprintf("caption asset %s cue %d overlaps the previous cue", asset.ID, i+1))
			}
		}
		report.add("caption.syntax", "pass", fmt.Sprintf("caption asset %s has %d valid cues", asset.ID, len(cues)))
	}
	if project.Project.CaptionsRequired && captionCount == 0 {
		status := "warning"
		if release {
			status = "fail"
		}
		report.add("caption.required", status, "captions are required but no local caption asset is declared")
	} else if captionCount == 0 {
		report.add("caption.absent", "warning", "no caption asset was found; review accessibility needs")
	}
}

var ratioPattern = regexp.MustCompile(`^\s*(\d+)\s*:\s*(\d+)\s*$`)

func parseRatio(value string) (float64, bool) {
	parts := ratioPattern.FindStringSubmatch(value)
	if len(parts) != 3 {
		return 0, false
	}
	var width, height float64
	if _, err := fmt.Sscanf(parts[1], "%f", &width); err != nil || !finitePositive(width) {
		return 0, false
	}
	if _, err := fmt.Sscanf(parts[2], "%f", &height); err != nil || !finitePositive(height) {
		return 0, false
	}
	return width / height, true
}

func projectFilePath(root, path string) (string, error) {
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("asset path must be relative to the project directory: %s", path)
	}
	return confinedPath(root, path)
}

func projectOutputPath(root, path string) (string, error) {
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("delivery.video_path must be relative to the project directory")
	}
	return confinedPath(root, path)
}

func confinedPath(root, path string) (string, error) {
	cleaned := filepath.Clean(filepath.FromSlash(path))
	if cleaned == "." || cleaned == "" {
		return "", fmt.Errorf("path must name a file")
	}
	candidate := filepath.Join(root, cleaned)
	rel, err := filepath.Rel(root, candidate)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes the project directory: %s", path)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		resolvedRoot = root
	}
	for existing := candidate; ; existing = filepath.Dir(existing) {
		resolvedPath, resolveErr := filepath.EvalSymlinks(existing)
		if resolveErr == nil {
			resolvedRel, relErr := filepath.Rel(resolvedRoot, resolvedPath)
			if relErr != nil || resolvedRel == ".." || strings.HasPrefix(resolvedRel, ".."+string(filepath.Separator)) {
				return "", fmt.Errorf("path resolves outside the project directory: %s", path)
			}
			break
		}
		if !os.IsNotExist(resolveErr) {
			return "", fmt.Errorf("could not resolve project path %s: %w", path, resolveErr)
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			break
		}
	}
	return candidate, nil
}

func finitePositive(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}
func finiteNonNegative(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func intersects(left, right []string) bool {
	values := make(map[string]struct{}, len(left))
	for _, value := range left {
		values[value] = struct{}{}
	}
	for _, value := range right {
		if _, ok := values[value]; ok {
			return true
		}
	}
	return false
}
