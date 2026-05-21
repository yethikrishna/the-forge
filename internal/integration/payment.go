// Package integration provides payment processing when authorized.
package integration

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// PaymentProvider identifies the payment provider.
type PaymentProvider string

const (
	PaymentStripe   PaymentProvider = "stripe"
	PaymentPayPal   PaymentProvider = "paypal"
	PaymentCrypto   PaymentProvider = "crypto"
)

// PaymentStatus is the status of a payment.
type PaymentStatus string

const (
	PaymentPending   PaymentStatus = "pending"
	PaymentCompleted PaymentStatus = "completed"
	PaymentFailed    PaymentStatus = "failed"
	PaymentRefunded  PaymentStatus = "refunded"
)

// Payment represents a payment transaction.
type Payment struct {
	ID          string          `json:"id"`
	Amount      float64         `json:"amount"`
	Currency    string          `json:"currency"`
	Provider    PaymentProvider `json:"provider"`
	Status      PaymentStatus   `json:"status"`
	Description string          `json:"description"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
	Error       string          `json:"error,omitempty"`
}

// PaymentConfig configures payment integration.
type PaymentConfig struct {
	Provider    PaymentProvider `json:"provider"`
	APIKey      string          `json:"api_key"`
	WebhookSecret string        `json:"webhook_secret,omitempty"`
	Currency    string          `json:"currency"`
	TestMode    bool            `json:"test_mode"`
}

// PaymentManager handles payment operations.
type PaymentManager struct {
	config   PaymentConfig
	payments map[string]*Payment
	mu       sync.RWMutex
}

// NewPaymentManager creates a payment manager.
func NewPaymentManager(config PaymentConfig) *PaymentManager {
	if config.Currency == "" {
		config.Currency = "USD"
	}
	return &PaymentManager{
		config:   config,
		payments: make(map[string]*Payment),
	}
}

// Charge creates a payment charge.
func (pm *PaymentManager) Charge(amount float64, description string, metadata map[string]string) (*Payment, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("payment: amount must be positive")
	}
	if amount > 10000 {
		return nil, fmt.Errorf("payment: amount exceeds $10,000 limit without explicit authorization")
	}

	payment := &Payment{
		ID:          fmt.Sprintf("pay-%d", time.Now().UnixNano()),
		Amount:      amount,
		Currency:    pm.config.Currency,
		Provider:    pm.config.Provider,
		Status:      PaymentPending,
		Description: description,
		Metadata:    metadata,
		CreatedAt:   time.Now(),
	}

	// In production: call provider API
	if pm.config.TestMode {
		payment.Status = PaymentCompleted
		now := time.Now()
		payment.CompletedAt = &now
	}

	pm.mu.Lock()
	pm.payments[payment.ID] = payment
	pm.mu.Unlock()

	return payment, nil
}

// Refund refunds a payment.
func (pm *PaymentManager) Refund(paymentID string) (*Payment, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	payment, ok := pm.payments[paymentID]
	if !ok {
		return nil, fmt.Errorf("payment %s not found", paymentID)
	}

	if payment.Status != PaymentCompleted {
		return nil, fmt.Errorf("payment %s is not completed (status: %s)", paymentID, payment.Status)
	}

	payment.Status = PaymentRefunded
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

// List returns all payments.
func (pm *PaymentManager) List() []*Payment {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	result := make([]*Payment, 0, len(pm.payments))
	for _, p := range pm.payments {
		result = append(result, p)
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

var _ = json.Marshal
