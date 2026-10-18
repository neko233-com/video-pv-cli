package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

type MP4Integrity struct {
	Brand           string  `json:"brand"`
	FastStart       bool    `json:"fast_start"`
	AtomCount       int     `json:"atom_count"`
	DecodedFrames   int     `json:"decoded_frames"`
	FirstPTS        float64 `json:"first_pts"`
	LastPTS         float64 `json:"last_pts"`
	MaxCadenceError float64 `json:"max_cadence_error_seconds"`
}

// Parse bounded top-level ISO BMFF boxes without loading a movie into memory.
func inspectMP4Atoms(path string) (MP4Integrity, error) {
	var result MP4Integrity
	f, err := os.Open(path)
	if err != nil {
		return result, err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return result, err
	}
	total := stat.Size()
	seen := map[string]int{}
	var moov, mdat int64 = -1, -1
	for offset := int64(0); offset < total; {
		if total-offset < 8 {
			return result, fmt.Errorf("truncated MP4 box header at %d", offset)
		}
		var h [16]byte
		if _, err := f.ReadAt(h[:8], offset); err != nil {
			return result, err
		}
		size := int64(binary.BigEndian.Uint32(h[:4]))
		header := int64(8)
		kind := string(h[4:8])
		if size == 1 {
			if total-offset < 16 {
				return result, fmt.Errorf("truncated extended MP4 box")
			}
			if _, err := f.ReadAt(h[8:16], offset+8); err != nil {
				return result, err
			}
			u := binary.BigEndian.Uint64(h[8:16])
			if u > math.MaxInt64 {
				return result, fmt.Errorf("MP4 box size overflow")
			}
			size = int64(u)
			header = 16
		} else if size == 0 {
			size = total - offset
		}
		if size < header || size > total-offset {
			return result, fmt.Errorf("invalid/truncated MP4 %s box", kind)
		}
		result.AtomCount++
		if result.AtomCount > 100000 {
			return result, fmt.Errorf("too many MP4 boxes")
		}
		seen[kind]++
		switch kind {
		case "ftyp":
			if seen[kind] > 1 || size < header+8 || size > 1<<20 {
				return result, fmt.Errorf("invalid MP4 file type box")
			}
			raw := make([]byte, size-header)
			if _, err := f.ReadAt(raw, offset+header); err != nil {
				return result, err
			}
			result.Brand = string(raw[:4])
			compatible := false
			for i := 0; i+4 <= len(raw); i += 4 {
				if i == 4 {
					continue
				}
				if contains([]string{"isom", "iso2", "iso3", "iso4", "iso5", "iso6", "mp41", "mp42", "avc1", "M4V ", "MSNV", "dash"}, string(raw[i:i+4])) {
					compatible = true
				}
			}
			if !compatible || result.Brand == "qt  " {
				return result, fmt.Errorf("file is not an MP4-compatible brand")
			}
		case "moov":
			if seen[kind] > 1 {
				return result, fmt.Errorf("duplicate MP4 moov box")
			}
			moov = offset
		case "mdat":
			if mdat < 0 {
				mdat = offset
			}
			if size == header {
				return result, fmt.Errorf("empty MP4 media data")
			}
		}
		offset += size
	}
	if seen["ftyp"] != 1 || moov < 0 || mdat < 0 {
		return result, fmt.Errorf("MP4 needs ftyp, moov and mdat boxes")
	}
	result.FastStart = moov < mdat
	return result, nil
}

type mp4Frame struct {
	PTS string `json:"best_effort_timestamp_time"`
	Key int    `json:"key_frame"`
}

func validateMP4Frames(frames []mp4Frame, media MediaSummary, result *MP4Integrity) error {
	if len(frames) == 0 {
		return fmt.Errorf("MP4 has no decoded video frames")
	}
	if frames[0].Key != 1 {
		return fmt.Errorf("first video frame is not independently decodable")
	}
	interval := 1 / media.FPS
	if !finitePositive(interval) {
		return fmt.Errorf("invalid MP4 cadence")
	}
	previous := 0.0
	for i, frame := range frames {
		pts, err := strconv.ParseFloat(frame.PTS, 64)
		if err != nil || math.IsNaN(pts) || math.IsInf(pts, 0) {
			return fmt.Errorf("invalid frame timestamp %d", i)
		}
		if i == 0 {
			result.FirstPTS = pts
			if math.Abs(pts) > interval+.001 {
				return fmt.Errorf("video does not start near zero")
			}
		} else {
			if pts <= previous {
				return fmt.Errorf("non-monotonic frame timestamp %d", i)
			}
			delta := math.Abs((pts - previous) - interval)
			if delta > result.MaxCadenceError {
				result.MaxCadenceError = delta
			}
			if delta > math.Max(.001, interval*.025) {
				return fmt.Errorf("variable/broken video cadence at frame %d", i)
			}
		}
		previous = pts
	}
	result.DecodedFrames = len(frames)
	result.LastPTS = previous
	if media.Frames > 0 && media.Frames != int64(len(frames)) {
		return fmt.Errorf("declared and decoded frame counts disagree")
	}
	if media.VideoDurationSeconds > 0 && math.Abs(float64(len(frames))/media.FPS-media.VideoDurationSeconds) > math.Max(.01, interval*1.1) {
		return fmt.Errorf("video duration and decoded frames disagree")
	}
	return nil
}

func inspectMP4Integrity(path, ffprobe string, media MediaSummary) (*MP4Integrity, error) {
	result, err := inspectMP4Atoms(path)
	if err != nil {
		return nil, err
	}
	if media.VideoCodec != "h264" || media.PixelFormat != "yuv420p" || media.Width%2 != 0 || media.Height%2 != 0 {
		return nil, fmt.Errorf("portable MP4 requires H.264, 8-bit yuv420p and even dimensions")
	}
	if media.AudioCodec != "" && media.AudioCodec != "aac" {
		return nil, fmt.Errorf("portable MP4 audio must be AAC or absent")
	}
	if media.VideoStreams > 1 || media.AudioStreams > 1 {
		return nil, fmt.Errorf("portable MP4 requires one video stream and at most one audio stream")
	}
	if media.AudioCodec != "" && (math.Abs(media.AudioStartSeconds) > .12 || (media.AudioDurationSeconds > 0 && media.VideoDurationSeconds > 0 && math.Abs(media.AudioDurationSeconds-media.VideoDurationSeconds) > .25)) {
		return nil, fmt.Errorf("audio/video start or duration differs beyond the portable delivery tolerance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, ffprobe, "-v", "error", "-select_streams", "v:0", "-show_frames", "-show_entries", "frame=best_effort_timestamp_time,key_frame", "-of", "json", "-i", path)
	configureProcess(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("MP4 frame inspection: %v %s", err, stderr.String())
	}
	var data struct {
		Frames []mp4Frame `json:"frames"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &data); err != nil {
		return nil, err
	}
	if err := validateMP4Frames(data.Frames, media, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func runMP4(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "verify" {
		fmt.Fprintln(stderr, "Usage: video-pv-cli mp4 verify --input video.mp4 [--report report.json] [--silent] [--require-audio] [--fast-start]")
		return 2
	}
	f := flag.NewFlagSet("mp4 verify", flag.ContinueOnError)
	f.SetOutput(stderr)
	input := f.String("input", "", "MP4 to verify; no project/engine required")
	reportPath := f.String("report", "", "optional stable JSON report")
	silent := f.Bool("silent", false, "require no audio stream")
	audio := f.Bool("require-audio", false, "require an AAC audio stream")
	fast := f.Bool("fast-start", false, "require moov before mdat")
	jsonOut := f.Bool("json", false, "JSON stdout")
	auto := f.Bool("auto-install", true, "allow current-user FFmpeg setup")
	if code := parseFlags(f, args[1:]); code >= 0 {
		return code
	}
	if *input == "" || (*silent && *audio) {
		fmt.Fprintln(stderr, "input required; silent and require-audio are mutually exclusive")
		return 2
	}
	path, err := filepath.Abs(*input)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if *reportPath != "" {
		if err := protectOutput(*reportPath, path); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
	}
	report := Report{ProjectTitle: filepath.Base(path)}
	report.SourceSHA256, err = fileSHA256(path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	tools, err := resolveMediaTools("ffprobe", "ffmpeg", true, *auto, "", false, false)
	if err == nil {
		report.Media, err = inspectMedia(path, tools.FFprobe)
	}
	if err == nil {
		report.MP4, err = inspectMP4Integrity(path, tools.FFprobe, *report.Media)
	}
	if err == nil {
		err = decodeMedia(path, tools.FFmpeg)
	}
	if err == nil {
		after, e := fileSHA256(path)
		if e != nil {
			err = e
		} else if after != report.SourceSHA256 {
			err = fmt.Errorf("MP4 changed during verification")
		}
	}
	if err != nil {
		report.add("mp4.integrity", "fail", err.Error())
	} else {
		report.add("mp4.integrity", "pass", "MP4 structure, H.264/yuv420p, decoded frame count, monotonic timestamps, constant cadence and full decode passed")
		if *silent && report.Media.AudioCodec != "" {
			report.add("mp4.audio", "fail", "silent MP4 contains audio")
		}
		if *audio && report.Media.AudioCodec != "aac" {
			report.add("mp4.audio", "fail", "required AAC audio is absent")
		}
		if *fast && !report.MP4.FastStart {
			report.add("mp4.fast_start", "fail", "moov follows media data")
		}
	}
	report.finish(false)
	if *reportPath != "" {
		raw, _ := json.MarshalIndent(report, "", "  ")
		if err := writeFileAtomic(*reportPath, append(raw, '\n')); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
	}
	if *jsonOut {
		_ = json.NewEncoder(stdout).Encode(report)
	} else {
		printReport(stdout, report)
	}
	if !report.Passed {
		return 1
	}
	return 0
}
