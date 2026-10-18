package main

import (
	"fmt"
	"io"
	"strings"
)

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
	ReleaseMode  bool          `json:"release_mode,omitempty"`
	Passed       bool          `json:"passed"`
	ReleaseReady bool          `json:"release_ready,omitempty"`
	Checks       []Check       `json:"checks"`
	Media        *MediaSummary `json:"media,omitempty"`
	MP4          *MP4Integrity `json:"mp4,omitempty"`
}

func (r *Report) add(code, status, message string) {
	r.Checks = append(r.Checks, Check{code, status, message})
}
func (r *Report) finish(release bool) {
	r.ReleaseMode = release
	r.Passed = true
	for _, c := range r.Checks {
		if c.Status == "fail" {
			r.Passed = false
		}
	}
	r.ReleaseReady = release && r.Passed
}
func printReport(w io.Writer, r Report) {
	state := "PASS"
	if !r.Passed {
		state = "FAIL"
	}
	fmt.Fprintf(w, "%s: %s\n", state, r.ProjectTitle)
	for _, c := range r.Checks {
		if c.Status != "pass" {
			fmt.Fprintf(w, "  %s [%s] %s\n", strings.ToUpper(c.Status), c.Code, c.Message)
		}
	}
	if r.Media != nil {
		frames := r.Media.Frames
		if r.MP4 != nil {
			frames = int64(r.MP4.DecodedFrames)
		}
		fmt.Fprintf(w, "Media: %dx%d %.3g fps, %.3g s, %s/%s, decoded frames=%d\n", r.Media.Width, r.Media.Height, r.Media.FPS, r.Media.DurationSeconds, r.Media.VideoCodec, r.Media.PixelFormat, frames)
	}
}
