// Package file provides the file service interface.
package file

import (
	"context"
	"net/http"

	"github.com/gthbn/pastebin/internal/paste"
)

// FileService defines the interface for file upload and retrieval operations.
type FileService interface {
	Upload(ctx context.Context, req paste.UploadFileRequest) (*paste.FileRecord, error)
	GetBySlug(ctx context.Context, slug string) (*paste.FileRecord, error)
	ValidatePassword(ctx context.Context, slug, password string) (bool, error)
	ServeFile(ctx context.Context, slug string, w http.ResponseWriter) error
	PresignDownloadURL(ctx context.Context, slug string, inline bool) (string, error)
}
