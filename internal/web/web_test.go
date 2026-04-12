package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandler(t *testing.T) {
	h := Handler()

	tests := []struct {
		name              string
		path              string
		wantStatus        int
		wantBodyContains  string
		wantContentType   string
		wantCacheControl  string
	}{
		{
			name:             "root serves index.html",
			path:             "/",
			wantStatus:       http.StatusOK,
			wantBodyContains: "Observer",
			wantContentType:  "text/html; charset=utf-8",
		},
		{
			name:             "SPA fallback for unknown path",
			path:             "/flows/abc123",
			wantStatus:       http.StatusOK,
			wantBodyContains: "Observer",
			wantContentType:  "text/html; charset=utf-8",
		},
		{
			name:       "assets nonexistent returns 404",
			path:       "/assets/nonexistent.js",
			wantStatus: http.StatusNotFound,
		},
		{
			name:             "assets existing file returns 200 with cache header",
			path:             "/assets/test.js",
			wantStatus:       http.StatusOK,
			wantBodyContains: `console.log("ok")`,
			wantCacheControl: "public, max-age=31536000, immutable",
		},
		{
			// The SPA fallback for unknown paths must set text/html content-type.
			// (http.FileServer redirects /index.html → / with 301, so we use a
			// SPA path to exercise the fallback branch that sets Content-Type.)
			name:            "SPA fallback content-type is text/html",
			path:            "/some/deep/spa/route",
			wantStatus:      http.StatusOK,
			wantContentType: "text/html; charset=utf-8",
		},
		{
			name:             "nested asset request returns correct bytes",
			path:             "/assets/test.js",
			wantStatus:       http.StatusOK,
			wantBodyContains: `console.log("ok")`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantBodyContains != "" {
				body := rec.Body.String()
				if !strings.Contains(body, tc.wantBodyContains) {
					t.Errorf("body %q does not contain %q", body, tc.wantBodyContains)
				}
			}
			if tc.wantContentType != "" {
				ct := rec.Header().Get("Content-Type")
				if !strings.Contains(ct, tc.wantContentType) {
					t.Errorf("Content-Type = %q, want %q", ct, tc.wantContentType)
				}
			}
			if tc.wantCacheControl != "" {
				cc := rec.Header().Get("Cache-Control")
				if cc != tc.wantCacheControl {
					t.Errorf("Cache-Control = %q, want %q", cc, tc.wantCacheControl)
				}
			}
		})
	}
}

func TestHandlerAssetsMissingAlways404(t *testing.T) {
	h := Handler()
	// Verify that a missing asset path does NOT fall back to index.html.
	req := httptest.NewRequest(http.MethodGet, "/assets/missing.wasm", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing asset, got %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "Observer") {
		t.Error("missing asset must not fall back to index.html")
	}
}
