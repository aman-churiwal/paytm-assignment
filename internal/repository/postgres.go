package repository

import (
	"database/sql"
	"errors"
	"fmt"

	"url-shortener/internal/model"
	"url-shortener/internal/shortcode"
)

// PostgresRepository implements Repository backed by PostgreSQL.
type PostgresRepository struct {
	db *sql.DB
}

// NewPostgresRepository creates a new PostgresRepository.
func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CreateURL(originalURL string) (*model.URL, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get next ID from sequence before inserting, so we can compute the short code.
	var id int64
	err = tx.QueryRow("SELECT nextval('urls_id_seq')").Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("get next sequence value: %w", err)
	}

	code := shortcode.Encode(id)

	var u model.URL
	err = tx.QueryRow(
		`INSERT INTO urls (id, short_code, original_url)
		 VALUES ($1, $2, $3)
		 RETURNING id, short_code, original_url, created_at`,
		id, code, originalURL,
	).Scan(&u.ID, &u.ShortCode, &u.OriginalURL, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert url: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}
	return &u, nil
}

func (r *PostgresRepository) CreateURLWithAlias(originalURL, alias string) (*model.URL, error) {
	// Check if alias already exists.
	var existing model.URL
	err := r.db.QueryRow(
		"SELECT id, short_code, original_url, created_at FROM urls WHERE short_code = $1",
		alias,
	).Scan(&existing.ID, &existing.ShortCode, &existing.OriginalURL, &existing.CreatedAt)

	if err == nil {
		// Alias exists — same URL is idempotent, different URL is a conflict.
		if existing.OriginalURL == originalURL {
			return &existing, nil
		}
		return nil, ErrAliasConflict
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("check existing alias: %w", err)
	}

	// Alias is free — insert.
	var u model.URL
	err = r.db.QueryRow(
		`INSERT INTO urls (short_code, original_url)
		 VALUES ($1, $2)
		 RETURNING id, short_code, original_url, created_at`,
		alias, originalURL,
	).Scan(&u.ID, &u.ShortCode, &u.OriginalURL, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert url with alias: %w", err)
	}
	return &u, nil
}

func (r *PostgresRepository) GetByShortCode(shortCode string) (*model.URL, error) {
	var u model.URL
	err := r.db.QueryRow(
		"SELECT id, short_code, original_url, created_at FROM urls WHERE short_code = $1",
		shortCode,
	).Scan(&u.ID, &u.ShortCode, &u.OriginalURL, &u.CreatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get url by short code: %w", err)
	}
	return &u, nil
}
