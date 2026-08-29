package dashboard

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

// The release build replaces the contents of dist with the Next.js static export
// before compiling SkillBox, so the complete Dashboard becomes part of the binary.
//
// The all: prefix is required because Next.js stores its runtime assets under
// _next, and embed otherwise excludes files and directories starting with _.
//
//go:embed all:dist
var embedded embed.FS

func Handler() http.Handler {
	assets, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic(err)
	}
	return newHandler(assets)
}

func newHandler(assets fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		name := path.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		if name == "." {
			name = "index.html"
		}
		if name == ".." || strings.HasPrefix(name, "../") {
			http.NotFound(w, r)
			return
		}
		if info, err := fs.Stat(assets, name); err == nil && info.IsDir() {
			name = path.Join(name, "index.html")
		}

		status := http.StatusOK
		data, err := fs.ReadFile(assets, name)
		if err != nil {
			data, err = fs.ReadFile(assets, "404.html")
			if err != nil {
				http.NotFound(w, r)
				return
			}
			name = "404.html"
			status = http.StatusNotFound
		}

		if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		if strings.HasPrefix(name, "_next/static/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(status)
		if r.Method == http.MethodGet {
			_, _ = w.Write(data)
		}
	})
}
