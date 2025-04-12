package sse

import (
	"fmt"
	"net/http"
	"net/url"
)

// Locator is a struct that handles the location of session IDs in HTTP requests
type Locator struct{}

// Locate retrieves the session ID from the specified location in the HTTP request
func (l *Locator) Locate(location *Location, request *http.Request) (string, error) {
	if request == nil {
		return "", fmt.Errorf("request was nil")
	}

	switch location.Kind {
	case "header":
		return request.Header.Get(location.Name), nil
	case "query":
		return request.URL.Query().Get(location.Name), nil
	}
	return "", fmt.Errorf("unsupported sessionIdLocation kind: %s for name: %s", location.Kind, location.Name)
}

// Set sets the session ID in the specified location in the HTTP request
func (l *Locator) Set(location *Location, values url.Values, id string) error {
	if values == nil {
		return fmt.Errorf("values were nil")
	}
	switch location.Kind {
	case "query":
		// Clone and modify query parameters properly
		values.Set(location.Name, id) // Use Set instead of Add to avoid duplicates
	default:
		return fmt.Errorf("unsupported sessionIdLocation kind: %s for name: %s", location.Kind, location.Name)
	}
	return nil
}
