package auth

import (
	"context"
	"sync"
	"time"
)

// MemoryStore is an in-memory AuthStore for development and tests.
// It supports sliding idle TTL and absolute max TTL semantics.
type MemoryStore struct {
	mux         sync.RWMutex
	byID        map[string]*Grant
	byFamily    map[string]map[string]struct{}
	idleTTL     time.Duration
	maxTTL      time.Duration
	rotateGrace time.Duration
}

// NewMemoryStore creates a MemoryStore with given TTL settings.
func NewMemoryStore(idleTTL, maxTTL, rotateGrace time.Duration) *MemoryStore {
	return &MemoryStore{
		byID:        map[string]*Grant{},
		byFamily:    map[string]map[string]struct{}{},
		idleTTL:     idleTTL,
		maxTTL:      maxTTL,
		rotateGrace: rotateGrace,
	}
}

func (s *MemoryStore) Put(_ context.Context, g *Grant) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	now := time.Now()
	if g.CreatedAt.IsZero() {
		g.CreatedAt = now
	}
	if g.LastUsedAt.IsZero() {
		g.LastUsedAt = now
	}
	if g.ExpiresAt.IsZero() && s.idleTTL > 0 {
		g.ExpiresAt = now.Add(s.idleTTL)
	}
	if g.MaxExpiresAt.IsZero() && s.maxTTL > 0 {
		g.MaxExpiresAt = now.Add(s.maxTTL)
	}
	s.byID[g.ID] = cloneGrant(g)
	fam := s.byFamily[g.FamilyID]
	if fam == nil {
		fam = map[string]struct{}{}
		s.byFamily[g.FamilyID] = fam
	}
	fam[g.ID] = struct{}{}
	return nil
}

func (s *MemoryStore) Get(_ context.Context, id string) (*Grant, error) {
	s.mux.RLock()
	g, ok := s.byID[id]
	s.mux.RUnlock()
	if !ok {
		return nil, ErrNotFound
	}
	now := time.Now()
	if (!g.ExpiresAt.IsZero() && now.After(g.ExpiresAt)) || (!g.MaxExpiresAt.IsZero() && now.After(g.MaxExpiresAt)) {
		_ = s.Revoke(context.Background(), id)
		return nil, ErrNotFound
	}
	return cloneGrant(g), nil
}

func (s *MemoryStore) Touch(_ context.Context, id string, at time.Time) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	g, ok := s.byID[id]
	if !ok {
		return ErrNotFound
	}
	g.LastUsedAt = at
	if s.idleTTL > 0 {
		newExp := at.Add(s.idleTTL)
		if !g.MaxExpiresAt.IsZero() && newExp.After(g.MaxExpiresAt) {
			newExp = g.MaxExpiresAt
		}
		g.ExpiresAt = newExp
	}
	return nil
}

func (s *MemoryStore) Rotate(_ context.Context, oldID string, newGrant *Grant) (string, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	old, ok := s.byID[oldID]
	if !ok {
		return "", ErrNotFound
	}
	// prepare new grant in same family
	now := time.Now()
	ng := *newGrant
	if ng.ID == "" {
		ng.ID = uuid4()
	}
	ng.FamilyID = old.FamilyID
	if ng.CreatedAt.IsZero() {
		ng.CreatedAt = now
	}
	if ng.LastUsedAt.IsZero() {
		ng.LastUsedAt = now
	}
	if ng.ExpiresAt.IsZero() && s.idleTTL > 0 {
		ng.ExpiresAt = now.Add(s.idleTTL)
	}
	if ng.MaxExpiresAt.IsZero() && s.maxTTL > 0 {
		ng.MaxExpiresAt = now.Add(s.maxTTL)
	}
	s.byID[ng.ID] = &ng
	fam := s.byFamily[ng.FamilyID]
	if fam == nil {
		fam = map[string]struct{}{}
		s.byFamily[ng.FamilyID] = fam
	}
	fam[ng.ID] = struct{}{}
	// set grace on old by extending ExpiresAt briefly
	if s.rotateGrace > 0 {
		old.ExpiresAt = now.Add(s.rotateGrace)
	}
	return ng.ID, nil
}

func (s *MemoryStore) Revoke(_ context.Context, id string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	g, ok := s.byID[id]
	if !ok {
		return ErrNotFound
	}
	delete(s.byID, id)
	if fam := s.byFamily[g.FamilyID]; fam != nil {
		delete(fam, id)
		if len(fam) == 0 {
			delete(s.byFamily, g.FamilyID)
		}
	}
	return nil
}

func (s *MemoryStore) RevokeFamily(_ context.Context, familyID string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	fam := s.byFamily[familyID]
	if fam == nil {
		return nil
	}
	for id := range fam {
		delete(s.byID, id)
	}
	delete(s.byFamily, familyID)
	return nil
}

func cloneGrant(g *Grant) *Grant {
	if g == nil {
		return nil
	}
	dup := *g
	if g.Scopes != nil {
		dup.Scopes = append([]string(nil), g.Scopes...)
	}
	if g.Meta != nil {
		dup.Meta = map[string]string{}
		for k, v := range g.Meta {
			dup.Meta[k] = v
		}
	}
	return &dup
}

// uuid4 provides a simple UUID for in-memory rotation when NewGrant isn't used.
func uuid4() string {
	return NewGrant("").ID
}
