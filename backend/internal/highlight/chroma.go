package highlight

import (
	"bytes"
	"sort"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// ChromaHighlighter implements SyntaxHighlighter using the Chroma library.
type ChromaHighlighter struct {
	style     string
	formatter *html.Formatter
}

// NewChromaHighlighter creates a new ChromaHighlighter with the given style.
// If style is empty, "dracula" is used as default.
func NewChromaHighlighter(style string) *ChromaHighlighter {
	if style == "" {
		style = "dracula"
	}
	formatter := html.New(
		html.WithLineNumbers(false),
		html.WithClasses(false),
	)
	return &ChromaHighlighter{
		style:     style,
		formatter: formatter,
	}
}

// Highlight applies syntax highlighting to the given content using the specified language.
// If the language is not recognized, it defaults to "plaintext".
// Returns the highlighted HTML string.
func (h *ChromaHighlighter) Highlight(content, language string) (string, error) {
	lexer := lexers.Get(language)
	if lexer == nil {
		lexer = lexers.Get("plaintext")
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	style := styles.Get(h.style)
	if style == nil {
		style = styles.Fallback
	}

	iterator, err := lexer.Tokenise(nil, content)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = h.formatter.Format(&buf, style, iterator)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// SupportedLanguages returns a list of all languages supported by the highlighter.
// The list is sorted alphabetically by name.
func (h *ChromaHighlighter) SupportedLanguages() []Language {
	registry := lexers.GlobalLexerRegistry
	lexerNames := registry.Names(false)

	languages := make([]Language, 0, len(lexerNames))
	seen := make(map[string]bool)

	for _, name := range lexerNames {
		id := strings.ToLower(name)
		if seen[id] {
			continue
		}
		seen[id] = true
		languages = append(languages, Language{
			ID:   id,
			Name: name,
		})
	}

	sort.Slice(languages, func(i, j int) bool {
		return strings.ToLower(languages[i].Name) < strings.ToLower(languages[j].Name)
	})

	return languages
}
