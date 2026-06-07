// Package main is the entry point for the pastebin server.
package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"
	"github.com/gthbn/pastebin/internal/access"
	"github.com/gthbn/pastebin/internal/admin"
	"github.com/gthbn/pastebin/internal/db"
	"github.com/gthbn/pastebin/internal/expiry"
	"github.com/gthbn/pastebin/internal/file"
	"github.com/gthbn/pastebin/internal/handler"
	"github.com/gthbn/pastebin/internal/highlight"
	"github.com/gthbn/pastebin/internal/paste"
	"github.com/gthbn/pastebin/internal/quota"
	"github.com/gthbn/pastebin/internal/report"
	"github.com/gthbn/pastebin/internal/settings"
	"github.com/gthbn/pastebin/internal/urlgen"
)

// accessControllerAdapter adapts *access.Controller to the handler.AccessController interface.
// This is needed because the handler package defines its own AccessResult type.
type accessControllerAdapter struct {
	ctl *access.Controller
}

func (a *accessControllerAdapter) CheckAccess(ctx context.Context, resourceID, password string) (handler.AccessResult, error) {
	result, err := a.ctl.CheckAccess(ctx, resourceID, password)
	if err != nil {
		return handler.AccessDenied, err
	}
	if result == access.AccessGranted {
		return handler.AccessGranted, nil
	}
	return handler.AccessDenied, nil
}

func (a *accessControllerAdapter) RecordFailedAttempt(ctx context.Context, ip, resourceID string) error {
	return a.ctl.RecordFailedAttempt(ctx, ip, resourceID)
}

func (a *accessControllerAdapter) IsRateLimited(ctx context.Context, ip, resourceID string) (bool, error) {
	return a.ctl.IsRateLimited(ctx, ip, resourceID)
}

func (a *accessControllerAdapter) ResetRateLimit(ctx context.Context, ip, resourceSlug string) {
	a.ctl.ResetRateLimit(ctx, ip, resourceSlug)
}

func main() {
	logger := slog.Default()

	// Read configuration from environment variables.
	databaseURL := envOrDefault("DATABASE_URL", "postgres://localhost:5432/darkcopy?sslmode=disable")
	port := envOrDefault("PORT", "8080")
	uploadDir := envOrDefault("UPLOAD_DIR", "./uploads")
	adminToken := os.Getenv("ADMIN_TOKEN")
	redisURL := os.Getenv("REDIS_URL")

	// Cleanup interval for the expiry sweep (Go duration, e.g. "5m", "1h").
	// Defaults to the manager's built-in interval when unset or invalid.
	cleanupInterval := expiry.DefaultInterval
	if v := os.Getenv("CLEANUP_INTERVAL"); v != "" {
		if d, perr := time.ParseDuration(v); perr == nil && d > 0 {
			cleanupInterval = d
		} else {
			logger.Warn("invalid CLEANUP_INTERVAL, using default", "value", v, "default", expiry.DefaultInterval)
		}
	}

	// Create a root context with cancel for graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize database connection.
	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Run database migrations.
	if err := db.RunMigrations(ctx, pool); err != nil {
		logger.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	// Initialize Redis client if configured.
	var rdb *redis.Client
	if redisURL != "" {
		opt, err := redis.ParseURL(redisURL)
		if err != nil {
			logger.Error("invalid REDIS_URL", "error", err)
			os.Exit(1)
		}
		rdb = redis.NewClient(opt)

		// Verify connection
		pingCtx, pingCancel := context.WithTimeout(ctx, 3*time.Second)
		if err := rdb.Ping(pingCtx).Err(); err != nil {
			logger.Error("failed to connect to Redis, disabling Redis integration", "error", err)
			rdb = nil
		} else {
			logger.Info("connected to Redis successfully")
		}
		pingCancel()
	}

	// Initialize repositories.
	pasteRepo := db.NewPasteRepo(pool)
	fileRepo := db.NewFileRepo(pool)
	expiryStore := db.NewExpiryStore(pool)
	if rdb != nil {
		pasteRepo = pasteRepo.WithRedis(rdb)
		fileRepo = fileRepo.WithRedis(rdb)
		expiryStore = expiryStore.WithRedis(rdb)
	}
	settingsRepo := db.NewSettingsRepo(pool)
	reportRepo := db.NewReportRepo(pool)

	// Initialize runtime settings: start from defaults, then load any persisted
	// overrides from the database.
	currentSettings := settings.Defaults()
	if persisted, lerr := settingsRepo.Load(ctx); lerr != nil {
		logger.Warn("failed to load settings, using defaults", "error", lerr)
	} else if persisted != nil {
		if verr := persisted.Validate(); verr != nil {
			logger.Warn("persisted settings invalid, using defaults", "error", verr)
		} else {
			currentSettings = *persisted
		}
	}
	settingsProvider := settings.NewProvider(currentSettings)
	settingsMgr := settings.NewManager(settingsProvider, settingsRepo)

	// Initialize Quota Counter.
	var dailyQuota handler.DailyQuota = quota.NewCounter()
	var dailySizeQuota handler.DailySizeQuota = quota.NewSizeCounter()
	if rdb != nil {
		dailyQuota = quota.NewRedisCounter(rdb)
		dailySizeQuota = quota.NewRedisSizeCounter(rdb)
	}

	// Initialize services.
	var rateLimitStore access.RateLimitStore
	if rdb != nil {
		rateLimitStore = access.NewRedisRateLimitStore(rdb)
	}
	accessCtl := access.NewController(rateLimitStore)
	highlighter := highlight.NewChromaHighlighter("")

	// Create a SlugExistsFunc that queries both pastes and files tables.
	slugExistsFunc := db.SlugExists(pool)
	urlGen := urlgen.NewGenerator(slugExistsFunc)

	// Initialize file storage (S3 with Local fallback).
	s3ProvidersEnv := os.Getenv("S3_PROVIDERS")
	var fileStorage file.FileStorage

	if s3ProvidersEnv != "" {
		// Split provider prefixes (e.g. S3_PROVIDERS=FILEBASE,CLOUDFLARE)
		prefixes := strings.Split(s3ProvidersEnv, ",")
		var s3Storages []file.FileStorage
		var activePrefixes []string

		for _, prefix := range prefixes {
			prefix = strings.TrimSpace(strings.ToUpper(prefix))
			if prefix == "" {
				continue
			}

			// Load provider specific variables (e.g. S3_FILEBASE_BUCKET)
			bucket := os.Getenv("S3_" + prefix + "_BUCKET")
			if bucket == "" {
				logger.Warn("S3 provider bucket not configured, skipping", "prefix", prefix)
				continue
			}

			region := envOrDefault("S3_"+prefix+"_REGION", "us-east-1")
			endpoint := os.Getenv("S3_" + prefix + "_ENDPOINT")
			accessKey := os.Getenv("S3_" + prefix + "_ACCESS_KEY")
			secretKey := os.Getenv("S3_" + prefix + "_SECRET_KEY")

			logger.Info("initializing sharded S3 provider", "prefix", prefix, "bucket", bucket, "endpoint", endpoint)
			storage, err := file.NewS3Storage(bucket, region, endpoint, accessKey, secretKey)
			if err != nil {
				logger.Error("failed to initialize sharded S3 provider", "prefix", prefix, "error", err)
				os.Exit(1)
			}
			s3Storages = append(s3Storages, storage)
			activePrefixes = append(activePrefixes, prefix)

			customDomain := os.Getenv("S3_" + prefix + "_CUSTOM_DOMAIN")
			if customDomain != "" {
				storage.SetCustomDomain(customDomain)
				logger.Info("configured custom download domain for sharded S3 provider", "prefix", prefix, "domain", customDomain)
			}

			if envOrBool("S3_" + prefix + "_IS_PUBLIC") {
				storage.SetIsPublic(true)
				logger.Info("configured sharded S3 provider as PUBLIC", "prefix", prefix)
			}
		}

		if len(s3Storages) == 0 {
			logger.Error("S3_PROVIDERS was specified but no providers were successfully configured")
			os.Exit(1)
		}

		logger.Info("initializing Multi-S3 sharded storage", "count", len(s3Storages))
		fileStorage = file.NewMultiS3Storage(s3Storages, activePrefixes)
	} else if s3Bucket := os.Getenv("S3_BUCKET"); s3Bucket != "" {
		s3Region := envOrDefault("S3_REGION", "us-east-1")
		s3Endpoint := os.Getenv("S3_ENDPOINT")
		s3AccessKey := os.Getenv("S3_ACCESS_KEY")
		s3SecretKey := os.Getenv("S3_SECRET_KEY")

		logger.Info("initializing S3 file storage", "bucket", s3Bucket, "endpoint", s3Endpoint, "region", s3Region)
		storage, err := file.NewS3Storage(s3Bucket, s3Region, s3Endpoint, s3AccessKey, s3SecretKey)
		if err != nil {
			logger.Error("failed to initialize S3 storage", "error", err)
			os.Exit(1)
		}
		if s3CustomDomain := os.Getenv("S3_CUSTOM_DOMAIN"); s3CustomDomain != "" {
			storage.SetCustomDomain(s3CustomDomain)
			logger.Info("configured custom download domain for S3", "domain", s3CustomDomain)
		}
		if envOrBool("S3_IS_PUBLIC") {
			storage.SetIsPublic(true)
			logger.Info("configured S3 file storage as PUBLIC")
		}
		fileStorage = storage
	} else {
		logger.Info("initializing Local file storage", "directory", uploadDir)
		fileStorage = file.NewLocalStorage(uploadDir)
	}

	// Initialize business services with runtime-configurable size limits.
	pasteSvc := paste.NewService(pasteRepo, urlGen, accessCtl)
	pasteSvc.SetMaxContentSizeFunc(settingsProvider.MaxPasteSize)
	fileSvc := file.NewService(fileRepo, fileStorage, urlGen)
	fileSvc.SetMaxFileSizeFunc(settingsProvider.MaxFileSize)

	// Initialize distributed locker for expiry manager if Redis is active.
	var locker expiry.Locker
	if rdb != nil {
		locker = expiry.NewRedisLocker(rdb)
	}

	// Initialize expiry manager.
	expiryMgr := expiry.NewManager(
		expiryStore,
		fileStorage,
		expiry.WithLogger(logger),
		expiry.WithInterval(cleanupInterval),
		expiry.WithLocker(locker),
	)

	if rdb != nil {
		// Flush views/downloads from Redis to PostgreSQL every 10 seconds
		db.StartFlusher(ctx, rdb, pool, 10*time.Second, logger)
		logger.Info("started views/downloads flusher background task")
	}

	// The admin service can trigger an on-demand purge via the expiry manager.
	adminSvc := admin.NewService(pasteRepo, fileRepo, fileStorage, expiryMgr)

	// Report service for abuse/content reports.
	reportSvc := report.NewService(reportRepo)

	// Initialize HTTP handlers.
	// Wrap the access controller in an adapter to satisfy the handler's interface.
	handlerAccessCtl := &accessControllerAdapter{ctl: accessCtl}
	pasteHandler := handler.NewPasteHandler(pasteSvc, highlighter, handlerAccessCtl, fileSvc)
	pasteHandler.SetSettings(settingsProvider)
	pasteHandler.SetQuota(dailyQuota)
	fileHandler := handler.NewFileHandler(fileSvc, handlerAccessCtl)
	fileHandler.SetSettings(settingsProvider)
	fileHandler.SetQuota(dailyQuota)
	fileHandler.SetSizeQuota(dailySizeQuota)
	adminHandler := handler.NewAdminHandler(adminSvc, settingsMgr, reportSvc, adminToken)
	reportHandler := handler.NewReportHandler(reportSvc)
	// Limit reports to 20 per IP per day to curb spam.
	reportHandler.SetQuota(dailyQuota, 20)

	// Setup chi router.
	r := chi.NewRouter()
	r.Use(RealIPMiddleware)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Register routes.
	handler.RegisterPasteRoutes(r, pasteHandler)
	handler.RegisterFileRoutes(r, fileHandler)
	handler.RegisterReportRoutes(r, reportHandler)
	handler.RegisterAdminRoutes(r, adminHandler)
	if adminToken == "" {
		logger.Warn("ADMIN_TOKEN not set — admin API is disabled")
	} else {
		logger.Info("admin API enabled")
	}

	// Serve static files.
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	// Start ExpiryManager background goroutine.
	expiryMgr.Start(ctx)

	// Start HTTP server.
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// Handle graceful shutdown on SIGINT/SIGTERM.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		logger.Info("shutting down server...")
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("server shutdown error", "error", err)
		}
	}()

	logger.Info("server starting", "port", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}

	logger.Info("server stopped")
}

// envOrDefault returns the value of the environment variable named by key,
// or defaultVal if the variable is not set or empty.
func envOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// envOrBool returns the boolean value of the environment variable named by key,
// or false if the variable is not set, empty, or not a truthy value (true, 1, yes, on).
func envOrBool(key string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return v == "true" || v == "1" || v == "yes" || v == "on"
}

// RealIPMiddleware is a custom, highly robust middleware to resolve the real
// client IP address when behind multiple proxies (Cloudflare -> aaPanel OpenResty -> Next.js Proxy -> Go).
func RealIPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := ""

		// 1. Check Cloudflare-specific header
		if cfip := r.Header.Get("CF-Connecting-IP"); cfip != "" {
			clientIP = cfip
		} else if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// 2. Check X-Forwarded-For header and get the very first client IP
			ips := strings.Split(xff, ",")
			if firstIP := strings.TrimSpace(ips[0]); firstIP != "" {
				clientIP = firstIP
			}
		} else if xri := r.Header.Get("X-Real-IP"); xri != "" {
			// 3. Check X-Real-IP fallback
			clientIP = xri
		}

		// If a real client IP was successfully extracted, override r.RemoteAddr
		if clientIP != "" {
			if _, port, err := net.SplitHostPort(r.RemoteAddr); err == nil {
				r.RemoteAddr = net.JoinHostPort(clientIP, port)
			} else {
				r.RemoteAddr = clientIP
			}
		}

		next.ServeHTTP(w, r)
	})
}
