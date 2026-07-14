# URL Shortener — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a URL shortener service (Go/Gin + PostgreSQL + static frontend) that runs with a single `docker compose up`.

**Architecture:** Single Go binary serving REST API and an inline static frontend, backed by PostgreSQL. Short codes generated via Base62 encoding of auto-increment IDs. Docker Compose orchestrates both services.

**Tech Stack:** Go 1.22, Gin, PostgreSQL 16, database/sql + lib/pq, vanilla HTML/CSS/JS (inlined in a single HTML file)

## Global Constraints

- Go 1.22+, PostgreSQL 16
- No ORM — `database/sql` with `lib/pq` driver
- Module name: `url-shortener`
- Base62 alphabet: `0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ`
- Duplicate URLs always get a new short code
- Custom alias conflict: same URL → return existing (idempotent); different URL → 409
- Frontend: single `index.html` with inlined CSS/JS, dark theme, modern design
- Single `docker compose up` starts everything
- Reserved aliases: `shorten`, `static`, `health`, `api`

## File Structure

```
d:\Projects\paytm-assignment\
├── cmd/
│   └── server/
│       └── main.go                  # Entry point, DB connection, migrations, router
├── internal/
│   ├── handler/
│   │   ├── handler.go               # HTTP handlers (Shorten, Redirect)
│   │   └── handler_test.go          # Handler tests with mock repository
│   ├── model/
│   │   └── model.go                 # URL struct, request/response types
│   ├── repository/
│   │   ├── repository.go            # Repository interface + sentinel errors
│   │   └── postgres.go              # PostgreSQL implementation
│   ├── shortcode/
│   │   ├── base62.go                # Base62 encode/decode
│   │   └── base62_test.go           # Base62 tests (collisions, URL-safety, round-trip)
│   └── validator/
│       ├── validator.go             # URL and alias validation
│       └── validator_test.go        # Validator tests
├── static/
│   └── index.html                   # Frontend (CSS/JS inlined)
├── Dockerfile                       # Multi-stage build
├── docker-compose.yml               # PostgreSQL + API services
├── go.mod
├── README.md
└── .gitignore
```

---

### Task 1: Project Scaffolding + Base62 Encoding

**Files:**
- Create: `go.mod`
- Create: `internal/shortcode/base62.go`
- Create: `internal/shortcode/base62_test.go`
- Create: `.gitignore`

**Interfaces:**
- Produces: `shortcode.Encode(id int64) string`, `shortcode.Decode(code string) (int64, error)`

- [ ] **Step 1: Initialize Go module and .gitignore**

```bash
cd d:\Projects\paytm-assignment
go mod init url-shortener
```

Create `.gitignore`:
```gitignore
# Binaries
*.exe
*.exe~
*.dll
*.so
*.dylib
/server

# Test binary
*.test

# Go workspace
go.work

# IDE
.idea/
.vscode/
*.swp

# OS
.DS_Store
Thumbs.db

# Environment
.env
```

- [ ] **Step 2: Write Base62 tests**

Create `internal/shortcode/base62_test.go`:

```go
package shortcode

import (
	"testing"
)

func TestEncodeKnownValues(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{9, "9"},
		{10, "a"},
		{35, "z"},
		{36, "A"},
		{61, "Z"},
		{62, "10"},
		{63, "11"},
		{3843, "ZZ"},
		{3844, "100"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := Encode(tt.input)
			if result != tt.expected {
				t.Errorf("Encode(%d) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDecodeKnownValues(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"0", 0},
		{"1", 1},
		{"Z", 61},
		{"10", 62},
		{"ZZ", 3843},
		{"100", 3844},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := Decode(tt.input)
			if err != nil {
				t.Fatalf("Decode(%q) unexpected error: %v", tt.input, err)
			}
			if result != tt.expected {
				t.Errorf("Decode(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	for i := int64(0); i < 100000; i++ {
		encoded := Encode(i)
		decoded, err := Decode(encoded)
		if err != nil {
			t.Fatalf("Decode(Encode(%d)) error: %v", i, err)
		}
		if decoded != i {
			t.Fatalf("Decode(Encode(%d)) = %d", i, decoded)
		}
	}
}

func TestDecodeInvalidCharacter(t *testing.T) {
	_, err := Decode("abc!")
	if err == nil {
		t.Error("expected error for invalid character, got nil")
	}
}

func TestEncodeOutputIsURLSafe(t *testing.T) {
	for i := int64(0); i < 10000; i++ {
		code := Encode(i)
		for _, c := range code {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
				t.Fatalf("Encode(%d) = %q contains non-URL-safe char %q", i, code, c)
			}
		}
	}
}

func TestEncodeNoDuplicates(t *testing.T) {
	seen := make(map[string]int64)
	for i := int64(0); i < 100000; i++ {
		code := Encode(i)
		if prev, ok := seen[code]; ok {
			t.Fatalf("collision: Encode(%d) = Encode(%d) = %q", i, prev, code)
		}
		seen[code] = i
	}
}
```

- [ ] **Step 3: Run tests — verify they fail**

```bash
go test ./internal/shortcode/ -v
```

Expected: compilation error (functions not defined)

- [ ] **Step 4: Implement Base62 encode/decode**

Create `internal/shortcode/base62.go`:

```go
package shortcode

import (
	"fmt"
	"strings"
)

const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

var base = int64(len(alphabet))

// Encode converts a positive integer ID to a Base62 string.
func Encode(id int64) string {
	if id == 0 {
		return string(alphabet[0])
	}

	var chars []byte
	for id > 0 {
		remainder := id % base
		chars = append([]byte{alphabet[remainder]}, chars...)
		id /= base
	}
	return string(chars)
}

// Decode converts a Base62 string back to an integer ID.
func Decode(code string) (int64, error) {
	var id int64
	for _, c := range code {
		idx := strings.IndexRune(alphabet, c)
		if idx == -1 {
			return 0, fmt.Errorf("invalid character in short code: %q", c)
		}
		id = id*base + int64(idx)
	}
	return id, nil
}
```

- [ ] **Step 5: Run tests — verify they pass**

```bash
go test ./internal/shortcode/ -v
```

Expected: all 6 tests PASS

- [ ] **Step 6: Commit**

```bash
git add .
git commit -m "feat: project scaffolding and Base62 encoding with tests"
```

---

### Task 2: URL and Alias Validation

**Files:**
- Create: `internal/validator/validator.go`
- Create: `internal/validator/validator_test.go`

**Interfaces:**
- Produces: `validator.ValidateURL(rawURL string) error`, `validator.ValidateAlias(alias string) error`

- [ ] **Step 1: Write validation tests**

Create `internal/validator/validator_test.go`:

```go
package validator

import (
	"strings"
	"testing"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid https", "https://example.com", false},
		{"valid http", "http://example.com/path?q=1", false},
		{"valid with port", "https://example.com:8080/path", false},
		{"valid with fragment", "https://example.com/page#section", false},
		{"empty string", "", true},
		{"no scheme", "example.com", true},
		{"ftp scheme", "ftp://example.com", true},
		{"no host", "https://", true},
		{"just scheme", "http://", true},
		{"too long", "https://example.com/" + strings.Repeat("a", 2048), true},
		{"spaces in url", "https://exam ple.com", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAlias(t *testing.T) {
	tests := []struct {
		name    string
		alias   string
		wantErr bool
	}{
		{"valid lowercase", "my-link", false},
		{"valid alphanumeric", "abc123", false},
		{"valid with hyphens", "my-cool-link", false},
		{"valid 3 chars", "abc", false},
		{"valid 30 chars", strings.Repeat("a", 30), false},
		{"too short", "ab", true},
		{"too long", strings.Repeat("a", 31), true},
		{"underscore not allowed", "my_link", true},
		{"special chars", "my-link!", true},
		{"spaces", "my link", true},
		{"reserved shorten", "shorten", true},
		{"reserved static", "static", true},
		{"reserved health", "health", true},
		{"reserved api", "api", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAlias(tt.alias)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAlias(%q) error = %v, wantErr %v", tt.alias, err, tt.wantErr)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
go test ./internal/validator/ -v
```

Expected: compilation error (functions not defined)

- [ ] **Step 3: Implement validators**

Create `internal/validator/validator.go`:

```go
package validator

import (
	"fmt"
	"net/url"
	"strings"
)

const MaxURLLength = 2048

// Reserved aliases that conflict with server routes.
var reservedAliases = map[string]bool{
	"shorten": true,
	"static":  true,
	"health":  true,
	"api":     true,
}

// ValidateURL checks that rawURL is a valid HTTP or HTTPS URL.
func ValidateURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("url is required")
	}
	if len(rawURL) > MaxURLLength {
		return fmt.Errorf("url exceeds maximum length of %d characters", MaxURLLength)
	}
	if strings.ContainsAny(rawURL, " \t\n\r") {
		return fmt.Errorf("url must not contain whitespace")
	}
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("url must have http or https scheme")
	}
	if u.Host == "" {
		return fmt.Errorf("url must have a valid host")
	}
	return nil
}

// ValidateAlias checks that alias is 3-30 alphanumeric/hyphen characters and not reserved.
func ValidateAlias(alias string) error {
	if len(alias) < 3 || len(alias) > 30 {
		return fmt.Errorf("custom alias must be 3-30 characters")
	}
	for _, c := range alias {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '-') {
			return fmt.Errorf("custom alias must contain only alphanumeric characters and hyphens")
		}
	}
	if reservedAliases[alias] {
		return fmt.Errorf("alias %q is reserved", alias)
	}
	return nil
}
```

- [ ] **Step 4: Run tests — verify they pass**

```bash
go test ./internal/validator/ -v
```

Expected: all tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/validator/
git commit -m "feat: URL and alias validation with tests"
```

---

### Task 3: Data Model + Repository

**Files:**
- Create: `internal/model/model.go`
- Create: `internal/repository/repository.go`
- Create: `internal/repository/postgres.go`

**Interfaces:**
- Consumes: `shortcode.Encode(id int64) string`
- Produces: `repository.Repository` interface with `CreateURL`, `CreateURLWithAlias`, `GetByShortCode`; sentinel errors `ErrNotFound`, `ErrAliasConflict`

- [ ] **Step 1: Create model types**

Create `internal/model/model.go`:

```go
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
```

- [ ] **Step 2: Create repository interface and sentinel errors**

Create `internal/repository/repository.go`:

```go
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
```

- [ ] **Step 3: Implement PostgreSQL repository**

Create `internal/repository/postgres.go`:

```go
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
```

- [ ] **Step 4: Add lib/pq dependency**

```bash
go get github.com/lib/pq
```

- [ ] **Step 5: Verify compilation**

```bash
go build ./internal/...
```

Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add .
git commit -m "feat: data model and PostgreSQL repository"
```

---

### Task 4: HTTP Handlers + Tests

**Files:**
- Create: `internal/handler/handler.go`
- Create: `internal/handler/handler_test.go`

**Interfaces:**
- Consumes: `repository.Repository`, `validator.ValidateURL`, `validator.ValidateAlias`, `repository.ErrNotFound`, `repository.ErrAliasConflict`
- Produces: `handler.Handler` with `Shorten(c *gin.Context)`, `Redirect(c *gin.Context)`

- [ ] **Step 1: Add Gin dependency**

```bash
go get github.com/gin-gonic/gin
```

- [ ] **Step 2: Write handler tests**

Create `internal/handler/handler_test.go`:

```go
package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"url-shortener/internal/handler"
	"url-shortener/internal/model"
	"url-shortener/internal/repository"

	"github.com/gin-gonic/gin"
)

// --- Mock Repository ---

type mockRepository struct {
	urls   map[string]*model.URL
	nextID int64
}

func newMockRepo() *mockRepository {
	return &mockRepository{urls: make(map[string]*model.URL), nextID: 1}
}

func (m *mockRepository) CreateURL(originalURL string) (*model.URL, error) {
	code := fmt.Sprintf("abc%d", m.nextID)
	u := &model.URL{
		ID:          m.nextID,
		ShortCode:   code,
		OriginalURL: originalURL,
		CreatedAt:   time.Now(),
	}
	m.nextID++
	m.urls[code] = u
	return u, nil
}

func (m *mockRepository) CreateURLWithAlias(originalURL, alias string) (*model.URL, error) {
	if existing, ok := m.urls[alias]; ok {
		if existing.OriginalURL == originalURL {
			return existing, nil
		}
		return nil, repository.ErrAliasConflict
	}
	u := &model.URL{
		ID:          m.nextID,
		ShortCode:   alias,
		OriginalURL: originalURL,
		CreatedAt:   time.Now(),
	}
	m.nextID++
	m.urls[alias] = u
	return u, nil
}

func (m *mockRepository) GetByShortCode(shortCode string) (*model.URL, error) {
	u, ok := m.urls[shortCode]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return u, nil
}

// --- Helpers ---

func setupRouter(repo repository.Repository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := handler.New(repo, "http://localhost:8080")
	r := gin.New()
	r.POST("/shorten", h.Shorten)
	r.GET("/:code", h.Redirect)
	return r
}

func shortenRequest(t *testing.T, router *gin.Engine, body map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/shorten", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// --- Tests ---

func TestShortenValidURL(t *testing.T) {
	router := setupRouter(newMockRepo())
	w := shortenRequest(t, router, map[string]string{"url": "https://example.com"})

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.ShortenResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.ShortCode == "" {
		t.Error("short_code should not be empty")
	}
	if resp.OriginalURL != "https://example.com" {
		t.Errorf("original_url = %q, want %q", resp.OriginalURL, "https://example.com")
	}
}

func TestShortenEmptyURL(t *testing.T) {
	router := setupRouter(newMockRepo())
	w := shortenRequest(t, router, map[string]string{"url": ""})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestShortenInvalidURL(t *testing.T) {
	router := setupRouter(newMockRepo())
	w := shortenRequest(t, router, map[string]string{"url": "not-a-url"})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestShortenMissingBody(t *testing.T) {
	router := setupRouter(newMockRepo())
	req := httptest.NewRequest(http.MethodPost, "/shorten", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestShortenDuplicateURLGetsDifferentCodes(t *testing.T) {
	router := setupRouter(newMockRepo())
	w1 := shortenRequest(t, router, map[string]string{"url": "https://example.com"})
	w2 := shortenRequest(t, router, map[string]string{"url": "https://example.com"})

	var resp1, resp2 model.ShortenResponse
	json.Unmarshal(w1.Body.Bytes(), &resp1)
	json.Unmarshal(w2.Body.Bytes(), &resp2)

	if resp1.ShortCode == resp2.ShortCode {
		t.Errorf("duplicate URL should get different codes, both got %q", resp1.ShortCode)
	}
}

func TestShortenWithCustomAlias(t *testing.T) {
	router := setupRouter(newMockRepo())
	w := shortenRequest(t, router, map[string]string{
		"url":          "https://example.com",
		"custom_alias": "my-link",
	})

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.ShortenResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.ShortCode != "my-link" {
		t.Errorf("short_code = %q, want %q", resp.ShortCode, "my-link")
	}
}

func TestShortenCustomAliasSameURLIsIdempotent(t *testing.T) {
	router := setupRouter(newMockRepo())
	w1 := shortenRequest(t, router, map[string]string{
		"url":          "https://example.com",
		"custom_alias": "my-link",
	})
	w2 := shortenRequest(t, router, map[string]string{
		"url":          "https://example.com",
		"custom_alias": "my-link",
	})

	if w1.Code != http.StatusCreated || w2.Code != http.StatusCreated {
		t.Fatalf("expected both 201, got %d and %d", w1.Code, w2.Code)
	}

	var resp1, resp2 model.ShortenResponse
	json.Unmarshal(w1.Body.Bytes(), &resp1)
	json.Unmarshal(w2.Body.Bytes(), &resp2)
	if resp1.ShortCode != resp2.ShortCode {
		t.Error("same alias + same URL should return the same short code")
	}
}

func TestShortenCustomAliasConflict(t *testing.T) {
	router := setupRouter(newMockRepo())
	shortenRequest(t, router, map[string]string{
		"url":          "https://example.com",
		"custom_alias": "my-link",
	})
	w := shortenRequest(t, router, map[string]string{
		"url":          "https://different.com",
		"custom_alias": "my-link",
	})

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestShortenInvalidAliasTooShort(t *testing.T) {
	router := setupRouter(newMockRepo())
	w := shortenRequest(t, router, map[string]string{
		"url":          "https://example.com",
		"custom_alias": "ab",
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestShortenReservedAlias(t *testing.T) {
	router := setupRouter(newMockRepo())
	w := shortenRequest(t, router, map[string]string{
		"url":          "https://example.com",
		"custom_alias": "shorten",
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRedirectValidCode(t *testing.T) {
	mock := newMockRepo()
	router := setupRouter(mock)

	// Create a URL first
	shortenRequest(t, router, map[string]string{"url": "https://example.com"})

	// Get the code from mock
	var code string
	for k := range mock.urls {
		code = k
		break
	}

	req := httptest.NewRequest(http.MethodGet, "/"+code, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusMovedPermanently {
		t.Fatalf("expected 301, got %d: %s", w.Code, w.Body.String())
	}
	location := w.Header().Get("Location")
	if location != "https://example.com" {
		t.Errorf("Location = %q, want %q", location, "https://example.com")
	}
}

func TestRedirectUnknownCode(t *testing.T) {
	router := setupRouter(newMockRepo())

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 3: Run tests — verify they fail**

```bash
go test ./internal/handler/ -v
```

Expected: compilation error (handler package not defined)

- [ ] **Step 4: Implement handlers**

Create `internal/handler/handler.go`:

```go
package handler

import (
	"errors"
	"net/http"

	"url-shortener/internal/model"
	"url-shortener/internal/repository"
	"url-shortener/internal/validator"

	"github.com/gin-gonic/gin"
)

// Handler contains HTTP handlers for the URL shortener API.
type Handler struct {
	repo    repository.Repository
	baseURL string
}

// New creates a new Handler.
func New(repo repository.Repository, baseURL string) *Handler {
	return &Handler{repo: repo, baseURL: baseURL}
}

// Shorten handles POST /shorten — creates a short URL mapping.
func (h *Handler) Shorten(c *gin.Context) {
	var req model.ShortenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := validator.ValidateURL(req.URL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var u *model.URL
	var err error

	if req.CustomAlias != "" {
		if valErr := validator.ValidateAlias(req.CustomAlias); valErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": valErr.Error()})
			return
		}
		u, err = h.repo.CreateURLWithAlias(req.URL, req.CustomAlias)
		if errors.Is(err, repository.ErrAliasConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "custom alias already taken by a different URL"})
			return
		}
	} else {
		u, err = h.repo.CreateURL(req.URL)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create short URL"})
		return
	}

	c.JSON(http.StatusCreated, model.ShortenResponse{
		ShortCode:   u.ShortCode,
		ShortURL:    h.baseURL + "/" + u.ShortCode,
		OriginalURL: u.OriginalURL,
	})
}

// Redirect handles GET /:code — redirects to the original URL.
func (h *Handler) Redirect(c *gin.Context) {
	code := c.Param("code")

	u, err := h.repo.GetByShortCode(code)
	if errors.Is(err, repository.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "short code not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve short code"})
		return
	}

	c.Redirect(http.StatusMovedPermanently, u.OriginalURL)
}
```

- [ ] **Step 5: Run tests — verify they pass**

```bash
go test ./internal/handler/ -v
```

Expected: all 12 tests PASS

- [ ] **Step 6: Commit**

```bash
git add internal/handler/ internal/model/
git commit -m "feat: HTTP handlers with comprehensive tests"
```

---

### Task 5: Server Entry Point + Database Migration

**Files:**
- Create: `cmd/server/main.go`

**Interfaces:**
- Consumes: `handler.New`, `repository.NewPostgresRepository`, Gin router

- [ ] **Step 1: Implement main.go**

Create `cmd/server/main.go`:

```go
package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"url-shortener/internal/handler"
	"url-shortener/internal/repository"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

func main() {
	dbURL := getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/urlshortener?sslmode=disable")
	baseURL := getEnv("BASE_URL", "http://localhost:8080")
	port := getEnv("PORT", "8080")

	db, err := connectDB(dbURL)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer db.Close()
	log.Println("Connected to database")

	if err := runMigrations(db); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	log.Println("Migrations applied")

	repo := repository.NewPostgresRepository(db)
	h := handler.New(repo, baseURL)

	r := gin.Default()

	// Serve frontend
	r.StaticFile("/", "./static/index.html")
	r.StaticFile("/favicon.ico", "./static/favicon.ico")

	// API
	r.POST("/shorten", h.Shorten)

	// Redirect — catch-all for short codes
	r.GET("/:code", h.Redirect)

	log.Printf("Server starting on :%s (base URL: %s)", port, baseURL)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// connectDB retries connection to the database up to 30 times (1 s apart).
func connectDB(dbURL string) (*sql.DB, error) {
	var db *sql.DB
	var err error
	for i := 0; i < 30; i++ {
		db, err = sql.Open("postgres", dbURL)
		if err == nil {
			if pingErr := db.Ping(); pingErr == nil {
				return db, nil
			}
		}
		log.Printf("Waiting for database... attempt %d/30", i+1)
		time.Sleep(time.Second)
	}
	return nil, fmt.Errorf("could not connect after 30 attempts: %w", err)
}

func runMigrations(db *sql.DB) error {
	migration := `
	CREATE TABLE IF NOT EXISTS urls (
		id           BIGSERIAL PRIMARY KEY,
		short_code   VARCHAR(30) NOT NULL UNIQUE,
		original_url TEXT        NOT NULL,
		created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_urls_short_code ON urls(short_code);
	`
	_, err := db.Exec(migration)
	return err
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./cmd/server/
```

Expected: compiles without error (binary is not run — no database yet)

- [ ] **Step 3: Commit**

```bash
git add cmd/
git commit -m "feat: server entry point with DB connection and auto-migration"
```

---

### Task 6: Static Frontend

**Files:**
- Create: `static/index.html`

> [!IMPORTANT]
> All CSS and JS are inlined to avoid routing conflicts with Gin's `/:code` wildcard route. Google Fonts loaded via CDN `<link>`.

- [ ] **Step 1: Create the frontend**

Create `static/index.html` — dark-themed, glassmorphic single-page app with Inter font, micro-animations, and copy-to-clipboard:

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>URL Shortener</title>
    <meta name="description" content="Shorten long URLs into compact, shareable short links">
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" rel="stylesheet">
    <style>
        *, *::before, *::after { margin: 0; padding: 0; box-sizing: border-box; }

        :root {
            --bg-primary: #0a0a0f;
            --bg-card: rgba(255, 255, 255, 0.04);
            --border-card: rgba(255, 255, 255, 0.08);
            --text-primary: #e8e8ed;
            --text-secondary: #8a8a9a;
            --accent: #6c5ce7;
            --accent-glow: rgba(108, 92, 231, 0.3);
            --accent-hover: #7c6ef7;
            --success: #00d2a0;
            --error: #ff6b6b;
            --radius: 16px;
        }

        body {
            font-family: 'Inter', -apple-system, sans-serif;
            background: var(--bg-primary);
            color: var(--text-primary);
            min-height: 100vh;
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;
            padding: 24px;
            overflow: hidden;
        }

        /* Animated background orbs */
        body::before, body::after {
            content: '';
            position: fixed;
            border-radius: 50%;
            filter: blur(120px);
            opacity: 0.15;
            z-index: -1;
            animation: float 20s ease-in-out infinite;
        }
        body::before {
            width: 600px; height: 600px;
            background: var(--accent);
            top: -200px; left: -100px;
        }
        body::after {
            width: 500px; height: 500px;
            background: #e84393;
            bottom: -200px; right: -100px;
            animation-delay: -10s;
        }

        @keyframes float {
            0%, 100% { transform: translate(0, 0) scale(1); }
            33% { transform: translate(30px, -30px) scale(1.05); }
            66% { transform: translate(-20px, 20px) scale(0.95); }
        }

        .container {
            width: 100%;
            max-width: 560px;
            animation: fadeUp 0.6s ease-out;
        }

        @keyframes fadeUp {
            from { opacity: 0; transform: translateY(24px); }
            to { opacity: 1; transform: translateY(0); }
        }

        h1 {
            font-size: 2rem;
            font-weight: 700;
            text-align: center;
            margin-bottom: 8px;
            letter-spacing: -0.02em;
        }
        h1 span { color: var(--accent); }

        .subtitle {
            text-align: center;
            color: var(--text-secondary);
            font-size: 0.95rem;
            margin-bottom: 32px;
        }

        .card {
            background: var(--bg-card);
            border: 1px solid var(--border-card);
            border-radius: var(--radius);
            padding: 32px;
            backdrop-filter: blur(24px);
            -webkit-backdrop-filter: blur(24px);
        }

        .input-group {
            margin-bottom: 16px;
        }

        label {
            display: block;
            font-size: 0.8rem;
            font-weight: 500;
            color: var(--text-secondary);
            margin-bottom: 8px;
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }

        input {
            width: 100%;
            padding: 14px 16px;
            background: rgba(255, 255, 255, 0.05);
            border: 1px solid rgba(255, 255, 255, 0.1);
            border-radius: 10px;
            color: var(--text-primary);
            font-family: inherit;
            font-size: 0.95rem;
            outline: none;
            transition: border-color 0.2s, box-shadow 0.2s;
        }
        input::placeholder { color: rgba(255, 255, 255, 0.2); }
        input:focus {
            border-color: var(--accent);
            box-shadow: 0 0 0 3px var(--accent-glow);
        }

        button#shorten-btn {
            width: 100%;
            padding: 14px;
            margin-top: 8px;
            background: var(--accent);
            color: #fff;
            font-family: inherit;
            font-size: 1rem;
            font-weight: 600;
            border: none;
            border-radius: 10px;
            cursor: pointer;
            transition: background 0.2s, transform 0.1s, box-shadow 0.2s;
        }
        button#shorten-btn:hover {
            background: var(--accent-hover);
            box-shadow: 0 4px 24px var(--accent-glow);
        }
        button#shorten-btn:active { transform: scale(0.98); }
        button#shorten-btn:disabled {
            opacity: 0.5;
            cursor: not-allowed;
            transform: none;
        }

        /* Result */
        .result {
            margin-top: 24px;
            padding: 20px;
            background: rgba(0, 210, 160, 0.06);
            border: 1px solid rgba(0, 210, 160, 0.15);
            border-radius: 12px;
            display: none;
            animation: fadeUp 0.4s ease-out;
        }
        .result.visible { display: block; }

        .result-label {
            font-size: 0.75rem;
            font-weight: 500;
            color: var(--success);
            text-transform: uppercase;
            letter-spacing: 0.05em;
            margin-bottom: 10px;
        }

        .result-url-row {
            display: flex;
            align-items: center;
            gap: 10px;
        }

        .result-url {
            flex: 1;
            font-size: 1rem;
            font-weight: 600;
            color: var(--text-primary);
            word-break: break-all;
        }
        .result-url a {
            color: var(--text-primary);
            text-decoration: none;
            border-bottom: 1px dashed rgba(255,255,255,0.2);
            transition: border-color 0.2s;
        }
        .result-url a:hover { border-color: var(--accent); }

        button.copy-btn {
            padding: 8px 16px;
            background: rgba(255,255,255,0.08);
            color: var(--text-secondary);
            border: 1px solid rgba(255,255,255,0.1);
            border-radius: 8px;
            font-family: inherit;
            font-size: 0.8rem;
            cursor: pointer;
            transition: all 0.2s;
            white-space: nowrap;
        }
        button.copy-btn:hover { background: rgba(255,255,255,0.12); color: var(--text-primary); }
        button.copy-btn.copied { background: rgba(0,210,160,0.15); color: var(--success); border-color: rgba(0,210,160,0.3); }

        /* Error */
        .error-msg {
            margin-top: 16px;
            padding: 14px 16px;
            background: rgba(255, 107, 107, 0.08);
            border: 1px solid rgba(255, 107, 107, 0.2);
            border-radius: 10px;
            color: var(--error);
            font-size: 0.9rem;
            display: none;
            animation: fadeUp 0.3s ease-out;
        }
        .error-msg.visible { display: block; }

        .footer {
            margin-top: 40px;
            text-align: center;
            font-size: 0.8rem;
            color: var(--text-secondary);
        }
    </style>
</head>
<body>
    <div class="container">
        <h1><span>⚡</span> URL Shortener</h1>
        <p class="subtitle">Turn long URLs into compact, shareable links</p>

        <div class="card">
            <div class="input-group">
                <label for="url-input">Paste your long URL</label>
                <input type="url" id="url-input" placeholder="https://example.com/very/long/path..." autofocus>
            </div>
            <div class="input-group">
                <label for="alias-input">Custom alias <span style="opacity:0.5">(optional)</span></label>
                <input type="text" id="alias-input" placeholder="my-custom-link">
            </div>
            <button id="shorten-btn">Shorten URL</button>

            <div class="error-msg" id="error-msg"></div>

            <div class="result" id="result">
                <div class="result-label">Your short link</div>
                <div class="result-url-row">
                    <div class="result-url"><a id="short-url" href="#" target="_blank"></a></div>
                    <button class="copy-btn" id="copy-btn">Copy</button>
                </div>
            </div>
        </div>

        <p class="footer">Paste a URL, get a short link. It's that simple.</p>
    </div>

    <script>
        const urlInput    = document.getElementById('url-input');
        const aliasInput  = document.getElementById('alias-input');
        const shortenBtn  = document.getElementById('shorten-btn');
        const resultEl    = document.getElementById('result');
        const shortUrlEl  = document.getElementById('short-url');
        const copyBtn     = document.getElementById('copy-btn');
        const errorEl     = document.getElementById('error-msg');

        function showError(msg) {
            resultEl.classList.remove('visible');
            errorEl.textContent = msg;
            errorEl.classList.add('visible');
        }

        function hideError() {
            errorEl.classList.remove('visible');
        }

        async function shorten() {
            hideError();
            const url = urlInput.value.trim();
            if (!url) { showError('Please enter a URL.'); return; }

            shortenBtn.disabled = true;
            shortenBtn.textContent = 'Shortening...';

            try {
                const body = { url };
                const alias = aliasInput.value.trim();
                if (alias) body.custom_alias = alias;

                const res = await fetch('/shorten', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(body),
                });

                const data = await res.json();

                if (!res.ok) {
                    showError(data.error || 'Something went wrong.');
                    return;
                }

                shortUrlEl.textContent = data.short_url;
                shortUrlEl.href = data.short_url;
                resultEl.classList.add('visible');
                copyBtn.textContent = 'Copy';
                copyBtn.classList.remove('copied');
            } catch (err) {
                showError('Network error. Is the server running?');
            } finally {
                shortenBtn.disabled = false;
                shortenBtn.textContent = 'Shorten URL';
            }
        }

        shortenBtn.addEventListener('click', shorten);
        urlInput.addEventListener('keydown', (e) => { if (e.key === 'Enter') shorten(); });

        copyBtn.addEventListener('click', () => {
            navigator.clipboard.writeText(shortUrlEl.textContent).then(() => {
                copyBtn.textContent = 'Copied!';
                copyBtn.classList.add('copied');
                setTimeout(() => { copyBtn.textContent = 'Copy'; copyBtn.classList.remove('copied'); }, 2000);
            });
        });
    </script>
</body>
</html>
```

- [ ] **Step 2: Commit**

```bash
git add static/
git commit -m "feat: minimalistic dark-themed frontend"
```

---

### Task 7: Docker Configuration

**Files:**
- Create: `Dockerfile`
- Create: `docker-compose.yml`

- [ ] **Step 1: Create Dockerfile**

Create `Dockerfile` — multi-stage build for a small final image:

```dockerfile
# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server

# Runtime stage
FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/server .
COPY --from=builder /app/static ./static
EXPOSE 8080
CMD ["./server"]
```

- [ ] **Step 2: Create docker-compose.yml**

Create `docker-compose.yml`:

```yaml
services:
  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: urlshortener
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 5s
      retries: 5

  api:
    build: .
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgres://postgres:postgres@db:5432/urlshortener?sslmode=disable
      BASE_URL: http://localhost:8080
      PORT: "8080"
    depends_on:
      db:
        condition: service_healthy

volumes:
  pgdata:
```

- [ ] **Step 3: Commit**

```bash
git add Dockerfile docker-compose.yml
git commit -m "feat: Docker and docker-compose configuration"
```

---

### Task 8: README + Final Verification

**Files:**
- Create: `README.md`

- [ ] **Step 1: Create README**

Create `README.md`:

````markdown
# URL Shortener

A URL shortening service that turns long URLs into compact short codes and redirects visitors to the original link. Built with Go (Gin), PostgreSQL, and a minimalistic frontend.

## Quick Start

```bash
docker compose up --build
```

The app will be available at **http://localhost:8080**.

## API

### Shorten a URL

```bash
curl -X POST http://localhost:8080/shorten \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com/very/long/path"}'
```

Response:
```json
{
  "short_code": "1",
  "short_url": "http://localhost:8080/1",
  "original_url": "https://example.com/very/long/path"
}
```

### Shorten with a custom alias

```bash
curl -X POST http://localhost:8080/shorten \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com", "custom_alias": "my-link"}'
```

### Redirect

```bash
curl -L http://localhost:8080/1
# → redirects (301) to https://example.com/very/long/path
```

### Unknown code

```bash
curl http://localhost:8080/nonexistent
# → 404 {"error": "short code not found"}
```

## Design Decisions

### Short-code generation

Short codes are generated by **Base62-encoding the PostgreSQL auto-increment ID**. The alphabet is `0-9a-zA-Z` — all characters are URL-safe with no encoding needed.

This approach is **collision-free by design**: each database row gets a unique, monotonically increasing integer ID, which maps to a unique Base62 string. There are no retry loops or collision checks needed.

At 7 characters, Base62 supports 62⁷ ≈ 3.5 trillion unique codes.

### Duplicate URL handling

Every `POST /shorten` (without a custom alias) generates a **new short code**, even if the same URL was shortened before. This is deliberate:

- **Simpler insert path** — no read-before-write needed
- **Independent tracking** — each short code can represent a separate campaign or context
- **No hidden coupling** — users always get a fresh, predictable mapping

### Custom alias conflicts

- Same alias + same URL → returns the existing mapping (idempotent)
- Same alias + different URL → returns `409 Conflict`
- Reserved words (`shorten`, `static`, `health`, `api`) are rejected

### URL validation

- Must have `http://` or `https://` scheme
- Must have a valid host
- Maximum 2048 characters
- No whitespace allowed

## Running Tests

```bash
go test ./... -v
```

## Tech Stack

- **Go 1.22** with Gin web framework
- **PostgreSQL 16** for persistence
- **Docker Compose** for orchestration
- Vanilla HTML/CSS/JS frontend (inlined, no build step)

## Project Structure

```
├── cmd/server/main.go           # Entry point, DB setup, router
├── internal/
│   ├── handler/                 # HTTP handlers + tests
│   ├── model/                   # Data types
│   ├── repository/              # DB interface + PostgreSQL impl
│   ├── shortcode/               # Base62 encoding + tests
│   └── validator/               # URL/alias validation + tests
├── static/index.html            # Frontend (CSS/JS inlined)
├── Dockerfile                   # Multi-stage Go build
├── docker-compose.yml           # PostgreSQL + API
└── README.md
```
````

- [ ] **Step 2: Run all tests**

```bash
go test ./... -v -count=1
```

Expected: all tests in `shortcode`, `validator`, and `handler` packages PASS

- [ ] **Step 3: Build and start with Docker Compose**

```bash
docker compose up --build -d
```

Expected: both `db` and `api` services start successfully

- [ ] **Step 4: Smoke test the running service**

```bash
# Shorten a URL
curl -s -X POST http://localhost:8080/shorten \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com"}' | jq .

# Redirect (should return 301)
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/1

# Unknown code (should return 404)
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/nonexistent

# Custom alias
curl -s -X POST http://localhost:8080/shorten \
  -H "Content-Type: application/json" \
  -d '{"url": "https://google.com", "custom_alias": "goog"}' | jq .

# Alias conflict (should return 409)
curl -s -X POST http://localhost:8080/shorten \
  -H "Content-Type: application/json" \
  -d '{"url": "https://different.com", "custom_alias": "goog"}' | jq .
```

- [ ] **Step 5: Commit**

```bash
git add README.md
git commit -m "docs: README with install, run, test, and design decisions"
```

---

## Verification Plan

### Automated Tests

```bash
go test ./... -v -count=1
```

Covers:
- Base62 encode/decode, round-trips, URL-safety, no-duplicate guarantee (100k IDs)
- URL validation (valid, invalid scheme, missing host, too long, whitespace)
- Alias validation (length bounds, special chars, reserved words)
- Handler: shorten valid URL, empty URL, invalid URL, missing body, duplicate URLs get different codes, custom alias, alias idempotency, alias conflict (409), redirect (301), unknown code (404)

### Manual Verification

- `docker compose up --build` starts both services
- Frontend at http://localhost:8080 loads and can shorten URLs
- Shortened URLs redirect correctly in a browser
- Copy button works
