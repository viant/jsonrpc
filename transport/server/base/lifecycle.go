package base

// RemovalPolicy determines when a session should be removed from the session store.
type RemovalPolicy int

const (
	// RemovalOnDisconnect removes session as soon as streaming connection closes.
	// Useful for strict cleanup behavior.
	RemovalOnDisconnect RemovalPolicy = iota
	// RemovalAfterGrace keeps session for a grace period to allow quick reconnects.
	RemovalAfterGrace
	// RemovalAfterIdle removes session after it has been idle for a configured TTL.
	RemovalAfterIdle
	// RemovalManual leaves removal entirely to explicit DELETE or external cleanup.
	RemovalManual
)
