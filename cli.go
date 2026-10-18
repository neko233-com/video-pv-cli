package main

import (
	"fmt"
	"io"
	"strings"
)

func realMain(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(stdout, `video-pv-cli — deterministic HTML/WebGL PV production and QA

Usage: video-pv-cli <command> [options]
  init     Create a local game/character/product PV with an original WebGL demo
  setup    Install locked Multiplane dependencies and Chromium in a project
  render   Lint, inspect, verify, sample, render MP4, and fully decode the result
  check    Validate a project manifest, captions and encoded media
  doctor   Inspect FFmpeg and the project's renderer environment
  skill    Install or pack the bundled pv-production agent skill
  version  Print the CLI version

Run a command with --help for its options. Rendering needs Node.js >=24.
Go 1.27 builds the CLI; check without media needs no external runtime.`)
		return 0
	}
	switch args[0] {
	case "version", "--version":
		fmt.Fprintln(stdout, "video-pv-cli "+version)
		return 0
	case "init":
		return runInit(args[1:], stdout, stderr)
	case "setup":
		return runSetup(args[1:], stdout, stderr)
	case "render":
		return runRender(args[1:], stdout, stderr)
	case "check":
		return runCheck(args[1:], stdout, stderr)
	case "doctor":
		return runRendererDoctor(args[1:], stdout, stderr)
	case "skill":
		return runSkill(args[1:], stdout, stderr)
	default:
		if strings.HasPrefix(args[0], "-") {
			return runCheck(args, stdout, stderr)
		}
		fmt.Fprintf(stderr, "unknown command %q; run video-pv-cli --help\n", args[0])
		return 2
	}
}
