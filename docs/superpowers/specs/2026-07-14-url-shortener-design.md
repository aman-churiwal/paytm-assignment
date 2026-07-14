# URL Shortener Service — Design Specification

## Overview

A URL shortening service that turns long URLs into short codes and redirects visitors to the original link. Built with Go (Gin), PostgreSQL, and a minimalistic static frontend. The entire stack runs with a single `docker compose up`.

## Requirements (from assignment)

1. **POST /shorten** — accept a URL and return a short code
2. **GET /{code}** — redirect (301) to the original URL
3. Persist mappings in PostgreSQL
4. Support custom aliases; return 404 for unknown codes
5. Short-code generator that won't collide
6. Validate incoming URLs
7. Handle duplicate URLs: always generate a new short code (deliberate — each POST creates a fresh mapping)

## Architecture

**Single Go binary** serving both the REST API and a static HTML/CSS/JS frontend, backed by PostgreSQL.

```
┌──────────────────────────────────────────┐
│           Docker Compose                 │
│                                          │
│  ┌──────────────┐    ┌───────────────┐   │
│  │  Go API      │───▶│  PostgreSQL   │   │
│  │  (Gin)       │    │  (port 5432)  │   │
│  │  port 8080   │    └───────────────┘   │
│  │              │                        │
│  │  /static/*   │◀── Static frontend     │
│  └──────────────┘                        │
└──────────────────────────────────────────┘
```

### Components

1. **Go API (Gin)** — HTTP handlers, URL validation, short-code generation, database operations
2. **PostgreSQL** — persists URL mappings in a `urls` table
3. **Static Frontend** — single-page HTML/CSS/JS served by the Go binary from `/static`

## Short-Code Generation

**Strategy: Base62-encoded PostgreSQL sequence ID**

- PostgreSQL `BIGSERIAL` column provides a monotonically increasing, unique integer ID
- After INSERT, the ID is Base62-encoded into a compact, URL-safe string
- Base62 alphabet: `0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ`
- All characters are URL-safe (no special characters needing encoding)
- At 7 characters, supports 62^7 ≈ 3.5 trillion unique codes
- **No collisions possible** — each database row gets a unique auto-increment ID

**Why this approach:**
- Collision-free by design — no retry loops, no collision checks
- Deterministic — ID → code mapping is a pure function
- Short codes — early IDs produce 1-2 character codes, growing as needed
- Fast — single INSERT + encode, no read-before-write

## API Design

### POST /shorten

**Request:**
```json
{
  "url": "https://example.com/very/long/path",
  "custom_alias": "my-link"   // optional
}
```

**Response (201 Created):**
```json
{
  "short_code": "my-link",
  "short_url": "http://localhost:8080/my-link",
  "original_url": "https://example.com/very/long/path"
}
```

**Validation:**
- `url` is required, must be a valid HTTP/HTTPS URL
- `custom_alias` is optional; if provided, must be 3-30 characters, alphanumeric plus hyphens

**Error responses:**
- `400 Bad Request` — missing or invalid URL, invalid alias format
- `409 Conflict` — custom alias already taken by a different URL

### GET /{code}

- **301 Moved Permanently** → redirects to the original URL
- **404 Not Found** → unknown code

### GET /

- Serves the static frontend (index.html)

## Duplicate URL Handling

**Policy: Always create a new short code.**

Every call to `POST /shorten` (without `custom_alias`) creates a new database row and returns a fresh short code, even if the same URL was shortened before. This is deliberate:
- Simpler insert path — no read-before-write
- Users may want distinct short codes for tracking different campaigns
- Each short code independently represents a mapping

**Custom alias exception:** If a custom alias is requested and that exact alias already maps to the **same URL**, return the existing mapping (idempotent). If it maps to a **different URL**, return `409 Conflict`.

## Database Schema

```sql
CREATE TABLE urls (
    id          BIGSERIAL PRIMARY KEY,
    short_code  VARCHAR(30) NOT NULL UNIQUE,
    original_url TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_urls_short_code ON urls(short_code);
```

- `short_code` has a UNIQUE constraint to prevent conflicts between generated codes and custom aliases
- No unique constraint on `original_url` — same URL can have multiple short codes

## URL Validation

- Must be a valid URL with `http://` or `https://` scheme
- Parsed using Go's `net/url.Parse` — must have a valid scheme and host
- Maximum length: 2048 characters

## Frontend

Minimalistic single-page app:
- Input field for long URL
- Optional field for custom alias
- "Shorten" button
- Displays the shortened URL with a copy-to-clipboard button
- Dark-themed, modern styling with subtle animations

## Docker Compose

Two services:
1. **api** — Go binary, depends on PostgreSQL, exposes port 8080
2. **db** — PostgreSQL 16, persistent volume, health check

The Go service waits for PostgreSQL readiness before starting. Database schema is auto-migrated on startup.

## Testing Strategy

- **Unit tests:** Base62 encoding/decoding, URL validation, handler logic with mocked DB
- **Integration tests:** Full HTTP round-trips against a real PostgreSQL (via Docker Compose or test setup)
- **Edge cases:**
  - Empty URL, invalid URL, URL without scheme
  - Unknown short code → 404
  - Custom alias conflict → 409
  - Same URL shortened twice → two different codes
  - Custom alias for same URL → idempotent return
