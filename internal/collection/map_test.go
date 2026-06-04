package collection

import "testing"

func TestSyncMapRangeAllowsMutationFromCallback(t *testing.T) {
	store := NewSyncMap[string, int]()
	store.Put("a", 1)
	store.Put("b", 2)

	var visited []string
	store.Range(func(key string, value int) bool {
		visited = append(visited, key)
		if key == "a" {
			store.Delete("b")
			store.Put("c", 3)
		}
		return true
	})

	if len(visited) != 2 {
		t.Fatalf("expected snapshot iteration over 2 original entries, got %d (%v)", len(visited), visited)
	}

	if _, ok := store.Get("b"); ok {
		t.Fatalf("expected key b to be deleted during callback")
	}

	if value, ok := store.Get("c"); !ok || value != 3 {
		t.Fatalf("expected key c to be added during callback, got ok=%v value=%d", ok, value)
	}
}
