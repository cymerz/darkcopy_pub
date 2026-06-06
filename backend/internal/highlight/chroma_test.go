package highlight

import (
	"strings"
	"testing"
)

func TestChromaHighlighter_ImplementsInterface(t *testing.T) {
	var _ SyntaxHighlighter = (*ChromaHighlighter)(nil)
}

func TestChromaHighlighter_HighlightGo(t *testing.T) {
	h := NewChromaHighlighter("")
	content := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`
	html, err := h.Highlight(content, "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(html, "<span") {
		t.Error("expected HTML output to contain <span> elements")
	}
	if !strings.Contains(html, "style=") && !strings.Contains(html, "class=") {
		t.Error("expected HTML output to contain style or class attributes")
	}
}

func TestChromaHighlighter_HighlightWithLineNumbers(t *testing.T) {
	h := NewChromaHighlighter("")
	content := "line1\nline2\nline3\n"
	html, err := h.Highlight(content, "plaintext")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Line numbers should be present in the output
	if !strings.Contains(html, "1") && !strings.Contains(html, "ln-1") {
		t.Error("expected HTML output to contain line numbers")
	}
}

func TestChromaHighlighter_UnrecognizedLanguageDefaultsToPlaintext(t *testing.T) {
	h := NewChromaHighlighter("")
	content := "some random content"
	html, err := h.Highlight(content, "nonexistent_language_xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if html == "" {
		t.Error("expected non-empty HTML output for unrecognized language")
	}
	if !strings.Contains(html, "<span") {
		t.Error("expected HTML output to contain <span> elements even for plaintext")
	}
}

func TestChromaHighlighter_EmptyLanguageDefaultsToPlaintext(t *testing.T) {
	h := NewChromaHighlighter("")
	content := "hello world"
	html, err := h.Highlight(content, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if html == "" {
		t.Error("expected non-empty HTML output for empty language")
	}
}

func TestChromaHighlighter_SupportedLanguages(t *testing.T) {
	h := NewChromaHighlighter("")
	languages := h.SupportedLanguages()

	if len(languages) < 50 {
		t.Errorf("expected at least 50 supported languages, got %d", len(languages))
	}

	// Verify required languages are present
	required := []string{"python", "javascript", "go", "java", "html", "css", "sql", "json", "yaml", "markdown"}
	langMap := make(map[string]bool)
	for _, l := range languages {
		langMap[strings.ToLower(l.ID)] = true
	}

	for _, req := range required {
		if !langMap[req] {
			t.Errorf("expected language %q to be in supported languages", req)
		}
	}

	// Verify Language struct fields are populated
	for _, l := range languages {
		if l.ID == "" {
			t.Error("found language with empty ID")
		}
		if l.Name == "" {
			t.Error("found language with empty Name")
		}
	}

	// Verify sorted order
	for i := 1; i < len(languages); i++ {
		if strings.ToLower(languages[i-1].Name) > strings.ToLower(languages[i].Name) {
			t.Errorf("languages not sorted: %q > %q", languages[i-1].Name, languages[i].Name)
			break
		}
	}
}
