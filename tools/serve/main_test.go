package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestHandler checks what the browser needs from the server: files from
// the folder, WebAssembly with its proper type (browsers refuse to stream
// it otherwise), and no caching, so a rebuild shows up on reload.
func TestHandler(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{"index.html": "<p>hi</p>", "codebreaker.wasm": "\x00asm"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	h := handler(dir)

	tests := []struct {
		path, wantType string
		wantCode       int
	}{
		{"/", "text/html; charset=utf-8", http.StatusOK},
		{"/codebreaker.wasm", "application/wasm", http.StatusOK},
		{"/missing.js", "", http.StatusNotFound},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", tt.path, nil))
		if rec.Code != tt.wantCode {
			t.Errorf("GET %s: status %d, want %d", tt.path, rec.Code, tt.wantCode)
		}
		if tt.wantType != "" && rec.Header().Get("Content-Type") != tt.wantType {
			t.Errorf("GET %s: Content-Type %q, want %q", tt.path, rec.Header().Get("Content-Type"), tt.wantType)
		}
		// Go's file server drops caching headers from error responses, so
		// only files that were found are checked.
		if rec.Code == http.StatusOK && rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("GET %s: Cache-Control %q, want no-store", tt.path, rec.Header().Get("Cache-Control"))
		}
	}
}
