package handler

import (
	"errors"
	"fmt"
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
			c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("Custom alias '%s' already taken by a different URL. Choose another custom alias", req.CustomAlias)})
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
