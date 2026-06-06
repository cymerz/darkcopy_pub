package file

import (
	"context"
	"testing"
	"time"
)

func TestS3Storage_PresignURL_Public_Download(t *testing.T) {
	storage := &S3Storage{
		bucket:       "erwinbackup",
		customDomain: "cdn.beta.qzz.io/file/",
		isPublic:     true,
	}

	url, err := storage.PresignURL(context.Background(), "uploads/xik4d7q0/file.jpg", 1*time.Hour, false) // inline = false (Download)
	if err != nil {
		t.Fatalf("failed to presign public URL: %v", err)
	}

	expected := "https://cdn.beta.qzz.io/file/erwinbackup/uploads/xik4d7q0/file.jpg?download=true"
	if url != expected {
		t.Errorf("expected URL %q, got %q", expected, url)
	}
}

func TestS3Storage_PresignURL_Public_Preview(t *testing.T) {
	storage := &S3Storage{
		bucket:       "erwinbackup",
		customDomain: "cdn.beta.qzz.io/file/",
		isPublic:     true,
	}

	url, err := storage.PresignURL(context.Background(), "uploads/xik4d7q0/file.jpg", 1*time.Hour, true) // inline = true (Preview)
	if err != nil {
		t.Fatalf("failed to presign public URL: %v", err)
	}

	expected := "https://cdn.beta.qzz.io/file/erwinbackup/uploads/xik4d7q0/file.jpg"
	if url != expected {
		t.Errorf("expected URL %q, got %q", expected, url)
	}
}
