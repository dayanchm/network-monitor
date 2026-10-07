package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestWebRoutes(t *testing.T) {
	t.Chdir("../..")
	mux := http.NewServeMux()
	registerWebRoutes(mux)
	for _, path := range []string{"/devices", "/history", "/connection", "/sites", "/settings/wifi", "/assets/common.js", "/assets/styles.css"} {
		r := httptest.NewRecorder()
		mux.ServeHTTP(r, httptest.NewRequest("GET", path, nil))
		if r.Code != http.StatusOK {
			t.Errorf("%s: status %d", path, r.Code)
		}
	}
	r := httptest.NewRecorder()
	mux.ServeHTTP(r, httptest.NewRequest("GET", "/", nil))
	if r.Code != http.StatusFound || r.Header().Get("Location") != "/devices" {
		t.Fatal("missing root redirect")
	}
	for _, name := range []string{"devices", "history", "connection", "sites", "wifi"} {
		if _, err := os.Stat(filepath.Join("web/assets", name+".js")); err != nil {
			t.Error(err)
		}
	}
}
