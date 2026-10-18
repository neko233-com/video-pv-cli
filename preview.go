package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func runPreview(args []string, stdout, stderr io.Writer) int {
	f := flag.NewFlagSet("preview", flag.ContinueOnError)
	f.SetOutput(stderr)
	port := f.Int("port", 7335, "loopback HTTP port; 0 chooses a free port")
	runtimeProject := f.String("renderer-project", "", "prepared project; default local dependencies or current-user runtime")
	out := f.String("out", "work/browser-validation", "stable browser evidence directory")
	delivery := f.String("bundle", "", "optional verified website bundle served at /delivery/background.html")
	if code := parseFlags(f, args); code >= 0 {
		return code
	}
	if *port < 0 || *port > 65535 {
		fmt.Fprintln(stderr, "invalid port")
		return 2
	}
	runtimeRoot, err := preparedRuntimeRoot(*runtimeProject, ".")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if _, err := rendererPath(runtimeRoot); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	templates, transitions, err := readCatalog()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	output, err := filepath.Abs(*out)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if err := prepareOwnedEvidence(output, "video-pv-cli/browser-evidence/1"); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	origin := "http://" + listener.Addr().String()
	handler := newPreviewHandler(runtimeRoot, output, origin, templates, transitions)
	if *delivery != "" {
		handler, err = websiteDeliveryHandler(*delivery, origin, handler)
		if err != nil {
			_ = listener.Close()
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stdout, "Website background: %s/delivery/background.html\n", origin)
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: time.Minute}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	go func() {
		<-ctx.Done()
		stop, done := context.WithTimeout(context.Background(), 3*time.Second)
		defer done()
		_ = server.Shutdown(stop)
	}()
	fmt.Fprintf(stdout, "PV browser workbench: %s\nEvidence replaces files inside: %s\n", origin, output)
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func websiteDeliveryHandler(root, origin string, fallback http.Handler) (http.Handler, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(root, "bundle-report.json"))
	var report DeliveryBundle
	if err != nil || json.Unmarshal(data, &report) != nil || !report.Passed || report.Schema != "video-pv-cli/delivery/1" || report.Target != "website" {
		return nil, fmt.Errorf("preview requires a successful website delivery bundle")
	}
	files := map[string]string{}
	for _, name := range []string{"background.html", "background.mp4", "poster.png"} {
		path, err := projectFilePath(root, name)
		if err != nil {
			return nil, err
		}
		hash, err := fileSHA256(path)
		if err != nil || hash != report.Files[name] {
			return nil, fmt.Errorf("delivery bundle changed: %s; rebuild the bundle", name)
		}
		files["/delivery/"+name] = path
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/delivery-audit" && r.Host == strings.TrimPrefix(origin, "http://") && r.Method == "GET" {
			data, _ := os.ReadFile(files["/delivery/background.html"])
			audit, _ := bundled.ReadFile("assets/preview/delivery.html")
			data = bytes.Replace(data, []byte("</html>"), append(audit, []byte("</html>")...), 1)
			data = bytes.Replace(data, []byte("<style>"), []byte(`<base href="/delivery/"><style>`), 1)
			// Keep the foreground anchor on the audit page despite the media base URL.
			data = bytes.ReplaceAll(data, []byte(`href="#details"`), []byte(`href="/delivery-audit#details"`))
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			_, _ = w.Write(data)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/delivery/") {
			fallback.ServeHTTP(w, r)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		if r.Host != strings.TrimPrefix(origin, "http://") {
			http.Error(w, "unexpected host", 403)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			w.WriteHeader(405)
			return
		}
		path, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, path)
	}), nil
}

func prepareOwnedEvidence(root, schema string) error {
	marker := filepath.Join(root, ".pv-evidence-owner.json")
	if entries, err := os.ReadDir(root); err == nil && len(entries) > 0 {
		data, e := os.ReadFile(marker)
		if e != nil {
			return fmt.Errorf("existing evidence directory is not owned by video-pv-cli: %s", root)
		}
		var owner struct {
			Schema string `json:"schema"`
		}
		if json.Unmarshal(data, &owner) != nil || owner.Schema != schema {
			return fmt.Errorf("evidence ownership mismatch")
		}
		return nil
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}
	data, _ := json.Marshal(map[string]string{"schema": schema})
	return writeFileAtomic(marker, append(data, '\n'))
}

func newPreviewHandler(runtimeRoot, output, origin string, templates map[string]TemplateData, transitions map[string]TransitionData) http.Handler {
	styles, _ := readStyles(templates, transitions)
	sourceDigests := previewSourceDigests()
	atlas, browserCases := browserAtlas(templates, transitions)
	jsonReply := func(w http.ResponseWriter, value any) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(value)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Host != strings.TrimPrefix(origin, "http://") {
			http.Error(w, "unexpected host", 403)
			return
		}
		if r.Method == http.MethodPost {
			if r.Header.Get("Origin") != origin {
				http.Error(w, "same-origin request required", 403)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 16<<20)
			if r.URL.Path == "/api/report" || r.URL.Path == "/api/background-report" {
				data, err := io.ReadAll(r.Body)
				if err != nil || !json.Valid(data) {
					http.Error(w, "invalid report", 400)
					return
				}
				name := "edge-report.json"
				if r.URL.Path == "/api/background-report" {
					name = "background-report.json"
				}
				if err := writeFileAtomic(filepath.Join(output, name), data); err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
				jsonReply(w, map[string]string{"saved": name})
				return
			}
			if r.URL.Path == "/api/frame" {
				var body struct {
					PNG string `json:"png"`
				}
				if json.NewDecoder(r.Body).Decode(&body) != nil {
					http.Error(w, "invalid frame", 400)
					return
				}
				data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(body.PNG, "data:image/png;base64,"))
				if err != nil || len(data) < 8 || string(data[:8]) != "\x89PNG\r\n\x1a\n" {
					http.Error(w, "invalid PNG", 400)
					return
				}
				if err := writeFileAtomic(filepath.Join(output, "frame.png"), data); err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
				jsonReply(w, map[string]string{"saved": "frame.png"})
				return
			}
			http.NotFound(w, r)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			w.WriteHeader(405)
			return
		}
		if r.URL.Path == "/api/catalog" {
			tpl := []TemplateData{}
			tr := []TransitionData{}
			for _, t := range templates {
				tpl = append(tpl, t)
			}
			for _, t := range transitions {
				tr = append(tr, t)
			}
			sort.Slice(tpl, func(i, j int) bool { return tpl[i].ID < tpl[j].ID })
			sort.Slice(tr, func(i, j int) bool { return tr[i].ID < tr[j].ID })
			styleList := []StylePreset{}
			for _, s := range styles {
				styleList = append(styleList, s)
			}
			sort.Slice(styleList, func(i, j int) bool { return styleList[i].ID < styleList[j].ID })
			jsonReply(w, map[string]any{"templates": tpl, "transitions": tr, "styles": styleList, "source_sha256": sourceDigests})
			return
		}
		if r.URL.Path == "/api/browser-cases" {
			jsonReply(w, browserCases)
			return
		}
		var data []byte
		var err error
		path := r.URL.Path
		if path == "/" {
			data, err = bundled.ReadFile("assets/preview/index.html")
			path = "index.html"
		} else if path == "/gallery" || path == "/gallery/" || path == "/gallery/sprite.png" || path == "/gallery/index.json" {
			file := "index.html"
			if strings.HasSuffix(path, "sprite.png") {
				file = "sprite.png"
			}
			if strings.HasSuffix(path, "index.json") {
				file = "index.json"
			}
			data, err = bundled.ReadFile("docs/style-gallery/" + file)
			path = file
			if file == "index.html" {
				data = bytes.Replace(data, []byte("<meta charset="), []byte("<base href=\"/gallery/\"><meta charset="), 1)
			}
		} else if strings.HasPrefix(path, "/multiplane/runtime/") {
			file := strings.TrimPrefix(path, "/multiplane/runtime/")
			if file != "shot.js" && file != "shot.css" {
				http.NotFound(w, r)
				return
			}
			data, err = os.ReadFile(filepath.Join(runtimeRoot, "node_modules", "multiplane", "src", "runtime", file))
		} else {
			parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
			if len(parts) != 3 || (parts[0] != "template" && parts[0] != "transition" && parts[0] != "atlas" && parts[0] != "style") {
				http.NotFound(w, r)
				return
			}
			var template TemplateData
			var ok bool
			if parts[0] == "atlas" {
				ok = parts[1] == "all"
			} else if parts[0] == "style" {
				var s StylePreset
				s, ok = styles[parts[1]]
				if ok {
					template = applyStyle(templates[s.Render.BaseTemplate], s)
				}
			} else if parts[0] == "template" {
				template, ok = templates[parts[1]]
			} else if _, ok = transitions[parts[1]]; ok {
				template = templates["keynote-product-porcelain"]
				template.ID = "transition-preview-" + parts[1]
				template.Scenes = append([]SceneData(nil), template.Scenes[:2]...)
				template.Scenes[0].Title = "BEFORE"
				template.Scenes[1].Title = "AFTER"
				template.Scenes[0].Type = "hero-device"
				template.Scenes[1].Type = "bento"
				for i := range template.Scenes {
					template.Scenes[i].Duration = 2.5
					template.Scenes[i].TransitionDuration = 1.2
					template.Scenes[i].Transition = parts[1]
					template.Scenes[i].Motion = MotionData{Name: "none", Strength: 0, Ease: "linear"}
				}
			}
			if !ok {
				http.NotFound(w, r)
				return
			}
			file := parts[2]
			path = file
			if file == "template.json" {
				if parts[0] == "atlas" {
					jsonReply(w, atlas)
				} else {
					jsonReply(w, template)
				}
				return
			}
			if file == "transitions.json" {
				jsonReply(w, transitions)
				return
			}
			if file == "astral-knight.png" {
				data, err = bundled.ReadFile("assets/character/" + file)
			} else {
				data, err = bundled.ReadFile("assets/starter/" + file)
			}
			if file == "catalog-runtime.js" && err == nil {
				marker := []byte("globalThis.__PV_WEBGPU_FLUSH=()=>shader.flush();")
				if bytes.Count(data, marker) != 1 {
					http.Error(w, "preview inspection contract changed", 500)
					return
				}
				data = bytes.Replace(data, marker, append(append([]byte(nil), marker...), []byte("globalThis.__PV_WEBGPU_DEVICE=shader.device;globalThis.__PV_WEBGPU_DISPOSE=()=>shader.dispose();")...), 1)
			}
			if file == "webgpu-runtime.js" && err == nil {
				for _, change := range [][2]string{{"let fatal;", "let fatal;let disposed=false;"}, {"const fail=reason=>{fatal=", "const fail=reason=>{if(disposed)return;fatal="}, {"metadata,device,", "metadata,device,dispose(){disposed=true;device.destroy();},"}} {
					if bytes.Count(data, []byte(change[0])) != 1 {
						http.Error(w, "preview disposal contract changed", 500)
						return
					}
					data = bytes.Replace(data, []byte(change[0]), []byte(change[1]), 1)
				}
			}
		}
		if err != nil {
			http.NotFound(w, r)
			return
		}
		contentType := mime.TypeByExtension(filepath.Ext(path))
		if contentType == "" {
			contentType = "text/plain; charset=utf-8"
		}
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write(data)
	})
}
