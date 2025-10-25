package auth

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrNotFound indicates no grant was found for the given id.
	ErrNotFound = errors.New("auth grant not found")
)

// Store defines the contract for a durable BFF authentication grant store.
// Implementations should be safe for concurrent use and resilient across restarts.
// A Redis-based implementation is recommended for production deployments.
type Store interface {
	// Put inserts or updates a grant. Implementations may enforce TTLs based on grant fields.
	Put(ctx context.Context, g *Grant) error

	// Get retrieves a grant by id. Should return ErrNotFound if missing or expired.
	Get(ctx context.Context, id string) (*Grant, error)

	// Touch updates last-used timestamp and extends idle expiry (sliding TTL) as appropriate.
	Touch(ctx context.Context, id string, at time.Time) error

	// Rotate atomically replaces an existing grant id with a new one.
	// Returns the new id; implementations may keep the old id valid for a short grace window.
	Rotate(ctx context.Context, oldID string, newGrant *Grant) (string, error)

	// Revoke deletes a specific grant id immediately.
	Revoke(ctx context.Context, id string) error

	// RevokeFamily deletes all grants in the same family (logout-all across devices/tabs).
	RevokeFamily(ctx context.Context, familyID string) error
}
