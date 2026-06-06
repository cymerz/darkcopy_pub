// Package access provides the access controller interface and types.
package access

import "context"

// AccessResult represents the outcome of an access check.
type AccessResult int

const (
	AccessGranted          AccessResult = iota
	AccessDenied
	AccessRequiresPassword
	AccessRateLimited
)

// AccessController defines the interface for managing resource access control.
type AccessController interface {
	CheckAccess(ctx context.Context, resourceID string, password string) (AccessResult, error)
	RecordFailedAttempt(ctx context.Context, ip string, resourceID string) error
	IsRateLimited(ctx context.Context, ip string, resourceID string) (bool, error)
}
