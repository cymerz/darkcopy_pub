// Package main provides a utility script to configure CORS on configured S3 buckets.
package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type s3Config struct {
	name      string
	bucket    string
	region    string
	endpoint  string
	accessKey string
	secretKey string
}

func main() {
	log.Println("Starting S3 CORS configuration script...")

	// 1. Gather all configured S3 providers from environment variables
	var configs []s3Config

	s3ProvidersEnv := os.Getenv("S3_PROVIDERS")
	if s3ProvidersEnv != "" {
		prefixes := strings.Split(s3ProvidersEnv, ",")
		for _, prefix := range prefixes {
			prefix = strings.TrimSpace(strings.ToUpper(prefix))
			if prefix == "" {
				continue
			}
			bucket := os.Getenv("S3_" + prefix + "_BUCKET")
			if bucket == "" {
				continue
			}
			region := envOrDefault("S3_"+prefix+"_REGION", "us-east-1")
			endpoint := os.Getenv("S3_" + prefix + "_ENDPOINT")
			accessKey := os.Getenv("S3_" + prefix + "_ACCESS_KEY")
			secretKey := os.Getenv("S3_" + prefix + "_SECRET_KEY")

			configs = append(configs, s3Config{
				name:      prefix,
				bucket:    bucket,
				region:    region,
				endpoint:  endpoint,
				accessKey: accessKey,
				secretKey: secretKey,
			})
		}
	} else if s3Bucket := os.Getenv("S3_BUCKET"); s3Bucket != "" {
		s3Region := envOrDefault("S3_REGION", "us-east-1")
		s3Endpoint := os.Getenv("S3_ENDPOINT")
		s3AccessKey := os.Getenv("S3_ACCESS_KEY")
		s3SecretKey := os.Getenv("S3_SECRET_KEY")

		configs = append(configs, s3Config{
			name:      "PRIMARY_S3",
			bucket:    s3Bucket,
			region:    s3Region,
			endpoint:  s3Endpoint,
			accessKey: s3AccessKey,
			secretKey: s3SecretKey,
		})
	}

	if len(configs) == 0 {
		log.Println("No S3 storage providers detected in environment configuration. Exiting.")
		os.Exit(0)
	}

	// Determine the allowed origins. Fallback to "*" for out-of-the-box compatibility
	allowedOrigin := "*"
	if appURL := os.Getenv("APP_URL"); appURL != "" {
		allowedOrigin = strings.TrimRight(appURL, "/")
	}

	log.Printf("Setting CORS origin to: %q", allowedOrigin)

	ctx := context.Background()

	// 2. Configure CORS for each S3 provider
	for _, cfg := range configs {
		log.Printf("[%s] Configuring CORS on bucket %q at %q...", cfg.name, cfg.bucket, cfg.endpoint)

		s3Client, err := createS3Client(ctx, cfg)
		if err != nil {
			log.Printf("[%s] ERROR: Failed to initialize client: %v", cfg.name, err)
			continue
		}

		corsRule := types.CORSRule{
			AllowedHeaders: []string{"*"},
			AllowedMethods: []string{"GET", "PUT", "POST", "HEAD"},
			AllowedOrigins: []string{allowedOrigin},
			ExposeHeaders:  []string{"ETag"},
			MaxAgeSeconds:  aws.Int32(3600),
		}

		_, err = s3Client.PutBucketCors(ctx, &s3.PutBucketCorsInput{
			Bucket: aws.String(cfg.bucket),
			CORSConfiguration: &types.CORSConfiguration{
				CORSRules: []types.CORSRule{corsRule},
			},
		})
		if err != nil {
			log.Printf("[%s] ERROR: Failed to apply CORS policy: %v", cfg.name, err)
			continue
		}

		log.Printf("[%s] SUCCESS: CORS policy successfully applied to bucket %q!", cfg.name, cfg.bucket)
	}

	log.Println("S3 CORS configuration script completed.")
}

func createS3Client(ctx context.Context, cfg s3Config) (*s3.Client, error) {
	opts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.region),
	}

	if cfg.accessKey != "" && cfg.secretKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.accessKey, cfg.secretKey, ""),
		))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}

	// Standardize endpoint format (must start with http:// or https://)
	var endpointURL string
	if cfg.endpoint != "" {
		endpointURL = cfg.endpoint
		if !strings.HasPrefix(endpointURL, "http://") && !strings.HasPrefix(endpointURL, "https://") {
			endpointURL = "https://" + endpointURL
		}
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if endpointURL != "" {
			o.BaseEndpoint = aws.String(endpointURL)
		}
		// Set virtual-host style routing to false for S3-compatible providers (like MinIO/B2/R2)
		o.UsePathStyle = true
	})

	return s3Client, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
