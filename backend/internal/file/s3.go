package file

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Storage implements FileStorage using AWS S3 or compatible API.
type S3Storage struct {
	client       *s3.Client
	bucket       string
	customDomain string // Optional custom download domain/CDN
	isPublic     bool   // Optional setting for public S3 buckets
}

// SetCustomDomain sets an optional custom download domain (e.g. "cdn.example.com" or "cdn.example.com/file/darkcopy")
// to rewrite the generated presigned URLs.
func (s *S3Storage) SetCustomDomain(domain string) {
	s.customDomain = strings.TrimSpace(domain)
}

// SetIsPublic configures whether the S3 bucket is public, allowing the service to bypass
// generating temporary pre-signed query params that break Cloudflare caching.
func (s *S3Storage) SetIsPublic(isPublic bool) {
	s.isPublic = isPublic
}

// NewS3Storage creates a new S3Storage backend.
func NewS3Storage(bucket, region, endpoint, accessKey, secretKey string) (*S3Storage, error) {
	opts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
		config.WithRetryMaxAttempts(1), // Do not retry failures to keep fallback lookups fast
	}

	// Use static credentials if provided in env
	if accessKey != "" && secretKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		))
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("s3 storage: failed to load default config: %w", err)
	}

	// Standardize endpoint format (must start with http:// or https:// for AWS SDK Go v2)
	if endpoint != "" {
		if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
			endpoint = "https://" + endpoint
		}
	}

	// Initialize S3 Client with optional custom endpoint (for MinIO, R2, etc.)
	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true // Path-style is standard for MinIO and custom endpoints
		}
	})

	return &S3Storage{
		client: s3Client,
		bucket: bucket,
	}, nil
}

// Save writes the content from reader to S3 bucket at the given storage key path.
func (s *S3Storage) Save(ctx context.Context, storageKey string, reader io.Reader) error {
	contentType := mime.TypeByExtension(filepath.Ext(storageKey))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(storageKey),
		Body:        reader,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("s3 storage: failed to put object %s: %w", storageKey, err)
	}
	return nil
}

// Open opens the S3 object at the given storage key path for reading.
func (s *S3Storage) Open(ctx context.Context, storageKey string) (io.ReadCloser, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(storageKey),
	}

	// Support HTTP Range queries for video/audio seeking.
	if val := ctx.Value("range_header"); val != nil {
		if rStr, ok := val.(string); ok && rStr != "" {
			input.Range = aws.String(rStr)
		}
	}

	output, err := s.client.GetObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("s3 storage: failed to download object %s: %w", storageKey, err)
	}
	return output.Body, nil
}

// Delete removes the S3 object at the given storage key path.
func (s *S3Storage) Delete(ctx context.Context, storageKey string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(storageKey),
	})
	if err != nil {
		return fmt.Errorf("s3 storage: failed to delete object %s: %w", storageKey, err)
	}
	return nil
}

// Head performs a lightweight existence check for the given storage key
// using S3 HeadObject. Returns nil if the object exists, an error otherwise.
func (s *S3Storage) Head(ctx context.Context, storageKey string) error {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(storageKey),
	})
	if err != nil {
		return fmt.Errorf("s3 storage: head object %s: %w", storageKey, err)
	}
	return nil
}

// PresignURL generates a secure, temporary pre-signed URL for the given storage key,
// allowing direct download or inline display from the S3 provider.
func (s *S3Storage) PresignURL(ctx context.Context, storageKey string, expires time.Duration, inline bool) (string, error) {
	if s.isPublic {
		var publicURL string
		if s.customDomain != "" {
			parts := strings.SplitN(s.customDomain, "/", 2)
			host := parts[0]
			path := "/" + s.bucket + "/" + storageKey
			if len(parts) > 1 {
				subpath := strings.Trim(parts[1], "/")
				path = "/" + subpath + path
			}
			publicURL = "https://" + host + path
		} else {
			baseEndpoint := ""
			if s.client.Options().BaseEndpoint != nil {
				baseEndpoint = *s.client.Options().BaseEndpoint
			}

			if baseEndpoint != "" {
				baseEndpoint = strings.TrimSuffix(baseEndpoint, "/")
				publicURL = fmt.Sprintf("%s/%s/%s", baseEndpoint, s.bucket, storageKey)
			} else {
				region := s.client.Options().Region
				if region == "" {
					region = "us-east-1"
				}
				publicURL = fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, region, storageKey)
			}
		}

		if !inline {
			publicURL += "?download=true"
		}
		return publicURL, nil
	}

	presignClient := s3.NewPresignClient(s.client)

	input := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(storageKey),
	}

	disposition := "attachment"
	if inline {
		disposition = "inline"
	} else {
		// Force the browser to download by overriding the MIME type to application/octet-stream.
		// This is a bulletproof way to prevent browsers from opening downloads in a new tab as previews.
		input.ResponseContentType = aws.String("application/octet-stream")
	}

	// Backblaze B2 supports response header overrides perfectly!
	// We set ResponseContentDisposition so B2 forces a native download prompt for attachments,
	// and serves files inline for previews!
	filename := filepath.Base(storageKey)
	input.ResponseContentDisposition = aws.String(fmt.Sprintf(`%s; filename="%s"`, disposition, filename))

	presignedReq, err := presignClient.PresignGetObject(ctx, input, s3.WithPresignExpires(expires))
	if err != nil {
		return "", fmt.Errorf("s3 storage: failed to presign object %s: %w", storageKey, err)
	}
	
	rawURL := presignedReq.URL
	if s.customDomain != "" {
		u, err := url.Parse(rawURL)
		if err == nil {
			// Split host and optional path (e.g. "customdomain.com/file/darkcopy")
			parts := strings.SplitN(s.customDomain, "/", 2)
			u.Host = parts[0]
			u.Scheme = "https"
			if len(parts) > 1 {
				subpath := strings.Trim(parts[1], "/")
				originalPath := strings.TrimPrefix(u.Path, "/")
				u.Path = "/" + subpath + "/" + originalPath
			}

			// Add standard S3 / CDN override parameters to the query string.
			q := u.Query()
			if !inline {
				q.Set("download_token", fmt.Sprintf("%d", time.Now().UnixNano()))
				
				// Standard S3 / CDN Overrides
				q.Set("response-content-disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
				q.Set("response-content-type", "application/octet-stream")
			} else {
				// Standard S3 / CDN Overrides for Preview
				q.Set("response-content-disposition", fmt.Sprintf(`inline; filename="%s"`, filename))
			}
			u.RawQuery = q.Encode()

			return u.String(), nil
		}
	}

	return rawURL, nil
}
