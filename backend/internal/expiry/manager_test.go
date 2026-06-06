package expiry

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// --- Mock implementations ---

type mockStore struct {
	mu             sync.Mutex
	expiredPastes  []ExpiredPaste
	expiredFiles   []ExpiredFile
	deletedPastes  []uuid.UUID
	deletedFiles   []uuid.UUID
	deletePasteErr map[uuid.UUID]error
	deleteFileErr  map[uuid.UUID]error
	listPastesErr  error
	listFilesErr   error
}

func newMockStore() *mockStore {
	return &mockStore{
		deletePasteErr: make(map[uuid.UUID]error),
		deleteFileErr:  make(map[uuid.UUID]error),
	}
}

func (s *mockStore) ListExpiredPastes(_ context.Context, _ time.Time, limit int) ([]ExpiredPaste, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listPastesErr != nil {
		return nil, s.listPastesErr
	}
	if limit >= len(s.expiredPastes) {
		return s.expiredPastes, nil
	}
	return s.expiredPastes[:limit], nil
}

func (s *mockStore) DeletePaste(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err, ok := s.deletePasteErr[id]; ok {
		return err
	}
	s.deletedPastes = append(s.deletedPastes, id)
	return nil
}

func (s *mockStore) ListExpiredFiles(_ context.Context, _ time.Time, limit int) ([]ExpiredFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listFilesErr != nil {
		return nil, s.listFilesErr
	}
	if limit >= len(s.expiredFiles) {
		return s.expiredFiles, nil
	}
	return s.expiredFiles[:limit], nil
}

func (s *mockStore) DeleteFile(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err, ok := s.deleteFileErr[id]; ok {
		return err
	}
	s.deletedFiles = append(s.deletedFiles, id)
	return nil
}

type mockFileDeleter struct {
	mu         sync.Mutex
	deletedKeys []string
	deleteErr  map[string]error
}

func newMockFileDeleter() *mockFileDeleter {
	return &mockFileDeleter{
		deleteErr: make(map[string]error),
	}
}

func (d *mockFileDeleter) Delete(_ context.Context, storageKey string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err, ok := d.deleteErr[storageKey]; ok {
		return err
	}
	d.deletedKeys = append(d.deletedKeys, storageKey)
	return nil
}

// --- Tests ---

func TestRunCleanup_DeletesExpiredPastes(t *testing.T) {
	store := newMockStore()
	deleter := newMockFileDeleter()

	paste1 := ExpiredPaste{ID: uuid.New(), Slug: "abc12345"}
	paste2 := ExpiredPaste{ID: uuid.New(), Slug: "def67890"}
	store.expiredPastes = []ExpiredPaste{paste1, paste2}

	mgr := NewManager(store, deleter,
		WithLogger(slog.Default()),
		WithNowFunc(time.Now),
	)

	deleted, err := mgr.RunCleanup(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted != 2 {
		t.Errorf("expected 2 deleted, got %d", deleted)
	}
	if len(store.deletedPastes) != 2 {
		t.Errorf("expected 2 pastes deleted from store, got %d", len(store.deletedPastes))
	}
	if store.deletedPastes[0] != paste1.ID || store.deletedPastes[1] != paste2.ID {
		t.Errorf("deleted paste IDs do not match")
	}
}

func TestRunCleanup_DeletesExpiredFiles(t *testing.T) {
	store := newMockStore()
	deleter := newMockFileDeleter()

	file1 := ExpiredFile{ID: uuid.New(), Slug: "file1234", StorageKey: "uploads/file1234/doc.pdf"}
	file2 := ExpiredFile{ID: uuid.New(), Slug: "file5678", StorageKey: "uploads/file5678/img.png"}
	store.expiredFiles = []ExpiredFile{file1, file2}

	mgr := NewManager(store, deleter,
		WithLogger(slog.Default()),
		WithNowFunc(time.Now),
	)

	deleted, err := mgr.RunCleanup(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted != 2 {
		t.Errorf("expected 2 deleted, got %d", deleted)
	}
	// Verify disk deletion happened.
	if len(deleter.deletedKeys) != 2 {
		t.Errorf("expected 2 files deleted from disk, got %d", len(deleter.deletedKeys))
	}
	if deleter.deletedKeys[0] != file1.StorageKey || deleter.deletedKeys[1] != file2.StorageKey {
		t.Errorf("deleted storage keys do not match")
	}
	// Verify DB deletion happened.
	if len(store.deletedFiles) != 2 {
		t.Errorf("expected 2 files deleted from DB, got %d", len(store.deletedFiles))
	}
}

func TestRunCleanup_ContinuesOnPasteDeleteFailure(t *testing.T) {
	store := newMockStore()
	deleter := newMockFileDeleter()

	paste1 := ExpiredPaste{ID: uuid.New(), Slug: "fail0001"}
	paste2 := ExpiredPaste{ID: uuid.New(), Slug: "pass0002"}
	store.expiredPastes = []ExpiredPaste{paste1, paste2}
	store.deletePasteErr[paste1.ID] = errors.New("db connection lost")

	mgr := NewManager(store, deleter,
		WithLogger(slog.Default()),
		WithNowFunc(time.Now),
	)

	deleted, err := mgr.RunCleanup(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Only paste2 should be deleted successfully.
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}
	if len(store.deletedPastes) != 1 || store.deletedPastes[0] != paste2.ID {
		t.Errorf("expected only paste2 to be deleted")
	}
}

func TestRunCleanup_ContinuesOnFileDiskDeleteFailure(t *testing.T) {
	store := newMockStore()
	deleter := newMockFileDeleter()

	file1 := ExpiredFile{ID: uuid.New(), Slug: "fdisk001", StorageKey: "uploads/fdisk001/a.txt"}
	file2 := ExpiredFile{ID: uuid.New(), Slug: "fdisk002", StorageKey: "uploads/fdisk002/b.txt"}
	store.expiredFiles = []ExpiredFile{file1, file2}
	deleter.deleteErr[file1.StorageKey] = errors.New("permission denied")

	mgr := NewManager(store, deleter,
		WithLogger(slog.Default()),
		WithNowFunc(time.Now),
	)

	deleted, err := mgr.RunCleanup(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Only file2 should be deleted successfully.
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}
	// file1 should not have its DB record deleted since disk delete failed.
	if len(store.deletedFiles) != 1 || store.deletedFiles[0] != file2.ID {
		t.Errorf("expected only file2 to be deleted from DB")
	}
}

func TestRunCleanup_ContinuesOnFileDBDeleteFailure(t *testing.T) {
	store := newMockStore()
	deleter := newMockFileDeleter()

	file1 := ExpiredFile{ID: uuid.New(), Slug: "fdb00001", StorageKey: "uploads/fdb00001/a.txt"}
	file2 := ExpiredFile{ID: uuid.New(), Slug: "fdb00002", StorageKey: "uploads/fdb00002/b.txt"}
	store.expiredFiles = []ExpiredFile{file1, file2}
	store.deleteFileErr[file1.ID] = errors.New("db error")

	mgr := NewManager(store, deleter,
		WithLogger(slog.Default()),
		WithNowFunc(time.Now),
	)

	deleted, err := mgr.RunCleanup(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// file1 disk delete succeeds but DB delete fails, so only file2 counts.
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}
}

func TestRunCleanup_RespectsBatchSizeLimit(t *testing.T) {
	store := newMockStore()
	deleter := newMockFileDeleter()

	// Create 5 expired pastes and 5 expired files, but set batch size to 3.
	for i := 0; i < 5; i++ {
		store.expiredPastes = append(store.expiredPastes, ExpiredPaste{ID: uuid.New(), Slug: "p" + string(rune('a'+i))})
	}
	for i := 0; i < 5; i++ {
		store.expiredFiles = append(store.expiredFiles, ExpiredFile{
			ID:         uuid.New(),
			Slug:       "f" + string(rune('a'+i)),
			StorageKey: "uploads/f" + string(rune('a'+i)) + "/file.txt",
		})
	}

	mgr := NewManager(store, deleter,
		WithBatchSize(3),
		WithLogger(slog.Default()),
		WithNowFunc(time.Now),
	)

	deleted, err := mgr.RunCleanup(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With batch size 3: pastes query gets 3 items, remaining = 0, no files queried.
	if deleted != 3 {
		t.Errorf("expected 3 deleted, got %d", deleted)
	}
	if len(store.deletedPastes) != 3 {
		t.Errorf("expected 3 pastes deleted, got %d", len(store.deletedPastes))
	}
	if len(store.deletedFiles) != 0 {
		t.Errorf("expected 0 files deleted, got %d", len(store.deletedFiles))
	}
}

func TestRunCleanup_BatchSizeSharedBetweenPastesAndFiles(t *testing.T) {
	store := newMockStore()
	deleter := newMockFileDeleter()

	// 2 pastes, 5 files, batch size 4 → should process 2 pastes + 2 files.
	for i := 0; i < 2; i++ {
		store.expiredPastes = append(store.expiredPastes, ExpiredPaste{ID: uuid.New(), Slug: "pa" + string(rune('0'+i))})
	}
	for i := 0; i < 5; i++ {
		store.expiredFiles = append(store.expiredFiles, ExpiredFile{
			ID:         uuid.New(),
			Slug:       "fa" + string(rune('0'+i)),
			StorageKey: "uploads/fa" + string(rune('0'+i)) + "/file.txt",
		})
	}

	mgr := NewManager(store, deleter,
		WithBatchSize(4),
		WithLogger(slog.Default()),
		WithNowFunc(time.Now),
	)

	deleted, err := mgr.RunCleanup(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 2 pastes + 2 files = 4 total (batch size).
	if deleted != 4 {
		t.Errorf("expected 4 deleted, got %d", deleted)
	}
	if len(store.deletedPastes) != 2 {
		t.Errorf("expected 2 pastes deleted, got %d", len(store.deletedPastes))
	}
	if len(store.deletedFiles) != 2 {
		t.Errorf("expected 2 files deleted, got %d", len(store.deletedFiles))
	}
}

func TestStartStop_ViaContextCancellation(t *testing.T) {
	store := newMockStore()
	deleter := newMockFileDeleter()

	// Add one expired paste so we can verify cleanup ran.
	store.expiredPastes = []ExpiredPaste{{ID: uuid.New(), Slug: "start001"}}

	mgr := NewManager(store, deleter,
		WithInterval(10*time.Millisecond), // Very short interval for testing.
		WithLogger(slog.Default()),
		WithNowFunc(time.Now),
	)

	ctx, cancel := context.WithCancel(context.Background())
	mgr.Start(ctx)

	// Wait enough time for at least one tick.
	time.Sleep(50 * time.Millisecond)

	cancel()

	// Give goroutine time to exit.
	time.Sleep(20 * time.Millisecond)

	store.mu.Lock()
	deletedCount := len(store.deletedPastes)
	store.mu.Unlock()

	if deletedCount == 0 {
		t.Error("expected at least one cleanup to have run")
	}
}

func TestRunCleanup_NothingExpired(t *testing.T) {
	store := newMockStore()
	deleter := newMockFileDeleter()

	mgr := NewManager(store, deleter,
		WithLogger(slog.Default()),
		WithNowFunc(time.Now),
	)

	deleted, err := mgr.RunCleanup(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted != 0 {
		t.Errorf("expected 0 deleted, got %d", deleted)
	}
}

func TestRunCleanup_ListPastesError(t *testing.T) {
	store := newMockStore()
	deleter := newMockFileDeleter()
	store.listPastesErr = errors.New("database unavailable")

	mgr := NewManager(store, deleter,
		WithLogger(slog.Default()),
		WithNowFunc(time.Now),
	)

	deleted, err := mgr.RunCleanup(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if deleted != 0 {
		t.Errorf("expected 0 deleted, got %d", deleted)
	}
}

func TestRunCleanup_ListFilesError(t *testing.T) {
	store := newMockStore()
	deleter := newMockFileDeleter()
	store.listFilesErr = errors.New("database unavailable")

	// Add a paste so we get past the paste phase.
	paste1 := ExpiredPaste{ID: uuid.New(), Slug: "lfe00001"}
	store.expiredPastes = []ExpiredPaste{paste1}

	mgr := NewManager(store, deleter,
		WithLogger(slog.Default()),
		WithNowFunc(time.Now),
	)

	deleted, err := mgr.RunCleanup(context.Background())
	if err == nil {
		t.Fatal("expected error from ListExpiredFiles, got nil")
	}
	// The paste should still have been deleted before the file list error.
	if deleted != 1 {
		t.Errorf("expected 1 deleted (paste), got %d", deleted)
	}
}
