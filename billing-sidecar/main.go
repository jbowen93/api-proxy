package main

import (
	"database/sql"
	"log"
	"net"
	"os"
	"time"

	accesslog "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3"
	als "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v3"
	_ "github.com/lib/pq"
	"github.com/stripe/stripe-go/v76"
	"google.golang.org/grpc"
)

type BillingSidecar struct {
	db               *sql.DB
	stripeAPIKey     string
	usageBatch       []UsageRecord
	batchSize        int
	flushInterval    time.Duration
}

type UsageRecord struct {
	APIKeyID     string
	Endpoint     string
	Method       string
	StatusCode   int
	ResponseTime int64
	Timestamp    time.Time
}

func main() {
	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	stripeAPIKey := os.Getenv("STRIPE_API_KEY")
	if stripeAPIKey != "" {
		stripe.Key = stripeAPIKey
	}

	sidecar := &BillingSidecar{
		db:            db,
		stripeAPIKey:  stripeAPIKey,
		usageBatch:    make([]UsageRecord, 0),
		batchSize:     100,
		flushInterval: 30 * time.Second,
	}

	// Start batch flusher
	go sidecar.startBatchFlusher()

	// Start gRPC server for Envoy ALS
	lis, err := net.Listen("tcp", ":8081")
	if err != nil {
		log.Fatal("Failed to listen:", err)
	}

	s := grpc.NewServer()
	als.RegisterAccessLogServiceServer(s, sidecar)

	log.Println("Billing Sidecar starting on :8081")
	if err := s.Serve(lis); err != nil {
		log.Fatal("Failed to serve:", err)
	}
}

func (b *BillingSidecar) StreamAccessLogs(stream als.AccessLogService_StreamAccessLogsServer) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}

		for _, logEntry := range req.GetHttpLogs().GetLogEntry() {
			b.processAccessLog(logEntry)
		}
	}
}

func (b *BillingSidecar) processAccessLog(logEntry *accesslog.HTTPAccessLogEntry) {
	request := logEntry.GetRequest()
	response := logEntry.GetResponse()
	commonProps := logEntry.GetCommonProperties()

	// Extract API key ID from headers
	apiKeyIDHeader := ""
	for key, value := range request.GetRequestHeaders() {
		if key == "x-api-key-id" {
			apiKeyIDHeader = value
			break
		}
	}

	if apiKeyIDHeader == "" {
		return // Skip non-authenticated requests
	}

	// Parse response time
	var responseTime int64
	if commonProps.GetTimeToLastDownstreamTxByte() != nil {
		responseTime = commonProps.GetTimeToLastDownstreamTxByte().AsDuration().Milliseconds()
	}

	usage := UsageRecord{
		APIKeyID:     apiKeyIDHeader,
		Endpoint:     request.GetPath(),
		Method:       request.GetRequestMethod().String(),
		StatusCode:   int(response.GetResponseCode().GetValue()),
		ResponseTime: responseTime,
		Timestamp:    time.Now(),
	}

	b.addToBatch(usage)
}

func (b *BillingSidecar) addToBatch(usage UsageRecord) {
	b.usageBatch = append(b.usageBatch, usage)
	
	if len(b.usageBatch) >= b.batchSize {
		b.flushBatch()
	}
}

func (b *BillingSidecar) startBatchFlusher() {
	ticker := time.NewTicker(b.flushInterval)
	for range ticker.C {
		if len(b.usageBatch) > 0 {
			b.flushBatch()
		}
	}
}

func (b *BillingSidecar) flushBatch() {
	if len(b.usageBatch) == 0 {
		return
	}

	// Insert usage records to database
	for _, usage := range b.usageBatch {
		b.insertUsageRecord(usage)
	}

	// Send to Stripe Meter API (if configured)
	if b.stripeAPIKey != "" {
		b.sendToStripeMeter()
	}

	// Clear batch
	b.usageBatch = b.usageBatch[:0]
}

func (b *BillingSidecar) insertUsageRecord(usage UsageRecord) {
	query := `
		INSERT INTO api_usage (api_key_id, endpoint, method, status_code, response_time_ms, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6)`
	
	_, err := b.db.Exec(query, usage.APIKeyID, usage.Endpoint, usage.Method, 
		usage.StatusCode, usage.ResponseTime, usage.Timestamp)
	if err != nil {
		log.Printf("Failed to insert usage record: %v", err)
	}
}

func (b *BillingSidecar) sendToStripeMeter() {
	// Aggregate usage by API key
	usageByKey := make(map[string]int)
	for _, usage := range b.usageBatch {
		if usage.StatusCode >= 200 && usage.StatusCode < 300 {
			usageByKey[usage.APIKeyID]++
		}
	}

	// Send meter events to Stripe
	for apiKeyID, count := range usageByKey {
		// Get user ID for the API key
		var userID string
		err := b.db.QueryRow("SELECT user_id FROM api_keys WHERE id = $1", apiKeyID).Scan(&userID)
		if err != nil {
			log.Printf("Failed to get user ID for API key %s: %v", apiKeyID, err)
			continue
		}

		// Create meter event (simplified for example)
		log.Printf("Would send %d API requests to Stripe for user %s", count, userID)
		
		// In production, implement proper Stripe Meter API integration
		// For now, just log the billing event

		// Record billing record in database
		b.recordBilling(apiKeyID, count)
	}
}

func (b *BillingSidecar) recordBilling(apiKeyID string, usageCount int) {
	// Simple pricing: $0.01 per API call
	amountCents := usageCount * 1

	query := `
		INSERT INTO billing_records (api_key_id, usage_count, amount_cents, billing_period_start, billing_period_end)
		VALUES ($1, $2, $3, $4, $5)`
	
	now := time.Now()
	periodStart := now.Truncate(time.Hour)
	periodEnd := periodStart.Add(time.Hour)
	
	_, err := b.db.Exec(query, apiKeyID, usageCount, amountCents, periodStart, periodEnd)
	if err != nil {
		log.Printf("Failed to record billing: %v", err)
	}
}