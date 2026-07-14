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
