package webui

import (
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestRuntimeTelemetryPrecedesApplicationOnEveryPage(t *testing.T) {
	root := fstest.MapFS{
		"index.html":    {Data: []byte(`<head><!--BOOKINGS_TELEMETRY--><script src="/telemetry.js"></script><script type="module" src="/assets/app.js"></script></head>`)},
		"assets/app.js": {Data: []byte(`throw new Error("fixture")`)},
		"telemetry.js":  {Data: []byte(`void 0`)},
	}
	serve := handler(root, `https://collector.test/collect/public"><script>unsafe()</script>`, "bookings")
	for _, route := range []string{"/", "/admin", "/connect", "/meet/private-slug"} {
		response := httptest.NewRecorder()
		serve.ServeHTTP(response, httptest.NewRequest("GET", route, nil))
		body := response.Body.String()
		if response.Code != 200 || strings.Contains(body, "<script>unsafe()") {
			t.Fatalf("unsafe or missing runtime metadata on %s: %s", route, body)
		}
		metadata := strings.Index(body, `name="faro-collector-url"`)
		bootstrap := strings.Index(body, `src="/telemetry.js"`)
		application := strings.Index(body, `type="module"`)
		if metadata < 0 || metadata > bootstrap || bootstrap > application || !strings.Contains(body, "/assets/app.js") {
			t.Fatalf("capture must initialize before app code on %s", route)
		}
	}
	response := httptest.NewRecorder()
	serve.ServeHTTP(response, httptest.NewRequest("GET", "/assets/missing.js", nil))
	if response.Code != 404 {
		t.Fatal("missing scripts must not masquerade as successful HTML responses")
	}
}
