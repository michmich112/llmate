package main

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:frontend_dist
var frontendFS embed.FS

// frontendHandler returns an http.Handler that serves the embedded SPA.
// Unknown paths fall back to index.html so client-side routing works.
func frontendHandler() http.Handler {
	sub, err := fs.Sub(frontendFS, "frontend_dist")
	if err != nil {
		// This should never happen since the directory is embedded at build time.
		panic("frontend_dist not found in embed.FS: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// fs.FS paths must not have a leading slash; strip it before probing.
		fsPath := strings.TrimPrefix(r.URL.Path, "/")
		f, err := sub.Open(fsPath)
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		// File not found — serve index.html for SPA client-side routing.
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		fileServer.ServeHTTP(w, r2)
	})
}
