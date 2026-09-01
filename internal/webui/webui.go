package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var content embed.FS

func Handler() http.Handler {
	root, err := fs.Sub(content, "dist")
	if err != nil {
		panic(err)
	}
	files := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requested := strings.TrimPrefix(path.Clean(request.URL.Path), "/")
		if requested == "." {
			requested = "index.html"
		}
		if _, err := fs.Stat(root, requested); err != nil {
			request.URL.Path = "/"
		}
		if strings.Contains(request.URL.Path, "/assets/") {
			response.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			response.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(response, request)
	})
}
