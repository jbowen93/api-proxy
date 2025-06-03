package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type HelloResponse struct {
	Message   string            `json:"message"`
	Timestamp time.Time         `json:"timestamp"`
	UserID    string            `json:"user_id,omitempty"`
	APIKeyID  string            `json:"api_key_id,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
}

func main() {
	r := gin.Default()

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// Hello world endpoint
	r.GET("/hello", func(c *gin.Context) {
		// Extract user info passed from Envoy
		userID := c.GetHeader("x-user-id")
		apiKeyID := c.GetHeader("x-api-key-id")

		response := HelloResponse{
			Message:   "Hello, World!",
			Timestamp: time.Now(),
			UserID:    userID,
			APIKeyID:  apiKeyID,
		}

		c.JSON(http.StatusOK, response)
	})

	// Echo endpoint that returns request details
	r.Any("/echo", func(c *gin.Context) {
		headers := make(map[string]string)
		for key, values := range c.Request.Header {
			if len(values) > 0 {
				headers[key] = values[0]
			}
		}

		response := HelloResponse{
			Message:   "Echo response",
			Timestamp: time.Now(),
			UserID:    c.GetHeader("x-user-id"),
			APIKeyID:  c.GetHeader("x-api-key-id"),
			Headers:   headers,
		}

		c.JSON(http.StatusOK, response)
	})

	// Catch-all endpoint
	r.NoRoute(func(c *gin.Context) {
		response := HelloResponse{
			Message:   "Hello from backend! Path: " + c.Request.URL.Path,
			Timestamp: time.Now(),
			UserID:    c.GetHeader("x-user-id"),
			APIKeyID:  c.GetHeader("x-api-key-id"),
		}

		c.JSON(http.StatusOK, response)
	})

	r.Run(":3000")
}