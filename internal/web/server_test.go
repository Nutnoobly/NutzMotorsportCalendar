package web_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nutnoobly/NutzMotorsportCalendar/internal/web"
)

func TestStaticThemeFile(t *testing.T) {
	// Locate repository root
	cwd, _ := os.Getwd()
	repoRoot := filepath.Join(cwd, "../..")
	if err := os.Chdir(repoRoot); err != nil {
		t.Skipf("skipping test; could not change directory to %s: %v", repoRoot, err)
	}
	defer os.Chdir(cwd)

	server := web.NewServer(nil, "secret")
	handler := server.Routes()

	req := httptest.NewRequest("GET", "/static/theme.js", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "window.toggleTheme") {
		t.Errorf("expected theme.js to contain window.toggleTheme")
	}
	if !strings.Contains(body, "localStorage.getItem('theme')") {
		t.Errorf("expected theme.js to check localStorage")
	}
}
