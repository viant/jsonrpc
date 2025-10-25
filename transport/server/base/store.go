package base

import "github.com/viant/jsonrpc/internal/collection"

// SessionStore abstracts session persistence.
// Default implementation is in-memory; custom stores (e.g., Redis) can implement this interface.
type SessionStore interface {
	Get(id string) (*Session, bool)
	Put(id string, s *Session)
	Delete(id string)
	Range(func(id string, s *Session) bool)
}

// memorySessionStore is an in-memory store backed by SyncMap.
type memorySessionStore struct {
	m *collection.SyncMap[string, *Session]
}

func (s *memorySessionStore) Get(id string) (*Session, bool) { return s.m.Get(id) }
func (s *memorySessionStore) Put(id string, v *Session)      { s.m.Put(id, v) }
func (s *memorySessionStore) Delete(id string)               { s.m.Delete(id) }
func (s *memorySessionStore) Range(f func(string, *Session) bool) {
	s.m.Range(f)
}

// NewMemorySessionStore creates an in-memory SessionStore.
func NewMemorySessionStore() SessionStore {
	return &memorySessionStore{m: collection.NewSyncMap[string, *Session]()}
}
