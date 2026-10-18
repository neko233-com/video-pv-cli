package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type captionCue struct {
	Start float64
	End   float64
}

var captionTime = regexp.MustCompile(`^\s*((?:\d{2,}:)?\d{2}:\d{2}[,.]\d{3})\s*-->\s*((?:\d{2,}:)?\d{2}:\d{2}[,.]\d{3})(?:\s+.*)?$`)

func parseCaptions(source, extension string) ([]captionCue, error) {
	extension = strings.ToLower(extension)
	if extension != ".srt" && extension != ".vtt" {
		return nil, fmt.Errorf("caption file must use .srt or .vtt extension")
	}
	source = strings.TrimPrefix(source, "\ufeff")
	lines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	startLine := 0
	if extension == ".vtt" {
		if len(lines) == 0 || !(lines[0] == "WEBVTT" || strings.HasPrefix(lines[0], "WEBVTT ") || strings.HasPrefix(lines[0], "WEBVTT\t")) {
			return nil, fmt.Errorf("VTT requires a WEBVTT header")
		}
		startLine = 1
		for startLine < len(lines) && strings.TrimSpace(lines[startLine]) != "" {
			if strings.Contains(lines[startLine], "-->") {
				return nil, fmt.Errorf("VTT header must be followed by a blank line")
			}
			startLine++
		}
	}
	var cues []captionCue
	for i := startLine; i < len(lines); {
		if strings.TrimSpace(lines[i]) == "" {
			i++
			continue
		}
		endBlock := i
		for endBlock < len(lines) && strings.TrimSpace(lines[endBlock]) != "" {
			endBlock++
		}
		line := strings.TrimSpace(lines[i])
		row := i
		if extension == ".vtt" && (line == "NOTE" || strings.HasPrefix(line, "NOTE ") || strings.HasPrefix(line, "NOTE\t") || line == "STYLE" || line == "REGION") {
			i = endBlock
			continue
		}
		if !strings.Contains(line, "-->") {
			if extension == ".srt" {
				if _, err := strconv.Atoi(line); err != nil {
					return nil, fmt.Errorf("line %d: invalid SRT cue identifier", i+1)
				}
			}
			row++
			if row >= endBlock {
				return nil, fmt.Errorf("line %d: cue has no timing row", i+1)
			}
			line = strings.TrimSpace(lines[row])
		}
		match := captionTime.FindStringSubmatch(line)
		if len(match) != 3 {
			return nil, fmt.Errorf("line %d: invalid cue timing row", row+1)
		}
		if extension == ".vtt" && (strings.Contains(match[1], ",") || strings.Contains(match[2], ",")) {
			return nil, fmt.Errorf("line %d: VTT timestamps require decimal dots", row+1)
		}
		if extension == ".srt" && (strings.Count(match[1], ":") != 2 || strings.Count(match[2], ":") != 2 || !strings.Contains(match[1], ",") || !strings.Contains(match[2], ",")) {
			return nil, fmt.Errorf("line %d: SRT timestamps require HH:MM:SS,mmm", row+1)
		}
		start, err := parseCaptionTime(match[1])
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		end, err := parseCaptionTime(match[2])
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		if end <= start {
			return nil, fmt.Errorf("line %d: cue end must be after cue start", i+1)
		}
		textLines := 0
		for j := row + 1; j < endBlock; j++ {
			if captionTime.MatchString(strings.TrimSpace(lines[j])) {
				return nil, fmt.Errorf("line %d: missing blank separator before next cue", j+1)
			}
			textLines++
		}
		if textLines == 0 {
			return nil, fmt.Errorf("line %d: cue has no caption text", i+1)
		}
		cues = append(cues, captionCue{Start: start, End: end})
		i = endBlock
	}
	if len(cues) == 0 {
		return nil, fmt.Errorf("no caption cues found")
	}
	return cues, nil
}

func parseCaptionTime(value string) (float64, error) {
	value = strings.ReplaceAll(value, ",", ".")
	parts := strings.Split(value, ":")
	if len(parts) != 2 && len(parts) != 3 {
		return 0, fmt.Errorf("invalid timestamp %q", value)
	}
	var hours, minutes int
	secondPart := 0
	if len(parts) == 3 {
		var err error
		hours, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid timestamp %q", value)
		}
		minutes, err = strconv.Atoi(parts[1])
		if err != nil || hours < 0 || minutes < 0 || minutes >= 60 {
			return 0, fmt.Errorf("invalid timestamp %q", value)
		}
		secondPart = 2
	} else {
		var err error
		minutes, err = strconv.Atoi(parts[0])
		if err != nil || minutes < 0 {
			return 0, fmt.Errorf("invalid timestamp %q", value)
		}
		secondPart = 1
	}
	seconds, err := strconv.ParseFloat(parts[secondPart], 64)
	if err != nil || !finiteNonNegative(seconds) || seconds >= 60 {
		return 0, fmt.Errorf("invalid timestamp %q", value)
	}
	return float64(hours*3600+minutes*60) + seconds, nil
}
