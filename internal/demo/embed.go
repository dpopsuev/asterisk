package demo

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed frontend/dist/*
var frontendFS embed.FS

// FrontendFS returns an http.FileSystem serving the embedded presentation SPA.
func FrontendFS() http.FileSystem {
	sub, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		return nil
	}
	return http.FS(sub)
}
