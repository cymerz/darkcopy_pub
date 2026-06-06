// Package highlight provides the syntax highlighter interface and types.
package highlight

// Language represents a supported programming language for syntax highlighting.
type Language struct {
	ID   string `json:"id"`   // internal identifier (e.g., "go", "python")
	Name string `json:"name"` // display name (e.g., "Go", "Python")
}

// SyntaxHighlighter defines the interface for syntax highlighting operations.
type SyntaxHighlighter interface {
	Highlight(content, language string) (html string, err error)
	SupportedLanguages() []Language
}
