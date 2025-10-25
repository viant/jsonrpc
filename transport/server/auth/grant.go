package auth

import (
	"github.com/google/uuid"
	"time"
)

// Grant represents a durable BFF authentication grant held server-side.
// It is referenced by an opaque cookie id (e.g., BFF-Auth-Session) and
// used to rehydrate user authentication without exposing tokens to the client.
type Grant struct {
	// ID is the opaque identifier stored in the httpOnly cookie.
	ID string
	// FamilyID groups rotated grants for logout-all semantics.
	FamilyID string

	// Subject identifies the authenticated principal (e.g., user id or account id).
	Subject string
	// Scopes or roles associated with this grant (optional).
	Scopes []string

	// CreatedAt is when the grant was issued.
	CreatedAt time.Time
	// LastUsedAt is updated on use (for sliding TTL logic).
	LastUsedAt time.Time
	// ExpiresAt is the idle expiration time (sliding TTL).
	ExpiresAt time.Time
	// MaxExpiresAt is the absolute expiration cap.
	MaxExpiresAt time.Time

	// Device binding hints (optional; tolerant matching recommended).
	UAHash string
	IPHint string

	// Arbitrary metadata to support implementers (optional).
	Meta map[string]string
}

// NewGrant creates a new Grant with generated IDs and timestamps.
func NewGrant(subject string) *Grant {
	now := time.Now()
	return &Grant{
		ID:         uuid.New().String(),
		FamilyID:   uuid.New().String(),
		Subject:    subject,
		CreatedAt:  now,
		LastUsedAt: now,
	}
}
