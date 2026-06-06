package db

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/gthbn/pastebin/internal/admin"
	"github.com/gthbn/pastebin/internal/expiry"
	"github.com/gthbn/pastebin/internal/paste"
)

// PasteRepo implements paste.PasteRepository using pgxpool.
type PasteRepo struct {
	pool *pgxpool.Pool
}

// NewPasteRepo creates a new PasteRepo.
func NewPasteRepo(pool *pgxpool.Pool) *PasteRepo {
	return &PasteRepo{pool: pool}
}

// InsertPaste inserts a new paste into the database.
func (r *PasteRepo) InsertPaste(ctx context.Context, p *paste.Paste) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO pastes (id, slug, title, content, language, visibility, password_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, p.ID, p.Slug, p.Title, p.Content, p.Language, p.Visibility, nilIfEmpty(p.PasswordHash), p.ExpiresAt, p.CreatedAt)
	return err
}

// GetBySlug retrieves a paste by its slug.
func (r *PasteRepo) GetBySlug(ctx context.Context, slug string) (*paste.Paste, error) {
	p := &paste.Paste{}
	var passwordHash *string
	err := r.pool.QueryRow(ctx, `
		SELECT id, slug, title, content, language, visibility, password_hash, expires_at, created_at, views
		FROM pastes WHERE slug = $1
	`, slug).Scan(&p.ID, &p.Slug, &p.Title, &p.Content, &p.Language, &p.Visibility, &passwordHash, &p.ExpiresAt, &p.CreatedAt, &p.Views)
	if err != nil {
		return nil, err
	}
	if passwordHash != nil {
		p.PasswordHash = *passwordHash
	}
	return p, nil
}

// ListPublicRecent returns the most recent public pastes up to the given limit.
func (r *PasteRepo) ListPublicRecent(ctx context.Context, limit int) ([]*paste.PasteSummary, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT slug, title, language, created_at, expires_at
		FROM pastes
		WHERE visibility = 'public'
		  AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []*paste.PasteSummary
	for rows.Next() {
		s := &paste.PasteSummary{}
		if err := rows.Scan(&s.Slug, &s.Title, &s.Language, &s.CreatedAt, &s.ExpiresAt); err != nil {
			return nil, err
		}
		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}

// ListAllPastes returns all pastes (any visibility, including expired) ordered
// by most recent first. Intended for administrative use only.
func (r *PasteRepo) ListAllPastes(ctx context.Context, limit, offset int) ([]*admin.PasteItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT slug, title, language, visibility, (password_hash IS NOT NULL), created_at, expires_at, views
		FROM pastes
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*admin.PasteItem
	for rows.Next() {
		item := &admin.PasteItem{}
		if err := rows.Scan(&item.Slug, &item.Title, &item.Language, &item.Visibility, &item.HasPassword, &item.CreatedAt, &item.ExpiresAt, &item.Views); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// DeletePasteBySlug deletes a paste by its slug. It returns true if a row was
// deleted, false if no paste matched the slug.
func (r *PasteRepo) DeletePasteBySlug(ctx context.Context, slug string) (bool, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM pastes WHERE slug = $1`, slug)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// CountPastes returns the total number of pastes in the database.
func (r *PasteRepo) CountPastes(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM pastes`).Scan(&count)
	return count, err
}

// IncrementViews increments the view count of a paste atomically.
func (r *PasteRepo) IncrementViews(ctx context.Context, slug string) error {
	_, err := r.pool.Exec(ctx, `UPDATE pastes SET views = views + 1 WHERE slug = $1`, slug)
	return err
}

// ListTopPastes returns the top limit pastes by views count.
func (r *PasteRepo) ListTopPastes(ctx context.Context, limit int) ([]*admin.PasteItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT slug, title, language, visibility, (password_hash IS NOT NULL), created_at, expires_at, views
		FROM pastes
		ORDER BY views DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*admin.PasteItem
	for rows.Next() {
		item := &admin.PasteItem{}
		if err := rows.Scan(&item.Slug, &item.Title, &item.Language, &item.Visibility, &item.HasPassword, &item.CreatedAt, &item.ExpiresAt, &item.Views); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}


// FileRepo implements file.FileRepository using pgxpool.
type FileRepo struct {
	pool *pgxpool.Pool
}

// NewFileRepo creates a new FileRepo.
func NewFileRepo(pool *pgxpool.Pool) *FileRepo {
	return &FileRepo{pool: pool}
}

// InsertFile inserts a new file record into the database.
func (r *FileRepo) InsertFile(ctx context.Context, f *paste.FileRecord) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO files (id, slug, filename, mime_type, size_bytes, storage_key, visibility, password_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, f.ID, f.Slug, f.Filename, f.MIMEType, f.SizeBytes, f.StorageKey, f.Visibility, nilIfEmpty(f.PasswordHash), f.ExpiresAt, f.CreatedAt)
	return err
}

// GetBySlug retrieves a file record by its slug.
func (r *FileRepo) GetBySlug(ctx context.Context, slug string) (*paste.FileRecord, error) {
	f := &paste.FileRecord{}
	var passwordHash *string
	err := r.pool.QueryRow(ctx, `
		SELECT id, slug, filename, mime_type, size_bytes, storage_key, visibility, password_hash, expires_at, created_at, downloads
		FROM files WHERE slug = $1
	`, slug).Scan(&f.ID, &f.Slug, &f.Filename, &f.MIMEType, &f.SizeBytes, &f.StorageKey, &f.Visibility, &passwordHash, &f.ExpiresAt, &f.CreatedAt, &f.Downloads)
	if err != nil {
		return nil, err
	}
	if passwordHash != nil {
		f.PasswordHash = *passwordHash
	}
	return f, nil
}

// ListPublicRecent returns the most recent public files up to the given limit.
func (r *FileRepo) ListPublicRecent(ctx context.Context, limit int) ([]*paste.FileSummary, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT slug, filename, mime_type, size_bytes, created_at, expires_at
		FROM files
		WHERE visibility = 'public'
		  AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []*paste.FileSummary
	for rows.Next() {
		s := &paste.FileSummary{}
		if err := rows.Scan(&s.Slug, &s.Filename, &s.MIMEType, &s.SizeBytes, &s.CreatedAt, &s.ExpiresAt); err != nil {
			return nil, err
		}
		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}

// ListAllFiles returns all uploaded files ordered by most recent first.
// Intended for administrative use only.
func (r *FileRepo) ListAllFiles(ctx context.Context, limit, offset int) ([]*admin.FileItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT slug, filename, mime_type, size_bytes, visibility, (password_hash IS NOT NULL), created_at, expires_at, downloads
		FROM files
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*admin.FileItem
	for rows.Next() {
		item := &admin.FileItem{}
		if err := rows.Scan(&item.Slug, &item.Filename, &item.MIMEType, &item.SizeBytes, &item.Visibility, &item.HasPassword, &item.CreatedAt, &item.ExpiresAt, &item.Downloads); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// DeleteFileBySlug deletes a file record by its slug. It returns true if a row
// was deleted, false if no file matched the slug.
func (r *FileRepo) DeleteFileBySlug(ctx context.Context, slug string) (bool, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM files WHERE slug = $1`, slug)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// CountFiles returns the total number of files in the database.
func (r *FileRepo) CountFiles(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM files`).Scan(&count)
	return count, err
}

// IncrementDownloads increments the download count of a file atomically.
func (r *FileRepo) IncrementDownloads(ctx context.Context, slug string) error {
	_, err := r.pool.Exec(ctx, `UPDATE files SET downloads = downloads + 1 WHERE slug = $1`, slug)
	return err
}

// SumFileSizes returns the sum of size_bytes of all files in the database.
func (r *FileRepo) SumFileSizes(ctx context.Context) (int64, error) {
	var total int64
	err := r.pool.QueryRow(ctx, `SELECT COALESCE(SUM(size_bytes), 0) FROM files`).Scan(&total)
	return total, err
}

// ListTopFiles returns the top limit files by downloads count.
func (r *FileRepo) ListTopFiles(ctx context.Context, limit int) ([]*admin.FileItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT slug, filename, mime_type, size_bytes, visibility, (password_hash IS NOT NULL), created_at, expires_at, downloads
		FROM files
		ORDER BY downloads DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*admin.FileItem
	for rows.Next() {
		item := &admin.FileItem{}
		if err := rows.Scan(&item.Slug, &item.Filename, &item.MIMEType, &item.SizeBytes, &item.Visibility, &item.HasPassword, &item.CreatedAt, &item.ExpiresAt, &item.Downloads); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

type ExpiryStore struct {
	pool *pgxpool.Pool
}

// NewExpiryStore creates a new ExpiryStore.
func NewExpiryStore(pool *pgxpool.Pool) *ExpiryStore {
	return &ExpiryStore{pool: pool}
}

// ListExpiredPastes returns up to `limit` pastes with expires_at < now.
func (r *ExpiryStore) ListExpiredPastes(ctx context.Context, now time.Time, limit int) ([]expiry.ExpiredPaste, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, slug FROM pastes
		WHERE expires_at IS NOT NULL AND expires_at < $1
		LIMIT $2
	`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []expiry.ExpiredPaste
	for rows.Next() {
		var item expiry.ExpiredPaste
		if err := rows.Scan(&item.ID, &item.Slug); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// DeletePaste deletes a paste by its ID.
func (r *ExpiryStore) DeletePaste(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM pastes WHERE id = $1`, id)
	return err
}

// ListExpiredFiles returns up to `limit` files with expires_at < now.
func (r *ExpiryStore) ListExpiredFiles(ctx context.Context, now time.Time, limit int) ([]expiry.ExpiredFile, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, slug, storage_key FROM files
		WHERE expires_at IS NOT NULL AND expires_at < $1
		LIMIT $2
	`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []expiry.ExpiredFile
	for rows.Next() {
		var item expiry.ExpiredFile
		if err := rows.Scan(&item.ID, &item.Slug, &item.StorageKey); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// DeleteFile deletes a file record by its ID.
func (r *ExpiryStore) DeleteFile(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM files WHERE id = $1`, id)
	return err
}

// SlugExists checks whether a slug exists in either the pastes or files table.
func SlugExists(pool *pgxpool.Pool) func(ctx context.Context, slug string) (bool, error) {
	return func(ctx context.Context, slug string) (bool, error) {
		var exists bool
		err := pool.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM pastes WHERE slug = $1
				UNION ALL
				SELECT 1 FROM files WHERE slug = $1
			)
		`, slug).Scan(&exists)
		if err != nil {
			return false, err
		}
		return exists, nil
	}
}

// nilIfEmpty returns nil if s is empty, otherwise returns a pointer to s.
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
