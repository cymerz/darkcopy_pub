package templates_test

import (
	"html/template"
	"os"
	"path/filepath"
	"testing"
)

func TestTemplatesParse(t *testing.T) {
	// Find the templates directory relative to this test file.
	dir := "."
	if _, err := os.Stat(filepath.Join(dir, "base.html")); err != nil {
		t.Fatalf("cannot find templates directory: %v", err)
	}

	// Define template function map (safeHTML is commonly needed).
	funcMap := template.FuncMap{
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
	}

	// Test that each template can be parsed together with base.html.
	templates := []string{
		"index.html",
		"paste_new.html",
		"paste_view.html",
		"paste_unlock.html",
		"upload.html",
		"file_unlock.html",
		"error.html",
	}

	for _, tmplFile := range templates {
		t.Run(tmplFile, func(t *testing.T) {
			_, err := template.New("").Funcs(funcMap).ParseFiles(
				filepath.Join(dir, "base.html"),
				filepath.Join(dir, tmplFile),
			)
			if err != nil {
				t.Errorf("failed to parse %s: %v", tmplFile, err)
			}
		})
	}
}
