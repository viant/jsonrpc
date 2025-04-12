package pointer

func Ref[T any](t T) *T {
	return &t
}

func Deref[T any](t *T) T {
	if t == nil {
		var zero T
		return zero
	}
	return *t
}
