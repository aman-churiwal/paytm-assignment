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
