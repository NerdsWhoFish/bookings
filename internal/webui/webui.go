package webui

import (
	"bytes"
	"embed"
	"encoding/json"
	"html"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

//go:embed all:dist
var content embed.FS

func Handler(faroURL, appName string) http.Handler {
	root, err := fs.Sub(content, "dist")
	if err != nil {
		panic(err)
	}
	return handler(root, faroURL, appName)
}

func handler(root fs.FS, faroURL, appName string) http.Handler {
	files := http.FileServer(http.FS(root))
	index, err := fs.ReadFile(root, "index.html")
	if err != nil {
		return http.NotFoundHandler()
	}
	assets, err := fs.Glob(root, "assets/*.js")
	if err != nil {
		panic(err)
	}
	for i := range assets {
		assets[i] = "/" + assets[i]
	}
	assets = append(assets, "/telemetry.js")
	assetJSON, _ := json.Marshal(assets)
	metadata := `<meta name="faro-collector-url" content="` + html.EscapeString(faroURL) + `"><meta name="faro-app-name" content="` + html.EscapeString(appName) + `"><meta name="faro-assets" content="` + html.EscapeString(string(assetJSON)) + `">`
	index = bytes.Replace(index, []byte("<!--BOOKINGS_TELEMETRY-->"), []byte(metadata), 1)
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requested := strings.TrimPrefix(path.Clean(request.URL.Path), "/")
		if requested == "." {
			requested = "index.html"
		}
		if _, err := fs.Stat(root, requested); err != nil {
			if path.Ext(requested) != "" {
				http.NotFound(response, request)
				return
			}
			requested = "index.html"
		}
		if strings.Contains(request.URL.Path, "/assets/") {
			response.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			response.Header().Set("Cache-Control", "no-cache")
		}
		if requested == "index.html" {
			http.ServeContent(response, request, "index.html", time.Time{}, bytes.NewReader(index))
			return
		}
		files.ServeHTTP(response, request)
	})
}
