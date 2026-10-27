//go:build embedui

package server

import (
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"embed"
	"encoding/hex"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
)

// webuiDist holds the statically-exported frontend (built with
// `cd web && npm run build:static`, then copied into server/webui/dist). Compiled
// into the binary only when building with `-tags embedui`. The `all:` prefix is
// required so Next's `_next/` asset dir (leading underscore) is included.
//
//go:embed all:webui/dist
var webuiDist embed.FS

// gzipCache keeps one compressed copy per path for the process lifetime.
// embed.FS content never changes after startup.
var gzipCache sync.Map // path -> []byte

type gzipEntry struct {
	body []byte
	etag string
}

// webuiHandler serves the embedded SPA. Public (no JWT) — auth is enforced
// client-side and on /api. Serves the exported per-route index.html files and
// falls back to index.html so client-side routing still resolves unknown paths.
func (s *Server) webuiHandler() http.Handler {
	root, err := fs.Sub(webuiDist, "webui/dist")
	if err != nil {
		panic(err)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "" {
			p = "index.html"
		}
		// 1) exact file (assets: _next/*, favicon.ico, ...)
		// 2) route dir → <p>/index.html (trailingSlash export)
		// 3) <p>.html
		// 4) SPA 兜底 → index.html（交给客户端路由）
		for _, cand := range []string{p, p + "/index.html", p + ".html"} {
			if serveEmbeddedFile(w, r, root, cand) {
				return
			}
		}
		_ = serveEmbeddedFile(w, r, root, "index.html")
	})
}

func serveEmbeddedFile(w http.ResponseWriter, r *http.Request, fsys fs.FS, name string) bool {
	f, err := fsys.Open(name)
	if err != nil {
		return false
	}
	st, statErr := f.Stat()
	if statErr != nil || st.IsDir() {
		_ = f.Close()
		return false
	}
	raw, err := io.ReadAll(f)
	_ = f.Close()
	if err != nil {
		http.Error(w, "read asset", http.StatusInternalServerError)
		return true
	}

	ctype := contentTypeFor(name)
	w.Header().Set("Content-Type", ctype)
	setCacheHeaders(w, name)

	etag := weakETag(raw)
	w.Header().Set("ETag", etag)
	if match := r.Header.Get("If-None-Match"); match != "" && etagMatch(match, etag) {
		w.WriteHeader(http.StatusNotModified)
		return true
	}

	body := raw
	if acceptsGzip(r) && compressible(name) {
		if gz, ok := gzipCached(name, raw); ok {
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Vary", "Accept-Encoding")
			body = gz
		}
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
	return true
}

func gzipCached(name string, raw []byte) ([]byte, bool) {
	if v, ok := gzipCache.Load(name); ok {
		return v.(gzipEntry).body, true
	}
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	if err != nil {
		return nil, false
	}
	if _, err := zw.Write(raw); err != nil {
		_ = zw.Close()
		return nil, false
	}
	if err := zw.Close(); err != nil {
		return nil, false
	}
	out := buf.Bytes()
	// Only keep gzip when it actually shrinks the payload.
	if len(out) >= len(raw) {
		return nil, false
	}
	gzipCache.Store(name, gzipEntry{body: out, etag: weakETag(raw)})
	return out, true
}

func setCacheHeaders(w http.ResponseWriter, name string) {
	if strings.HasPrefix(name, "_next/static/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	if strings.HasSuffix(name, ".html") || name == "index.html" {
		w.Header().Set("Cache-Control", "no-cache")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
}

func compressible(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".js", ".css", ".html", ".svg", ".json", ".txt", ".xml", ".map", ".wasm":
		return true
	default:
		return false
	}
}

func acceptsGzip(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
}

func contentTypeFor(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".html":
		return "text/html; charset=utf-8"
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".json":
		return "application/json"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".ico":
		return "image/x-icon"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".map":
		return "application/json"
	default:
		return "application/octet-stream"
	}
}

func weakETag(b []byte) string {
	sum := sha1.Sum(b)
	return `W/"` + hex.EncodeToString(sum[:]) + `"`
}

func etagMatch(header, etag string) bool {
	for _, part := range strings.Split(header, ",") {
		if strings.TrimSpace(part) == etag {
			return true
		}
	}
	return false
}
