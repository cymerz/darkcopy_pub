// Package urlgen provides the URL slug generator interface and implementation.
package urlgen

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
)

// ErrSlugGenerationFailed is returned when all retry attempts to generate a unique slug have been exhausted.
var ErrSlugGenerationFailed = errors.New("failed to generate unique slug after 10 attempts")

// charset contains the allowed characters for slug generation: a-z and 0-9.
const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

// slugLength is the number of characters in a generated slug.
const slugLength = 8

// maxRetries is the maximum number of attempts to generate a unique slug.
const maxRetries = 10

// URLGenerator defines the interface for generating unique URL slugs.
type URLGenerator interface {
	GenerateSlug(ctx context.Context) (string, error)
}

// SlugExistsFunc is a function type that checks whether a given slug already exists.
// It returns true if the slug is already taken, false otherwise.
type SlugExistsFunc func(ctx context.Context, slug string) (bool, error)

// Generator is the concrete implementation of URLGenerator.
// It uses crypto/rand for secure random slug generation and supports
// an injectable slug existence checker for uniqueness verification.
type Generator struct {
	slugExists SlugExistsFunc
}

// NewGenerator creates a new Generator with the given slug existence checker.
// The slugExists function is called to verify that a generated slug is unique.
// If slugExists is nil, no uniqueness check is performed (useful for testing).
func NewGenerator(slugExists SlugExistsFunc) *Generator {
	return &Generator{
		slugExists: slugExists,
	}
}

// GenerateSlug generates a unique 8-character alphanumeric slug using crypto/rand.
// It retries up to 10 times if a collision is detected (slug already exists).
// Returns ErrSlugGenerationFailed if all attempts are exhausted.
func (g *Generator) GenerateSlug(ctx context.Context) (string, error) {
	for attempt := 0; attempt < maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		slug, err := generateRandomSlug()
		if err != nil {
			return "", err
		}

		// If no existence checker is configured, return the slug directly.
		if g.slugExists == nil {
			return slug, nil
		}

		exists, err := g.slugExists(ctx, slug)
		if err != nil {
			return "", err
		}

		if !exists {
			return slug, nil
		}
		// Slug already exists, retry with a new one.
	}

	return "", ErrSlugGenerationFailed
}

// generateRandomSlug creates a random 8-character string from the charset using crypto/rand.
func generateRandomSlug() (string, error) {
	charsetLen := big.NewInt(int64(len(charset)))
	slug := make([]byte, slugLength)

	for i := range slug {
		idx, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", err
		}
		slug[i] = charset[idx.Int64()]
	}

	return string(slug), nil
}
