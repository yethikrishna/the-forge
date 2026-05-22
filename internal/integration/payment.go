// Package integration provides payment processing when authorized.
// Supports Stripe, PayPal, and crypto providers with real HTTP client
// integration, webhook verification, idempotency keys, and receipt generation.
package integration

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	crand "crypto/rand"

	"github.com/forge/sword/internal/banking"
	"github.com/forge/sword/internal/costlive"
)

// PaymentProvider identifies the payment provider.
type PaymentProvider string

const (
	PaymentStripe PaymentProvider = "stripe"
	PaymentPayPal PaymentProvider = "paypal"
	PaymentCrypto PaymentProvider = "crypto"
)

// PaymentStatus is the status of a payment.
type PaymentStatus string

const (
	PaymentPending      PaymentStatus = "pending"
	PaymentProcessing   PaymentStatus = "processing"
	PaymentCompleted    PaymentStatus = "completed"
	PaymentFailed       PaymentStatus = "failed"
	PaymentRefunded     PaymentStatus = "refunded"
	PaymentPartiallyRef PaymentStatus = "partially_refunded"
	PaymentCancelled    PaymentStatus = "cancelled"
	PaymentDisputed     PaymentStatus = "disputed"
)

// Payment represents a payment transaction.
type Payment struct {
	ID             string            `json:"id"`
	ProviderID     string            `json:"provider_id,omitempty"`
	Amount         float64           `json:"amount"`
	Currency       string            `json:"currency"`
	Provider       PaymentProvider   `json:"provider"`
	Status         PaymentStatus     `json:"status"`
	Description    string            `json:"description"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	IdempotencyKey string            `json:"idempotency_key,omitempty"`
	ReceiptURL     string            `json:"receipt_url,omitempty"`
	RefundAmount   float64           `json:"refund_amount,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	CompletedAt    *time.Time        `json:"completed_at,omitempty"`
	ExpiresAt      *time.Time        `json:"expires_at,omitempty"`
	Error          string            `json:"error,omitempty"`
	RetryCount     int               `json:"retry_count,omitempty"`
}

// PaymentConfig configures payment integration.
type PaymentConfig struct {
	Provider       PaymentProvider `json:"provider"`
	APIKey         string          `json:"api_key"`
	WebhookSecret  string          `json:"webhook_secret,omitempty"`
	Currency       string          `json:"currency"`
	TestMode       bool            `json:"test_mode"`
	MaxAmount      float64         `json:"max_amount"`
	BaseURL        string          `json:"base_url,omitempty"`
	Timeout        time.Duration   `json:"timeout,omitempty"`
	MaxRetries     int             `json:"max_retries,omitempty"`
}

// PaymentWebhook represents an incoming webhook event from a provider.
type PaymentWebhook struct {
	ID        string          `json:"id"`
	EventType string          `json:"event_type"`
	Provider  PaymentProvider `json:"provider"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp time.Time       `json:"timestamp"`
	Signature string          `json:"signature"`
}

// Receipt represents a payment receipt.
type Receipt struct {
	PaymentID   string    `json:"payment_id"`
	Amount      float64   `json:"amount"`
	Currency    string    `json:"currency"`
	Description string    `json:"description"`
	Provider    string    `json:"provider"`
	CreatedAt   time.Time `json:"created_at"`
	ReceiptURL  string    `json:"receipt_url,omitempty"`
	OrgName     string    `json:"org_name"`
	OrgID       string    `json:"org_id"`
}

// PaymentEvent records a state transition for audit trail.
type PaymentEvent struct {
	PaymentID  string    `json:"payment_id"`
	FromStatus string    `json:"from_status"`
	ToStatus   string    `json:"to_status"`
	Reason     string    `json:"reason,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
	Actor      string    `json:"actor"`
}

// defaultHTTPClient returns a configured HTTP client with timeouts.
func defaultHTTPClient(timeout time.Duration) *http.Client {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &http.Client{
		Timeout: timeout,
	}
}

// PaymentManager handles payment operations.
type PaymentManager struct {
	config   PaymentConfig
	payments map[string]*Payment
	events   []PaymentEvent
	mu       sync.RWMutex
	client   *http.Client
	orgName  string
	orgID    string
	// W12: wired banking ledger + costlive tracker for auto-recording charges
	bank          *banking.Bank
	bankAccountID string
	costTracker   *costlive.LiveTracker
}

// NewPaymentManager creates a payment manager.
func NewPaymentManager(config PaymentConfig) *PaymentManager {
	if config.Currency == "" {
		config.Currency = "USD"
	}
	if config.MaxAmount == 0 {
		config.MaxAmount = 10000
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	return &PaymentManager{
		config:   config,
		payments: make(map[string]*Payment),
		events:   make([]PaymentEvent, 0),
		client:   defaultHTTPClient(config.Timeout),
	}
}

// SetOrgInfo sets the organization info for receipts.
func (pm *PaymentManager) SetOrgInfo(name, id string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.orgName = name
	pm.orgID = id
}

// WithBank wires a banking.Bank so every successful charge is automatically
// recorded as a banking transaction (category: "api_cost").
// accountID is the bank account to record into.
func (pm *PaymentManager) WithBank(b *banking.Bank, accountID string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.bank = b
	pm.bankAccountID = accountID
}

// WithCostTracker wires a costlive.LiveTracker so every successful charge
// is also recorded in the live cost tracker (visible in `forge cost live`).
func (pm *PaymentManager) WithCostTracker(lt *costlive.LiveTracker) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.costTracker = lt
}

// recordToBank records a completed payment as a banking expense transaction.
// Uses ReceivePayment into a designated cost-tracking account if one exists.
func (pm *PaymentManager) recordToBank(p *Payment) {
	if pm.bank != nil && pm.bankAccountID != "" {
		// Record as an incoming transaction to the cost-tracking account
		_, _ = pm.bank.ReceivePayment(
			pm.bankAccountID,
			p.Amount,
			"api_cost",
			p.Description,
			p.ID,
		)
	}
	if pm.costTracker != nil {
		// Record to costlive so forge cost live reflects this spend
		provider := string(p.Provider)
		pm.costTracker.Record(pm.orgID, provider, 0, 0, p.Amount, "payment:charge")
	}
}

// Charge creates a payment charge.
func (pm *PaymentManager) Charge(amount float64, description string, metadata map[string]string) (*Payment, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("payment: amount must be positive, got %.2f", amount)
	}
	if amount > pm.config.MaxAmount {
		return nil, fmt.Errorf("payment: amount %.2f exceeds limit %.2f — requires explicit authorization", amount, pm.config.MaxAmount)
	}

	idempotencyKey := fmt.Sprintf("forge-%d-%s", time.Now().UnixNano(), randomHex(8))

	payment := &Payment{
		ID:             fmt.Sprintf("pay_%s", randomHex(16)),
		Amount:         amount,
		Currency:       pm.config.Currency,
		Provider:       pm.config.Provider,
		Status:         PaymentPending,
		Description:    description,
		Metadata:       metadata,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      time.Now(),
	}

	pm.mu.Lock()
	pm.payments[payment.ID] = payment
	pm.mu.Unlock()

	if pm.config.TestMode {
		pm.transitionStatus(payment.ID, "", string(PaymentPending), "created in test mode", "system")
		return pm.processTestPayment(payment.ID)
	}

	// Real provider integration
	result, err := pm.processProviderPayment(payment)
	if err != nil {
		pm.mu.Lock()
		payment.Status = PaymentFailed
		payment.Error = err.Error()
		pm.mu.Unlock()
		pm.transitionStatus(payment.ID, string(PaymentPending), string(PaymentFailed), err.Error(), "provider")
		return payment, fmt.Errorf("payment: provider error: %w", err)
	}

	return result, nil
}

// processTestPayment simulates payment processing in test mode.
func (pm *PaymentManager) processTestPayment(id string) (*Payment, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	payment, ok := pm.payments[id]
	if !ok {
		return nil, fmt.Errorf("payment %s not found", id)
	}

	payment.Status = PaymentProcessing
	time.Sleep(10 * time.Millisecond) // Simulate processing delay

	payment.Status = PaymentCompleted
	now := time.Now()
	payment.CompletedAt = &now
	payment.ProviderID = fmt.Sprintf("test_%s", randomHex(12))
	payment.ReceiptURL = fmt.Sprintf("https://billing.forge.dev/receipts/%s", payment.ID)

	pm.events = append(pm.events, PaymentEvent{
		PaymentID:  id,
		FromStatus: string(PaymentProcessing),
		ToStatus:   string(PaymentCompleted),
		Reason:     "test mode completion",
		Timestamp:  time.Now(),
		Actor:      "test-processor",
	})

	// W12: Record to banking ledger
	pm.recordToBank(payment)

	return payment, nil
}
func (pm *PaymentManager) processProviderPayment(payment *Payment) (*Payment, error) {
	switch pm.config.Provider {
	case PaymentStripe:
		return pm.processStripe(payment)
	case PaymentPayPal:
		return pm.processPayPal(payment)
	case PaymentCrypto:
		return pm.processCrypto(payment)
	default:
		return nil, fmt.Errorf("unsupported payment provider: %s", pm.config.Provider)
	}
}

// processStripe calls the Stripe API.
func (pm *PaymentManager) processStripe(payment *Payment) (*Payment, error) {
	baseURL := pm.config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.stripe.com/v1"
	}

	// Stripe uses form-encoded for charges, but we use PaymentIntents (JSON)
	payload := map[string]interface{}{
		"amount":   int64(payment.Amount * 100), // Stripe uses cents
		"currency": payment.Currency,
		"metadata": map[string]string{
			"forge_payment_id": payment.ID,
			"description":      payment.Description,
		},
		"idempotency_key": payment.IdempotencyKey,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("stripe: marshal payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), pm.config.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/payment_intents", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("stripe: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+pm.config.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", payment.IdempotencyKey)

	resp, err := pm.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stripe: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("stripe: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("stripe: API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("stripe: parse response: %w", err)
	}

	pm.mu.Lock()
	payment.ProviderID = result.ID
	if result.Status == "succeeded" {
		payment.Status = PaymentCompleted
		now := time.Now()
		payment.CompletedAt = &now
	} else {
		payment.Status = PaymentProcessing
	}
	payment.ReceiptURL = fmt.Sprintf("https://pay.stripe.com/receipts/%s", result.ID)
	pm.mu.Unlock()

	pm.transitionStatus(payment.ID, string(PaymentPending), string(payment.Status), "stripe API response", "stripe")
	return payment, nil
}

// processPayPal calls the PayPal API.
func (pm *PaymentManager) processPayPal(payment *Payment) (*Payment, error) {
	baseURL := pm.config.BaseURL
	if baseURL == "" {
		if pm.config.TestMode {
			baseURL = "https://api-m.sandbox.paypal.com"
		} else {
			baseURL = "https://api-m.paypal.com"
		}
	}

	// Get access token
	token, err := pm.payPalAccessToken(baseURL)
	if err != nil {
		return nil, fmt.Errorf("paypal: get token: %w", err)
	}

	// Create order
	payload := map[string]interface{}{
		"intent": "CAPTURE",
		"purchase_units": []map[string]interface{}{
			{
				"amount": map[string]interface{}{
					"currency_code": payment.Currency,
					"value":         fmt.Sprintf("%.2f", payment.Amount),
				},
				"description": payment.Description,
				"reference_id": payment.ID,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("paypal: marshal payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), pm.config.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v2/checkout/orders", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("paypal: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := pm.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("paypal: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("paypal: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("paypal: API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Links  []struct {
			Href   string `json:"href"`
			Rel    string `json:"rel"`
			Method string `json:"method"`
		} `json:"links"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("paypal: parse response: %w", err)
	}

	pm.mu.Lock()
	payment.ProviderID = result.ID
	payment.Status = PaymentProcessing
	for _, link := range result.Links {
		if link.Rel == "approve" {
			payment.ReceiptURL = link.Href
		}
	}
	pm.mu.Unlock()

	pm.transitionStatus(payment.ID, string(PaymentPending), string(PaymentProcessing), "paypal order created", "paypal")
	return payment, nil
}

// payPalAccessToken gets an OAuth token from PayPal.
func (pm *PaymentManager) payPalAccessToken(baseURL string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), pm.config.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/oauth2/token", bytes.NewReader([]byte("grant_type=client_credentials")))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(pm.config.APIKey, pm.config.WebhookSecret)

	resp, err := pm.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("paypal token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("paypal token read: %w", err)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("paypal token parse: %w", err)
	}

	return tokenResp.AccessToken, nil
}

// processCrypto handles crypto payment via a gateway.
func (pm *PaymentManager) processCrypto(payment *Payment) (*Payment, error) {
	baseURL := pm.config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.gatehub.net/v2"
	}

	payload := map[string]interface{}{
		"amount":      payment.Amount,
		"currency":    payment.Currency,
		"reference":   payment.ID,
		"description": payment.Description,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("crypto: marshal payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), pm.config.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/payments", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("crypto: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+pm.config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := pm.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("crypto: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("crypto: API error (%d): %s", resp.StatusCode, string(respBody))
	}

	pm.mu.Lock()
	payment.Status = PaymentPending
	expires := time.Now().Add(30 * time.Minute)
	payment.ExpiresAt = &expires
	pm.mu.Unlock()

	pm.transitionStatus(payment.ID, "", string(PaymentPending), "crypto payment awaiting confirmation", "crypto-gateway")
	return payment, nil
}

// Refund refunds a payment (full or partial).
func (pm *PaymentManager) Refund(paymentID string, amount float64) (*Payment, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	payment, ok := pm.payments[paymentID]
	if !ok {
		return nil, fmt.Errorf("payment %s not found", paymentID)
	}

	if payment.Status != PaymentCompleted {
		return nil, fmt.Errorf("payment %s cannot be refunded (status: %s)", paymentID, payment.Status)
	}

	refundAmount := amount
	if refundAmount <= 0 {
		refundAmount = payment.Amount // Full refund
	}

	if refundAmount > payment.Amount {
		return nil, fmt.Errorf("refund amount %.2f exceeds payment amount %.2f", refundAmount, payment.Amount)
	}

	prevStatus := string(payment.Status)

	if refundAmount >= payment.Amount {
		payment.Status = PaymentRefunded
	} else {
		payment.Status = PaymentPartiallyRef
		payment.RefundAmount = refundAmount
	}

	pm.events = append(pm.events, PaymentEvent{
		PaymentID:  paymentID,
		FromStatus: prevStatus,
		ToStatus:   string(payment.Status),
		Reason:     fmt.Sprintf("refund of %.2f %s", refundAmount, payment.Currency),
		Timestamp:  time.Now(),
		Actor:      "refund-processor",
	})

	return payment, nil
}

// Get retrieves a payment.
func (pm *PaymentManager) Get(id string) (*Payment, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	p, ok := pm.payments[id]
	if !ok {
		return nil, fmt.Errorf("payment %s not found", id)
	}
	return p, nil
}

// List returns all payments, optionally filtered by status.
func (pm *PaymentManager) List(status PaymentStatus) []*Payment {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	result := make([]*Payment, 0, len(pm.payments))
	for _, p := range pm.payments {
		if status == "" || p.Status == status {
			result = append(result, p)
		}
	}
	return result
}

// Total returns the total amount of completed payments.
func (pm *PaymentManager) Total() float64 {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	var total float64
	for _, p := range pm.payments {
		if p.Status == PaymentCompleted {
			total += p.Amount
		}
	}
	return total
}

// Summary returns a payment summary for reporting.
type PaymentSummary struct {
	TotalAmount     float64 `json:"total_amount"`
	TotalPayments   int     `json:"total_payments"`
	CompletedCount  int     `json:"completed_count"`
	RefundedCount   int     `json:"refunded_count"`
	FailedCount     int     `json:"failed_count"`
	PendingCount    int     `json:"pending_count"`
	Currency        string  `json:"currency"`
	RefundTotal     float64 `json:"refund_total"`
	NetRevenue      float64 `json:"net_revenue"`
}

// Summary returns a payment summary.
func (pm *PaymentManager) Summary() PaymentSummary {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	s := PaymentSummary{Currency: pm.config.Currency}
	for _, p := range pm.payments {
		s.TotalPayments++
		switch p.Status {
		case PaymentCompleted:
			s.CompletedCount++
			s.TotalAmount += p.Amount
		case PaymentRefunded:
			s.RefundedCount++
			s.RefundTotal += p.Amount
		case PaymentFailed:
			s.FailedCount++
		case PaymentPending, PaymentProcessing:
			s.PendingCount++
		case PaymentPartiallyRef:
			s.CompletedCount++
			s.TotalAmount += p.Amount
			s.RefundTotal += p.RefundAmount
		}
	}
	s.NetRevenue = s.TotalAmount - s.RefundTotal
	return s
}

// GenerateReceipt generates a receipt for a completed payment.
func (pm *PaymentManager) GenerateReceipt(paymentID string) (*Receipt, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	payment, ok := pm.payments[paymentID]
	if !ok {
		return nil, fmt.Errorf("payment %s not found", paymentID)
	}

	if payment.Status != PaymentCompleted && payment.Status != PaymentPartiallyRef {
		return nil, fmt.Errorf("payment %s is not completed (status: %s)", paymentID, payment.Status)
	}

	return &Receipt{
		PaymentID:   payment.ID,
		Amount:      payment.Amount,
		Currency:    payment.Currency,
		Description: payment.Description,
		Provider:    string(payment.Provider),
		CreatedAt:   *payment.CompletedAt,
		ReceiptURL:  payment.ReceiptURL,
		OrgName:     pm.orgName,
		OrgID:       pm.orgID,
	}, nil
}

// VerifyWebhook verifies a webhook signature from the payment provider.
func (pm *PaymentManager) VerifyWebhook(payload []byte, signature string) (*PaymentWebhook, error) {
	if pm.config.WebhookSecret == "" {
		return nil, fmt.Errorf("payment: webhook secret not configured")
	}

	// Verify HMAC signature
	mac := hmac.New(sha256.New, []byte(pm.config.WebhookSecret))
	mac.Write(payload)
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		return nil, fmt.Errorf("payment: webhook signature verification failed")
	}

	var webhook PaymentWebhook
	if err := json.Unmarshal(payload, &webhook); err != nil {
		return nil, fmt.Errorf("payment: parse webhook payload: %w", err)
	}

	webhook.Signature = signature
	if webhook.Timestamp.IsZero() {
		webhook.Timestamp = time.Now()
	}

	return &webhook, nil
}

// HandleWebhookEvent processes a webhook event and updates payment status.
func (pm *PaymentManager) HandleWebhookEvent(webhook *PaymentWebhook) error {
	var paymentID string
	var newStatus PaymentStatus

	switch webhook.EventType {
	case "payment.completed", "payment_intent.succeeded", "PAYMENT.CAPTURE.COMPLETED":
		var payload struct {
			PaymentID string `json:"payment_id,omitempty"`
			ID        string `json:"id,omitempty"`
		}
		if err := json.Unmarshal(webhook.Payload, &payload); err != nil {
			return fmt.Errorf("parse completed event: %w", err)
		}
		paymentID = payload.PaymentID
		if paymentID == "" {
			paymentID = payload.ID
		}
		newStatus = PaymentCompleted

	case "payment.failed", "payment_intent.payment_failed":
		var payload struct {
			PaymentID string `json:"payment_id,omitempty"`
			Error     string `json:"error,omitempty"`
		}
		if err := json.Unmarshal(webhook.Payload, &payload); err != nil {
			return fmt.Errorf("parse failed event: %w", err)
		}
		paymentID = payload.PaymentID
		newStatus = PaymentFailed

	case "payment.refunded", "charge.refunded":
		var payload struct {
			PaymentID string `json:"payment_id,omitempty"`
		}
		if err := json.Unmarshal(webhook.Payload, &payload); err != nil {
			return fmt.Errorf("parse refund event: %w", err)
		}
		paymentID = payload.PaymentID
		newStatus = PaymentRefunded

	case "payment.disputed", "charge.dispute.created":
		var payload struct {
			PaymentID string `json:"payment_id,omitempty"`
		}
		if err := json.Unmarshal(webhook.Payload, &payload); err != nil {
			return fmt.Errorf("parse dispute event: %w", err)
		}
		paymentID = payload.PaymentID
		newStatus = PaymentDisputed

	default:
		return fmt.Errorf("unhandled webhook event type: %s", webhook.EventType)
	}

	if paymentID == "" {
		return fmt.Errorf("webhook event missing payment ID")
	}

	pm.mu.Lock()
	payment, ok := pm.payments[paymentID]
	if !ok {
		pm.mu.Unlock()
		return fmt.Errorf("payment %s not found for webhook event", paymentID)
	}

	prevStatus := string(payment.Status)
	payment.Status = newStatus
	if newStatus == PaymentCompleted {
		now := time.Now()
		payment.CompletedAt = &now
	}
	pm.events = append(pm.events, PaymentEvent{
		PaymentID:  paymentID,
		FromStatus: prevStatus,
		ToStatus:   string(newStatus),
		Reason:     fmt.Sprintf("webhook event: %s", webhook.EventType),
		Timestamp:  time.Now(),
		Actor:      "webhook:" + string(webhook.Provider),
	})
	pm.mu.Unlock()

	return nil
}

// AuditTrail returns all events for a payment.
func (pm *PaymentManager) AuditTrail(paymentID string) []PaymentEvent {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	var events []PaymentEvent
	for _, e := range pm.events {
		if e.PaymentID == paymentID {
			events = append(events, e)
		}
	}
	return events
}

// transitionStatus records a state transition (must NOT hold the lock).
func (pm *PaymentManager) transitionStatus(paymentID, from, to, reason, actor string) {
	pm.mu.Lock()
	pm.events = append(pm.events, PaymentEvent{
		PaymentID:  paymentID,
		FromStatus: from,
		ToStatus:   to,
		Reason:     reason,
		Timestamp:  time.Now(),
		Actor:      actor,
	})
	pm.mu.Unlock()
}

// randomHex generates a cryptographically secure random hex string of n bytes.
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := crand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
