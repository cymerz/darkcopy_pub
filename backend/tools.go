//go:build tools

// Package pastebin declares tool and test dependencies that are used
// across the project but may not be directly imported in production code yet.
package pastebin

import (
	_ "github.com/alecthomas/chroma/v2"
	_ "github.com/go-chi/chi/v5"
	_ "golang.org/x/crypto/bcrypt"
	_ "pgregory.net/rapid"
)
