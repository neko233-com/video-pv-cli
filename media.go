package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type probeResult struct {
	Streams []probeStream `json:"streams"`
	Format  struct {
		Duration string `json:"duration"`
		Name     string `json:"format_name"`
	} `json:"format"`
}

type probeStream struct {
	PixelFormat   string `json:"pix_fmt"`
	Frames        string `json:"nb_frames"`
	CodecType     string `json:"codec_type"`
	CodecName     string `json:"codec_name"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	FrameRate     string `json:"avg_frame_rate"`
	RealFrameRate string `json:"r_frame_rate"`
	Duration      string `json:"duration"`
	Start         string `json:"start_time"`
}

func inspectMedia(path, ffprobe string) (*MediaSummary, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, ffprobe,
		"-v", "error",
		"-show_entries", "format=duration,format_name:stream=codec_type,codec_name,width,height,avg_frame_rate,r_frame_rate,duration,start_time,pix_fmt,nb_frames",
		"-of", "json",
		"-err_detect", "explode", "-i", path,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("ffprobe timed out after 30 seconds for %s", path)
		}
		if _, ok := err.(*exec.Error); ok {
			return nil, fmt.Errorf("ffprobe executable %q could not be started; run video-pv-cli --doctor to inspect FFmpeg availability", ffprobe)
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("ffprobe could not read %s: %s", path, message)
	}
	var probe probeResult
	if err := json.Unmarshal(stdout.Bytes(), &probe); err != nil {
		return nil, fmt.Errorf("ffprobe returned invalid JSON: %w", err)
	}
	var video *probeStream
	var audioCodec string
	var streamDuration float64
	videoStreams, audioStreams := 0, 0
	var audioDuration, audioStart float64
	for i := range probe.Streams {
		stream := &probe.Streams[i]
		duration, _ := strconv.ParseFloat(stream.Duration, 64)
		if duration > streamDuration {
			streamDuration = duration
		}
		switch stream.CodecType {
		case "video":
			videoStreams++
			if video == nil {
				video = stream
			}
		case "audio":
			audioStreams++
			if audioCodec == "" {
				audioCodec = stream.CodecName
				audioDuration = duration
				audioStart, _ = strconv.ParseFloat(stream.Start, 64)
			}
		}
	}
	if video == nil {
		return nil, fmt.Errorf("ffprobe found no video stream in %s", path)
	}
	duration, err := strconv.ParseFloat(probe.Format.Duration, 64)
	if err != nil || !finitePositive(duration) {
		duration = streamDuration
	}
	if !finitePositive(duration) {
		return nil, fmt.Errorf("ffprobe did not report a valid media duration for %s", path)
	}
	fps := parseFrameRate(video.FrameRate)
	if !finitePositive(fps) {
		fps = parseFrameRate(video.RealFrameRate)
	}
	if !finitePositive(fps) {
		return nil, fmt.Errorf("ffprobe did not report a valid video frame rate for %s", path)
	}
	if video.Width <= 0 || video.Height <= 0 {
		return nil, fmt.Errorf("ffprobe did not report valid video dimensions for %s", path)
	}
	videoDuration, _ := strconv.ParseFloat(video.Duration, 64)
	if !finitePositive(videoDuration) {
		videoDuration = 0
	}
	frames, _ := strconv.ParseInt(video.Frames, 10, 64)
	return &MediaSummary{
		VideoStreams: videoStreams, AudioStreams: audioStreams, AudioDurationSeconds: audioDuration, AudioStartSeconds: audioStart,
		PixelFormat: video.PixelFormat, VideoDurationSeconds: videoDuration, Frames: frames,
		Path: path, Container: probe.Format.Name, VideoCodec: video.CodecName, AudioCodec: audioCodec,
		Width: video.Width, Height: video.Height, FPS: fps, DurationSeconds: duration,
	}, nil
}

func decodeMedia(path, ffmpeg string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, ffmpeg,
		"-hide_banner", "-v", "error", "-xerror",
		"-err_detect", "explode",
		"-i", path,
		"-map", "0:v?", "-map", "0:a?", "-sn", "-dn",
		"-f", "null", "-",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("full decode timed out after 10 minutes for %s", path)
		}
		if _, ok := err.(*exec.Error); ok {
			return fmt.Errorf("ffmpeg executable %q could not be started; run video-pv-cli --doctor to inspect FFmpeg availability", ffmpeg)
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("ffmpeg could not decode %s: %s", path, message)
	}
	return nil
}

func parseFrameRate(value string) float64 {
	parts := strings.Split(value, "/")
	if len(parts) == 2 {
		numerator, err1 := strconv.ParseFloat(parts[0], 64)
		denominator, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 != nil || err2 != nil || denominator == 0 {
			return 0
		}
		return numerator / denominator
	}
	fps, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(fps) || math.IsInf(fps, 0) {
		return 0
	}
	return fps
}

func compareMedia(report *Report, project ProjectFile, media MediaSummary) {
	var problems []string
	if expected := project.Delivery.PixelFormat; expected != "" && media.PixelFormat != expected {
		problems = append(problems, fmt.Sprintf("pixel format is %s; expected %s", media.PixelFormat, expected))
	}
	if expected := strings.ToLower(strings.TrimSpace(project.Delivery.Container)); expected != "" && !containsCSVValue(media.Container, expected) {
		problems = append(problems, fmt.Sprintf("container is %s; expected %s", displayValue(media.Container), expected))
	}
	if media.Width != project.Project.Width || media.Height != project.Project.Height {
		problems = append(problems, fmt.Sprintf("resolution is %dx%d; expected %dx%d", media.Width, media.Height, project.Project.Width, project.Project.Height))
	}
	if math.Abs(media.FPS-project.Project.FPS) > 0.1 {
		problems = append(problems, fmt.Sprintf("frame rate is %.3g fps; expected %.3g fps", media.FPS, project.Project.FPS))
	}
	tolerance := math.Max(0.25, 2/math.Max(project.Project.FPS, 1))
	if math.Abs(media.DurationSeconds-project.Project.DurationSeconds) > tolerance {
		problems = append(problems, fmt.Sprintf("duration is %.3g s; expected %.3g s (tolerance %.3g s)", media.DurationSeconds, project.Project.DurationSeconds, tolerance))
	}
	if media.VideoDurationSeconds > 0 && math.Abs(media.VideoDurationSeconds-project.Project.DurationSeconds) > tolerance {
		problems = append(problems, "video stream duration differs from target (container/audio duration cannot stand in for video)")
	}
	if media.Frames > 0 && math.Abs(float64(media.Frames)-math.Round(project.Project.DurationSeconds*project.Project.FPS)) > 2 {
		problems = append(problems, "encoded video frame count differs from target")
	}
	if expected := strings.ToLower(strings.TrimSpace(project.Delivery.VideoCodec)); expected != "" && !strings.EqualFold(media.VideoCodec, expected) {
		problems = append(problems, fmt.Sprintf("video codec is %s; expected %s", media.VideoCodec, expected))
	}
	if project.Project.AudioRequired && media.AudioCodec == "" {
		problems = append(problems, "an audio stream is required but none was found")
	}
	if expected := strings.ToLower(strings.TrimSpace(project.Delivery.AudioCodec)); expected != "" && !strings.EqualFold(media.AudioCodec, expected) {
		problems = append(problems, fmt.Sprintf("audio codec is %s; expected %s", displayValue(media.AudioCodec), expected))
	}
	if len(problems) > 0 {
		report.add("media.spec", "fail", strings.Join(problems, "; "))
	} else {
		report.add("media.spec", "pass", "final media matches the declared technical specifications")
	}
}

func containsCSVValue(values, expected string) bool {
	for _, value := range strings.Split(values, ",") {
		if strings.EqualFold(strings.TrimSpace(value), expected) {
			return true
		}
	}
	return false
}
