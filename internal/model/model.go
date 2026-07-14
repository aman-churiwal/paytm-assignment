package model

import "time"

// URL represents a shortened URL mapping stored in the database.
type URL struct {
	ID          int64     `json:"id"`
	ShortCode   string    `json:"short_code"`
	OriginalURL string    `json:"original_url"`
	CreatedAt   time.Time `json:"created_at"`
}

// ShortenRequest is the JSON body for POST /shorten.
type ShortenRequest struct {
	URL         string `json:"url" binding:"required"`
	CustomAlias string `json:"custom_alias,omitempty"`
}

// ShortenResponse is the JSON body returned by POST /shorten.
type ShortenResponse struct {
	ShortCode   string `json:"short_code"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}
