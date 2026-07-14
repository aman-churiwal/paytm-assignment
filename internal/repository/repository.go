package repository

import (
	"errors"
	"url-shortener/internal/model"
)

// ErrNotFound is returned when a short code does not exist.
var ErrNotFound = errors.New("url not found")

// ErrAliasConflict is returned when a custom alias is already taken by a different URL.
var ErrAliasConflict = errors.New("alias already taken by a different url")

// Repository defines the interface for URL persistence operations.
type Repository interface {
	// CreateURL inserts a new URL and returns the mapping with a generated short code.
	CreateURL(originalURL string) (*model.URL, error)

	// CreateURLWithAlias inserts a URL with a custom alias.
	// Returns existing mapping if alias maps to the same URL (idempotent).
	// Returns ErrAliasConflict if alias maps to a different URL.
	CreateURLWithAlias(originalURL, alias string) (*model.URL, error)

	// GetByShortCode looks up a URL by its short code.
	// Returns ErrNotFound if the code does not exist.
	GetByShortCode(shortCode string) (*model.URL, error)
}
