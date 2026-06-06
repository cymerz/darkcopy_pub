package settings

import (
	"context"
	"testing"
)

func TestDefaults_AreValid(t *testing.T) {
	if err := Defaults().Validate(); err != nil {
		t.Fatalf("default settings should be valid, got %v", err)
	}
}

func TestValidate_RejectsBadSizes(t *testing.T) {
	s := Defaults()
	s.MaxPasteSizeBytes = 0
	if err := s.Validate(); err != ErrInvalidPasteSize {
		t.Errorf("expected ErrInvalidPasteSize, got %v", err)
	}

	s = Defaults()
	s.MaxFileSizeBytes = MaxFileSizeCeiling + 1
	if err := s.Validate(); err != ErrInvalidFileSize {
		t.Errorf("expected ErrInvalidFileSize, got %v", err)
	}
}

func TestValidate_RejectsEmptyExpiryOptions(t *testing.T) {
	s := Defaults()
	s.PasteExpiryOptions = nil
	if err := s.Validate(); err != ErrNoExpiryOptions {
		t.Errorf("expected ErrNoExpiryOptions, got %v", err)
	}
}

func TestValidate_RejectsBadExpiryOption(t *testing.T) {
	s := Defaults()
	s.FileExpiryOptions = []ExpiryOption{{Label: "", Minutes: 10}}
	if err := s.Validate(); err != ErrInvalidExpiry {
		t.Errorf("expected ErrInvalidExpiry, got %v", err)
	}

	s = Defaults()
	s.FileExpiryOptions = []ExpiryOption{{Label: "bad", Minutes: -5}}
	if err := s.Validate(); err != ErrInvalidExpiry {
		t.Errorf("expected ErrInvalidExpiry for negative minutes, got %v", err)
	}
}

func TestValidate_RejectsBadDailyLimits(t *testing.T) {
	s := Defaults()
	s.MaxPastesPerDayPerIP = -1
	if err := s.Validate(); err != ErrInvalidDailyLimit {
		t.Errorf("expected ErrInvalidDailyLimit, got %v", err)
	}
}

func TestProvider_GetReturnsCopy(t *testing.T) {
	p := NewProvider(Defaults())
	got := p.Get()
	got.PasteExpiryOptions[0].Label = "MUTATED"

	if p.Get().PasteExpiryOptions[0].Label == "MUTATED" {
		t.Error("Provider.Get must return a copy; mutation leaked into provider state")
	}
}

func TestProvider_SetAndAccessors(t *testing.T) {
	p := NewProvider(Defaults())
	s := Defaults()
	s.MaxPasteSizeBytes = 123
	s.MaxFileSizeBytes = 456
	p.Set(s)

	if p.MaxPasteSize() != 123 {
		t.Errorf("expected MaxPasteSize 123, got %d", p.MaxPasteSize())
	}
	if p.MaxFileSize() != 456 {
		t.Errorf("expected MaxFileSize 456, got %d", p.MaxFileSize())
	}
}

// stubStore captures the last saved settings and can simulate a save failure.
type stubStore struct {
	saved   *Settings
	saveErr error
}

func (s *stubStore) Load(_ context.Context) (*Settings, error) { return s.saved, nil }
func (s *stubStore) Save(_ context.Context, v Settings) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	cp := v
	s.saved = &cp
	return nil
}

func TestManager_Update_PersistsAndApplies(t *testing.T) {
	store := &stubStore{}
	p := NewProvider(Defaults())
	m := NewManager(p, store)

	s := Defaults()
	s.MaxPastesPerDayPerIP = 5
	if err := m.Update(context.Background(), s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.saved == nil || store.saved.MaxPastesPerDayPerIP != 5 {
		t.Error("expected settings to be persisted")
	}
	if m.Get().MaxPastesPerDayPerIP != 5 {
		t.Error("expected settings to be applied in provider")
	}
}

func TestManager_Update_RejectsInvalidBeforePersist(t *testing.T) {
	store := &stubStore{}
	p := NewProvider(Defaults())
	m := NewManager(p, store)

	bad := Defaults()
	bad.MaxFileSizeBytes = -1
	if err := m.Update(context.Background(), bad); err == nil {
		t.Fatal("expected validation error")
	}
	if store.saved != nil {
		t.Error("invalid settings must not be persisted")
	}
}
