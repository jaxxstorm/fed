package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed assets/*
var frontend embed.FS

func frontendAvailable() bool { _, err := frontend.ReadFile("assets/index.html"); return err == nil }
func frontendPage(w http.ResponseWriter, r *http.Request) {
	b, err := frontend.ReadFile("assets/index.html")
	if err != nil {
		http.Error(w, "Build the frontend with make build before starting the workspace.", 503)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(b)
}
func frontendAssets() http.Handler {
	root, _ := fs.Sub(frontend, "assets")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/assets/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		http.FileServer(http.FS(root)).ServeHTTP(w, r)
	})
}
