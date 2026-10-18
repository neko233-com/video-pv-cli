package main

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func atom(kind string, payload []byte) []byte {
	v := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(v, uint32(len(v)))
	copy(v[4:8], kind)
	copy(v[8:], payload)
	return v
}
func TestMP4AtomBoundsAndBrands(t *testing.T) {
	ftyp := atom("ftyp", []byte("isom\x00\x00\x00\x00isom"))
	moov := atom("moov", nil)
	mdat := atom("mdat", []byte{1, 2, 3})
	movie := func(parts ...[]byte) []byte { return bytes.Join(parts, nil) }
	extended := make([]byte, 19)
	binary.BigEndian.PutUint32(extended, 1)
	copy(extended[4:], "mdat")
	binary.BigEndian.PutUint64(extended[8:], 19)
	copy(extended[16:], []byte{1, 2, 3})
	zero := append([]byte(nil), mdat...)
	binary.BigEndian.PutUint32(zero, 0)
	overflow := append([]byte(nil), extended...)
	binary.BigEndian.PutUint64(overflow[8:], math.MaxUint64)
	for _, tc := range []struct {
		name        string
		data        []byte
		valid, fast bool
	}{
		{"fast", movie(ftyp, moov, mdat), true, true}, {"late", movie(ftyp, mdat, moov), true, false}, {"extended", movie(ftyp, moov, extended), true, true}, {"zero", movie(ftyp, moov, zero), true, true},
		{"empty", nil, false, false}, {"missing", movie(ftyp, moov), false, false}, {"duplicate", movie(ftyp, moov, moov, mdat), false, false}, {"duplicate-type", movie(ftyp, ftyp, moov, mdat), false, false},
		{"empty-data", movie(ftyp, moov, atom("mdat", nil)), false, false}, {"truncated", movie(ftyp, moov, mdat[:9]), false, false}, {"trailing", append(movie(ftyp, moov, mdat), 1), false, false},
		{"quicktime", movie(atom("ftyp", []byte("qt  \x00\x00\x00\x00isom")), moov, mdat), false, false}, {"overflow", movie(ftyp, moov, overflow), false, false}, {"short-extended", movie(ftyp, moov, extended[:12]), false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "test.mp4")
			_ = os.WriteFile(path, tc.data, 0600)
			got, err := inspectMP4Atoms(path)
			if (err == nil) != tc.valid {
				t.Fatal(err)
			}
			if tc.valid && got.FastStart != tc.fast {
				t.Fatal(got)
			}
		})
	}
}

func TestMP4FrameCadenceAndCounts(t *testing.T) {
	media := MediaSummary{FPS: 10, Frames: 3, VideoDurationSeconds: .3}
	valid := []mp4Frame{{"0", 1}, {".1", 0}, {".2", 0}}
	if err := validateMP4Frames(valid, media, &MP4Integrity{}); err != nil {
		t.Fatal(err)
	}
	for _, frames := range [][]mp4Frame{nil, {{"0", 0}}, {{"NaN", 1}}, {{"+Inf", 1}}, {{"N/A", 1}}, {{"1", 1}}, {{"0", 1}, {"0", 0}}, {{"0", 1}, {"-.1", 0}}, {{"0", 1}, {".2", 0}}} {
		if validateMP4Frames(frames, media, &MP4Integrity{}) == nil {
			t.Fatal("bad cadence accepted", frames)
		}
	}
	for _, m := range []MediaSummary{{FPS: 10, Frames: 4}, {FPS: 10, VideoDurationSeconds: 9}, {FPS: 0}, {FPS: math.NaN()}} {
		if validateMP4Frames(valid, m, &MP4Integrity{}) == nil {
			t.Fatal("bad metadata accepted", m)
		}
	}
}

func TestIntegrationPortableMP4Verification(t *testing.T) {
	if os.Getenv("PV_MEDIA_INTEGRATION") != "1" {
		t.Skip("set PV_MEDIA_INTEGRATION=1 for actual portable MP4 negative fixtures")
	}
	tools, err := resolveMediaTools("ffprobe", "ffmpeg", true, false, "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	var out, stderr bytes.Buffer
	for _, tc := range []struct {
		name, codec, pixel, audio string
		fast, valid               bool
	}{
		{"silent", "libx264", "yuv420p", "", true, true}, {"aac", "libx264", "yuv420p", "aac", true, true}, {"late", "libx264", "yuv420p", "", false, true},
		{"mpeg4", "mpeg4", "yuv420p", "", true, false}, {"444", "libx264", "yuv444p", "", true, false}, {"mp3", "libx264", "yuv420p", "libmp3lame", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(root, tc.name+".mp4")
			args := []string{"-nostdin", "-v", "error", "-n", "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=10:duration=1"}
			if tc.audio != "" {
				args = append(args, "-f", "lavfi", "-i", "sine=frequency=440:duration=1")
			}
			args = append(args, "-c:v", tc.codec, "-pix_fmt", tc.pixel)
			if tc.audio != "" {
				args = append(args, "-c:a", tc.audio, "-shortest")
			}
			if tc.fast {
				args = append(args, "-movflags", "+faststart")
			}
			args = append(args, path)
			if output, err := exec.Command(tools.FFmpeg, args...).CombinedOutput(); err != nil {
				t.Fatal(err, string(output))
			}
			report := filepath.Join(root, tc.name+"-report.json")
			cli := []string{"mp4", "verify", "--input", path, "--report", report, "--auto-install=false"}
			out.Reset()
			stderr.Reset()
			code := realMain(cli, &out, &stderr)
			if (code == 0) != tc.valid {
				t.Fatal(code, out.String(), stderr.String())
			}
			if !tc.valid {
				return
			}
			before := snapshotFiles(t, root)
			if realMain(cli, &out, &stderr) != 0 || !reflect.DeepEqual(before, snapshotFiles(t, root)) {
				t.Fatal("MP4 verification not idempotent")
			}
			if realMain(append(cli, "--fast-start"), &out, &stderr) == 0 && !tc.fast {
				t.Fatal("late moov accepted for streaming")
			}
			if tc.audio != "" && realMain(append(cli, "--silent"), &out, &stderr) == 0 {
				t.Fatal("audio accepted as silent")
			}
			if tc.audio == "" && realMain(append(cli, "--require-audio"), &out, &stderr) == 0 {
				t.Fatal("missing AAC accepted")
			}
			hash, _ := fileSHA256(path)
			if realMain([]string{"mp4", "verify", "--input", path, "--report", path}, &out, &stderr) != 2 {
				t.Fatal("input overwrite allowed")
			}
			after, _ := fileSHA256(path)
			if hash != after {
				t.Fatal("source changed")
			}
			raw, _ := os.ReadFile(path)
			broken := filepath.Join(root, tc.name+"-truncated.mp4")
			_ = os.WriteFile(broken, raw[:len(raw)/2], 0600)
			if realMain([]string{"mp4", "verify", "--input", broken, "--auto-install=false"}, &out, &stderr) == 0 {
				t.Fatal("truncation accepted")
			}
			if tc.name == "silent" {
				corrupt := append([]byte(nil), raw...)
				found := false
				for offset := 0; offset+8 <= len(corrupt); {
					size := int(binary.BigEndian.Uint32(corrupt[offset:]))
					if size < 8 || offset+size > len(corrupt) {
						t.Fatal("unexpected fixture box")
					}
					if string(corrupt[offset+4:offset+8]) == "mdat" {
						for i := offset + 8; i < offset+size; i++ {
							corrupt[i] = 0
						}
						found = true
						break
					}
					offset += size
				}
				if !found {
					t.Fatal("fixture lacks media payload")
				}
				damaged := filepath.Join(root, "damaged-payload.mp4")
				_ = os.WriteFile(damaged, corrupt, 0600)
				if _, err := inspectMP4Atoms(damaged); err != nil {
					t.Fatal("payload fixture should retain valid container bounds", err)
				}
				if realMain([]string{"mp4", "verify", "--input", damaged, "--auto-install=false"}, &out, &stderr) == 0 {
					t.Fatal("corrupt H.264 payload accepted")
				}
			}
		})
	}
}

func TestIntegrationMP4RejectsActualVariableCadence(t *testing.T) {
	if os.Getenv("PV_MEDIA_INTEGRATION") != "1" {
		t.Skip("set PV_MEDIA_INTEGRATION=1 for VFR fixture")
	}
	tools, err := resolveMediaTools("ffprobe", "ffmpeg", true, false, "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "vfr.mp4")
	if output, err := exec.Command(tools.FFmpeg, "-nostdin", "-v", "error", "-n", "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=10:duration=1", "-vf", "select=not(eq(n\\,3))", "-fps_mode", "vfr", "-c:v", "libx264", "-pix_fmt", "yuv420p", path).CombinedOutput(); err != nil {
		t.Fatal(err, string(output))
	}
	var out bytes.Buffer
	if realMain([]string{"mp4", "verify", "--input", path, "--auto-install=false"}, &out, &out) == 0 {
		t.Fatal("VFR accepted by CFR delivery contract")
	}
}

func TestIntegrationMP4RejectsAmbiguousStreamsAndAudioDrift(t *testing.T) {
	if os.Getenv("PV_MEDIA_INTEGRATION") != "1" {
		t.Skip("set PV_MEDIA_INTEGRATION=1 for stream/sync fixtures")
	}
	tools, err := resolveMediaTools("ffprobe", "ffmpeg", true, false, "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"two-video", "two-audio", "long-audio", "late-audio", "audio-only"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "fixture.mp4")
			args := []string{"-nostdin", "-v", "error", "-n", "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=10:duration=1"}
			if name == "late-audio" {
				args = append(args, "-itsoffset", "0.5")
			}
			duration := "1"
			if name == "long-audio" {
				duration = "2"
			}
			args = append(args, "-f", "lavfi", "-i", "sine=duration="+duration)
			if name != "audio-only" {
				args = append(args, "-map", "0:v:0")
			}
			if name == "two-video" {
				args = append(args, "-map", "0:v:0")
			}
			args = append(args, "-map", "1:a:0")
			if name == "two-audio" {
				args = append(args, "-map", "1:a:0")
			}
			args = append(args, "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", "-movflags", "+faststart", path)
			if output, err := exec.Command(tools.FFmpeg, args...).CombinedOutput(); err != nil {
				t.Fatal(err, string(output))
			}
			var out bytes.Buffer
			if realMain([]string{"mp4", "verify", "--input", path, "--auto-install=false"}, &out, &out) == 0 {
				t.Fatal("invalid portable delivery accepted", name)
			}
		})
	}
}
