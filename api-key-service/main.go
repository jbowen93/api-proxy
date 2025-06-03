package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/pbkdf2"
)

type APIKey struct {
	ID               uuid.UUID `json:"id" db:"id"`
	UserID           string    `json:"user_id" db:"user_id"`
	KeyHash          string    `json:"-" db:"key_hash"`
	Name             string    `json:"name" db:"name"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	LastUsedAt       *time.Time `json:"last_used_at" db:"last_used_at"`
	IsActive         bool      `json:"is_active" db:"is_active"`
	RateLimitPerMin  int       `json:"rate_limit_per_minute" db:"rate_limit_per_minute"`
	RateLimitPerDay  int       `json:"rate_limit_per_day" db:"rate_limit_per_day"`
}

type CreateAPIKeyRequest struct {
	UserID          string `json:"user_id" binding:"required"`
	Name            string `json:"name" binding:"required"`
	RateLimitPerMin int    `json:"rate_limit_per_minute"`
	RateLimitPerDay int    `json:"rate_limit_per_day"`
}

type CreateAPIKeyResponse struct {
	APIKey    APIKey `json:"api_key"`
	PlainKey  string `json:"plain_key"`
}

type Server struct {
	db    *sql.DB
	redis *redis.Client
}

func main() {
	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: strings.TrimPrefix(os.Getenv("REDIS_URL"), "redis://"),
	})

	server := &Server{db: db, redis: rdb}

	r := gin.Default()
	
	// API Key management endpoints
	r.POST("/api/keys", server.createAPIKey)
	r.GET("/api/keys/:user_id", server.listAPIKeys)
	r.DELETE("/api/keys/:id", server.deleteAPIKey)
	
	// Auth endpoint for Envoy
	r.GET("/auth", server.authenticate)

	log.Println("API Key Service starting on :8080")
	r.Run(":8080")
}

func (s *Server) createAPIKey(c *gin.Context) {
	var req CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate API key
	keyBytes := make([]byte, 32)
	rand.Read(keyBytes)
	plainKey := "sk-" + hex.EncodeToString(keyBytes)
	
	// Hash the key for storage
	keyHash := hashAPIKey(plainKey)

	// Set defaults
	if req.RateLimitPerMin == 0 {
		req.RateLimitPerMin = 1000
	}
	if req.RateLimitPerDay == 0 {
		req.RateLimitPerDay = 100000
	}

	apiKey := APIKey{
		ID:              uuid.New(),
		UserID:          req.UserID,
		KeyHash:         keyHash,
		Name:            req.Name,
		CreatedAt:       time.Now(),
		IsActive:        true,
		RateLimitPerMin: req.RateLimitPerMin,
		RateLimitPerDay: req.RateLimitPerDay,
	}

	query := `
		INSERT INTO api_keys (id, user_id, key_hash, name, created_at, is_active, rate_limit_per_minute, rate_limit_per_day)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	
	_, err := s.db.Exec(query, apiKey.ID, apiKey.UserID, apiKey.KeyHash, apiKey.Name, 
		apiKey.CreatedAt, apiKey.IsActive, apiKey.RateLimitPerMin, apiKey.RateLimitPerDay)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create API key"})
		return
	}

	c.JSON(http.StatusCreated, CreateAPIKeyResponse{
		APIKey:   apiKey,
		PlainKey: plainKey,
	})
}

func (s *Server) listAPIKeys(c *gin.Context) {
	userID := c.Param("user_id")
	
	query := `SELECT id, user_id, name, created_at, last_used_at, is_active, rate_limit_per_minute, rate_limit_per_day 
		FROM api_keys WHERE user_id = $1 AND is_active = TRUE`
	
	rows, err := s.db.Query(query, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch API keys"})
		return
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		var key APIKey
		err := rows.Scan(&key.ID, &key.UserID, &key.Name, &key.CreatedAt, 
			&key.LastUsedAt, &key.IsActive, &key.RateLimitPerMin, &key.RateLimitPerDay)
		if err != nil {
			continue
		}
		keys = append(keys, key)
	}

	c.JSON(http.StatusOK, keys)
}

func (s *Server) deleteAPIKey(c *gin.Context) {
	keyID := c.Param("id")
	
	query := `UPDATE api_keys SET is_active = FALSE WHERE id = $1`
	result, err := s.db.Exec(query, keyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete API key"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "API key deleted"})
}

func (s *Server) authenticate(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.Header("x-ext-authz-check-result", "forbidden")
		c.Status(http.StatusForbidden)
		return
	}

	// Extract API key from Bearer token
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		c.Header("x-ext-authz-check-result", "forbidden")
		c.Status(http.StatusForbidden)
		return
	}

	apiKey := parts[1]
	keyHash := hashAPIKey(apiKey)

	// Check if key exists and is active
	var dbKey APIKey
	query := `SELECT id, user_id, rate_limit_per_minute, rate_limit_per_day 
		FROM api_keys WHERE key_hash = $1 AND is_active = TRUE`
	
	err := s.db.QueryRow(query, keyHash).Scan(&dbKey.ID, &dbKey.UserID, 
		&dbKey.RateLimitPerMin, &dbKey.RateLimitPerDay)
	if err != nil {
		c.Header("x-ext-authz-check-result", "forbidden")
		c.Status(http.StatusForbidden)
		return
	}

	// Check rate limits
	if !s.checkRateLimit(dbKey.ID.String(), dbKey.RateLimitPerMin, dbKey.RateLimitPerDay) {
		c.Header("x-ext-authz-check-result", "rate_limited")
		c.Status(http.StatusTooManyRequests)
		return
	}

	// Update last used timestamp
	s.db.Exec(`UPDATE api_keys SET last_used_at = CURRENT_TIMESTAMP WHERE id = $1`, dbKey.ID)

	// Pass user info to upstream
	c.Header("x-user-id", dbKey.UserID)
	c.Header("x-api-key-id", dbKey.ID.String())
	c.Header("x-ext-authz-check-result", "allowed")
	c.Status(http.StatusOK)
}

func (s *Server) checkRateLimit(keyID string, perMin, perDay int) bool {
	ctx := c.Background()
	now := time.Now()
	
	// Check per-minute limit
	minKey := fmt.Sprintf("rate_limit:min:%s:%d", keyID, now.Unix()/60)
	minCount, _ := s.redis.Incr(ctx, minKey).Result()
	s.redis.Expire(ctx, minKey, time.Minute)
	
	if int(minCount) > perMin {
		return false
	}

	// Check per-day limit
	dayKey := fmt.Sprintf("rate_limit:day:%s:%s", keyID, now.Format("2006-01-02"))
	dayCount, _ := s.redis.Incr(ctx, dayKey).Result()
	s.redis.Expire(ctx, dayKey, 24*time.Hour)
	
	return int(dayCount) <= perDay
}

func hashAPIKey(key string) string {
	salt := []byte("api-key-salt")
	hash := pbkdf2.Key([]byte(key), salt, 4096, 32, sha256.New)
	return hex.EncodeToString(hash)
}