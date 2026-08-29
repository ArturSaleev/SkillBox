package dashboard

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestEmbeddedNextAssetsAreIncluded(t *testing.T) {
	if _, err := fs.Stat(embedded, "dist/index.html"); err != nil {
		t.Skip("Dashboard export has not been generated")
	}

	found := false
	err := fs.WalkDir(embedded, "dist/_next/static", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			found = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded Next.js assets: %v", err)
	}
	if !found {
		t.Fatal("embedded Dashboard contains no _next/static assets")
	}
}

func TestHandlerServesDashboardRoutesAndAssets(t *testing.T) {
	assets := fstest.MapFS{
		"index.html":                       {Data: []byte("home")},
		"404.html":                         {Data: []byte("missing")},
		"skills/view/index.html":           {Data: []byte("skill")},
		"_next/static/chunks/dashboard.js": {Data: []byte("javascript")},
	}
	handler := newHandler(fs.FS(assets))

	tests := []struct {
		path       string
		wantStatus int
		wantBody   string
		wantCache  string
	}{
		{path: "/", wantStatus: http.StatusOK, wantBody: "home", wantCache: "no-cache"},
		{path: "/skills/view/", wantStatus: http.StatusOK, wantBody: "skill", wantCache: "no-cache"},
		{path: "/_next/static/chunks/dashboard.js", wantStatus: http.StatusOK, wantBody: "javascript", wantCache: "public, max-age=31536000, immutable"},
		{path: "/unknown", wantStatus: http.StatusNotFound, wantBody: "missing", wantCache: "no-cache"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if recorder.Code != tt.wantStatus || recorder.Body.String() != tt.wantBody {
				t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
			}
			if got := recorder.Header().Get("Cache-Control"); got != tt.wantCache {
				t.Fatalf("Cache-Control=%q want=%q", got, tt.wantCache)
			}
		})
	}
}

func TestHandlerRejectsWrites(t *testing.T) {
	handler := newHandler(fstest.MapFS{"index.html": {Data: []byte("home")}})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/", nil))
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", recorder.Code)
	}
}
