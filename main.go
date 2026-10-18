package main

import (
	"fmt"
	"io"
	"os"
)

const version = "1.0.0"

func main() { os.Exit(realMain(os.Args[1:], os.Stdout, os.Stderr)) }

func realMain(args []string, out, errOut io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(out, `web2video — HTML to verified MP4, powered by HyperFrames

Usage: web2video <command> [options]
  setup       Prepare the pinned HyperFrames/GSAP runtime in the user cache
  init        Create a source-editable project: --dir <new-dir> --template <name>
  build       Compile project.json into native HyperFrames HTML
  preview     Serve a project over HTTP (never open index.html with file://)
  lint        Run HyperFrames composition lint
  check       Run HyperFrames runtime, layout, motion and contrast gates
  render      Render H.264 MP4 and verify every decoded frame; reuse matching jobs
  verify      Verify MP4 structure, codecs, timestamps, cadence and full decode
  snapshot    Capture frames into a fixed owned output directory
  timeline    Inspect native HyperFrames tracks and clips
  styles      List/show 100 independent style data files
  templates   List the authored project templates
  catalog     Search the actual HyperFrames component registry
  add         Install a native HyperFrames registry component
  doctor      Check the local runtime, browser and FFmpeg
  qa          Run Windows/Go/media verification from the source checkout
  version     Print the version

Go 1.27 CLI. Node.js >=24 runs HyperFrames 0.8.171.
Use --project <directory> for project commands. No skill installation or GUI app.`)
		return 0
	}
	switch args[0] {
	case "version", "--version":
		fmt.Fprintln(out, "web2video "+version)
		return 0
	case "setup":
		return runSetup(args[1:], out, errOut)
	case "init":
		return runInit(args[1:], out, errOut)
	case "build":
		return runBuild(args[1:], out, errOut)
	case "render":
		return runRender(args[1:], out, errOut)
	case "verify":
		return runMP4(append([]string{"verify"}, args[1:]...), out, errOut)
	case "mp4":
		return runMP4(args[1:], out, errOut)
	case "styles", "templates":
		return runLibrary(args[0], args[1:], out, errOut)
	case "snapshot":
		return runSnapshot(args[1:], out, errOut)
	case "preview":
		return runPreview(args[1:], out, errOut)
	case "qa":
		return runQA(args[1:], out, errOut)
	case "lint", "check", "timeline", "catalog", "add", "doctor", "keyframes", "info", "capture", "compositions", "audio", "beats":
		return runNative(args[0], args[1:], out, errOut)
	default:
		fmt.Fprintf(errOut, "unknown command %q; run web2video --help\n", args[0])
		return 2
	}
}
