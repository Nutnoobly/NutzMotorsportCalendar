package views_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/Nutnoobly/NutzMotorsportCalendar/internal/views"
)

func TestLayoutThemeAndNavbar(t *testing.T) {
	var buf bytes.Buffer
	component := views.Layout("Test Page")
	err := component.Render(context.Background(), &buf)
	if err != nil {
		t.Fatalf("unexpected render error: %v", err)
	}

	html := buf.String()

	// 1. Verify dark mode is default on html tag
	if !strings.Contains(html, `<html lang="en" class="h-full dark">`) {
		t.Errorf("expected <html ... class=\"h-full dark\">, got:\n%s", html)
	}

	// 2. Verify FOUC prevention script is in head
	if !strings.Contains(html, "localStorage.getItem('theme') === 'light'") {
		t.Errorf("expected theme check in head script")
	}

	// 3. Verify static/theme.js is loaded
	if !strings.Contains(html, `src="/static/theme.js"`) {
		t.Errorf("expected script tag for /static/theme.js")
	}

	// 4. Verify theme toggle button is present
	if !strings.Contains(html, `id="theme-toggle"`) {
		t.Errorf("expected #theme-toggle button in navbar")
	}
	if !strings.Contains(html, `onclick="toggleTheme()"`) {
		t.Errorf("expected onclick=\"toggleTheme()\" on button")
	}

	// 5. Verify Calendar button in navbar is removed
	if strings.Contains(html, `>Calendar</a>`) {
		t.Errorf("expected calendar link in navbar to be removed")
	}
}
