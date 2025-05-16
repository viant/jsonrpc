package session

// Location represents the location of the sessionId
type Location struct {
	Name string
	Kind string
}

// NewLocation creates a new sessionIdLocation
func NewLocation(name, kind string) *Location {
	return &Location{
		Name: name,
		Kind: kind,
	}
}

// NewHeaderLocation creates a new sessionIdLocation for header
func NewHeaderLocation(name string) *Location {
	// Header sessionIdLocation
	return &Location{
		Name: name,
		Kind: "header",
	}
}

// NewQueryLocation creates a new sessionIdLocation for query
func NewQueryLocation(name string) *Location {
	// Query sessionIdLocation
	return &Location{
		Name: name,
		Kind: "query",
	}
}
