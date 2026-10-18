package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func runPreview(args []string, out, errOut io.Writer) int {
	f := flag.NewFlagSet("preview", flag.ContinueOnError)
	f.SetOutput(errOut)
	dir := f.String("project", ".", "native HyperFrames project directory")
	port := f.Int("port", 7340, "loopback HTTP port; 0 chooses a free port")
	if code := parseFlags(f, args); code >= 0 {
		return code
	}
	root, err := filepath.Abs(*dir)
	if err != nil {
		return commandError(errOut, err)
	}
	if err = validateComposition(root); err != nil {
		return commandError(errOut, err)
	}
	e, err := preparedEngine()
	if err != nil {
		return commandError(errOut, err)
	}
	runtimeJS := filepath.Join(e.Root, "node_modules", "hyperframes", "dist", "hyperframe-runtime.js")
	handler, err := previewHandler(root, runtimeJS)
	if err != nil {
		return commandError(errOut, err)
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		return commandError(errOut, err)
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	fmt.Fprintf(out, "web2video preview: http://%s\nHyperFrames %s runtime · read-only player · Ctrl+C stops the server\n", listener.Addr(), engineVersion)
	if err = server.Serve(listener); err != nil && err != http.ErrServerClosed {
		return commandError(errOut, err)
	}
	return 0
}
func previewHandler(root, runtimeJS string) (http.Handler, error) {
	raw, err := os.ReadFile(filepath.Join(root, "index.html"))
	if err != nil {
		return nil, err
	}
	dimension := func(name string, fallback int) int {
		re := regexp.MustCompile(`data-` + name + `=["']([0-9]+)["']`)
		m := re.FindSubmatch(raw)
		if len(m) != 2 {
			return fallback
		}
		v, _ := strconv.Atoi(string(m[1]))
		if v < 1 || v > 7680 {
			return fallback
		}
		return v
	}
	width, height := dimension("width", 1920), dimension("height", 1080)
	page, _ := bundled.ReadFile("assets/runtime/preview.html")
	page = bytes.ReplaceAll(page, []byte("__WIDTH__"), []byte(strconv.Itoa(width)))
	page = bytes.ReplaceAll(page, []byte("__HEIGHT__"), []byte(strconv.Itoa(height)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, "read-only preview", http.StatusMethodNotAllowed)
			return
		}
		switch r.URL.Path {
		case "/favicon.ico":
			w.WriteHeader(http.StatusNoContent)
			return
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(page)
			return
		case "/__web2video/runtime.js":
			w.Header().Set("Content-Type", "application/javascript")
			http.ServeFile(w, r, runtimeJS)
			return
		case "/__web2video/composition":
			html, e := os.ReadFile(filepath.Join(root, "index.html"))
			if e != nil {
				http.Error(w, e.Error(), 500)
				return
			}
			head := regexp.MustCompile(`(?i)<head\b[^>]*>`)
			html = head.ReplaceAllFunc(html, func(tag []byte) []byte {
				return append(append([]byte(nil), tag...), []byte(`<base href="/"><script>(()=>{const originalContext=HTMLCanvasElement.prototype.getContext;HTMLCanvasElement.prototype.getContext=function(type,...args){if(/^(webgl2?|experimental-webgl)$/i.test(type))throw new Error('WEB2VIDEO_WEBGL_FORBIDDEN');return originalContext.call(this,type,...args);};})();</script>`)...)
			})
			html = bytes.Replace(html, []byte("</body>"), []byte(`<script src="/__web2video/runtime.js"></script></body>`), 1)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(html)
			return
		}
		rel := strings.TrimPrefix(r.URL.Path, "/")
		for _, part := range strings.Split(rel, "/") {
			if strings.HasPrefix(part, ".") {
				http.NotFound(w, r)
				return
			}
		}
		path, e := projectFilePath(root, rel)
		if e != nil {
			http.NotFound(w, r)
			return
		}
		info, e := os.Stat(path)
		if e != nil || !info.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}
		if filepath.Ext(path) == ".wgsl" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		}
		http.ServeFile(w, r, path)
	}), nil
}
