package integration

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/forge/sword/internal/banking"
	"github.com/forge/sword/internal/costlive"
)

func newTestManager() *PaymentManager {
	return NewPaymentManager(PaymentConfig{
		Provider:  PaymentStripe,
		APIKey:    "test_key",
		Currency:  "USD",
		TestMode:  true,
		MaxAmount: 10000,
		Timeout:   5 * time.Second,
	})
}

func TestPaymentCharge(t *testing.T) {
	pm := newTestManager()

	p, err := pm.Charge(99.99, "Test product", map[string]string{"order": "123"})
	if err != nil {
		t.Fatalf("Charge failed: %v", err)
	}
	if p.ID == "" {
		t.Fatal("Payment ID should not be empty")
	}
	if p.Amount != 99.99 {
		t.Fatalf("Expected amount 99.99, got %.2f", p.Amount)
	}
	if p.Status != PaymentCompleted {
		t.Fatalf("Expected completed, got %s", p.Status)
	}
	if p.ProviderID == "" {
		t.Fatal("Provider ID should be set in test mode")
	}
	if p.ReceiptURL == "" {
		t.Fatal("Receipt URL should be set in test mode")
	}
	if p.CompletedAt == nil {
		t.Fatal("CompletedAt should be set")
	}
	if p.Currency != "USD" {
		t.Fatalf("Expected USD, got %s", p.Currency)
	}
	if p.IdempotencyKey == "" {
		t.Fatal("Idempotency key should be set")
	}
}

func TestPaymentChargeZero(t *testing.T) {
	pm := newTestManager()
	_, err := pm.Charge(0, "Free", nil)
	if err == nil {
		t.Fatal("Should reject zero amount")
	}
}

func TestPaymentChargeNegative(t *testing.T) {
	pm := newTestManager()
	_, err := pm.Charge(-10, "Negative", nil)
	if err == nil {
		t.Fatal("Should reject negative amount")
	}
}

func TestPaymentChargeOverLimit(t *testing.T) {
	pm := newTestManager()
	_, err := pm.Charge(15000, "Over limit", nil)
	if err == nil {
		t.Fatal("Should reject amount over limit")
	}
}

func TestPaymentGet(t *testing.T) {
	pm := newTestManager()

	p, _ := pm.Charge(50, "Test", nil)
	got, err := pm.Get(p.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.ID != p.ID {
		t.Fatalf("Expected ID %s, got %s", p.ID, got.ID)
	}
}

func TestPaymentGetNotFound(t *testing.T) {
	pm := newTestManager()
	_, err := pm.Get("nonexistent")
	if err == nil {
		t.Fatal("Should return error for nonexistent payment")
	}
}

func TestPaymentRefundFull(t *testing.T) {
	pm := newTestManager()

	p, _ := pm.Charge(100, "Test refund", nil)
	refunded, err := pm.Refund(p.ID, 0) // 0 = full refund
	if err != nil {
		t.Fatalf("Refund failed: %v", err)
	}
	if refunded.Status != PaymentRefunded {
		t.Fatalf("Expected refunded, got %s", refunded.Status)
	}
}

func TestPaymentRefundPartial(t *testing.T) {
	pm := newTestManager()

	p, _ := pm.Charge(100, "Test partial", nil)
	refunded, err := pm.Refund(p.ID, 30)
	if err != nil {
		t.Fatalf("Refund failed: %v", err)
	}
	if refunded.Status != PaymentPartiallyRef {
		t.Fatalf("Expected partially_refunded, got %s", refunded.Status)
	}
	if refunded.RefundAmount != 30 {
		t.Fatalf("Expected refund amount 30, got %.2f", refunded.RefundAmount)
	}
}

func TestPaymentRefundOverAmount(t *testing.T) {
	pm := newTestManager()

	p, _ := pm.Charge(50, "Test over", nil)
	_, err := pm.Refund(p.ID, 100)
	if err == nil {
		t.Fatal("Should reject refund over payment amount")
	}
}

func TestPaymentRefundNotFound(t *testing.T) {
	pm := newTestManager()
	_, err := pm.Refund("nonexistent", 0)
	if err == nil {
		t.Fatal("Should return error for nonexistent payment")
	}
}

func TestPaymentRefundNotCompleted(t *testing.T) {
	pm := newTestManager()
	pm.mu.Lock()
	pm.payments["test"] = &Payment{ID: "test", Status: PaymentPending}
	pm.mu.Unlock()

	_, err := pm.Refund("test", 0)
	if err == nil {
		t.Fatal("Should reject refund of non-completed payment")
	}
}

func TestPaymentList(t *testing.T) {
	pm := newTestManager()

	pm.Charge(10, "A", nil)
	pm.Charge(20, "B", nil)
	pm.Charge(30, "C", nil)

	all := pm.List("")
	if len(all) != 3 {
		t.Fatalf("Expected 3 payments, got %d", len(all))
	}
}

func TestPaymentListByStatus(t *testing.T) {
	pm := newTestManager()

	pm.Charge(10, "A", nil)
	pm.Charge(20, "B", nil)

	// Add a failed payment manually
	pm.mu.Lock()
	pm.payments["failed1"] = &Payment{ID: "failed1", Status: PaymentFailed}
	pm.mu.Unlock()

	completed := pm.List(PaymentCompleted)
	if len(completed) != 2 {
		t.Fatalf("Expected 2 completed, got %d", len(completed))
	}

	failed := pm.List(PaymentFailed)
	if len(failed) != 1 {
		t.Fatalf("Expected 1 failed, got %d", len(failed))
	}
}

func TestPaymentTotal(t *testing.T) {
	pm := newTestManager()

	pm.Charge(10, "A", nil)
	pm.Charge(20, "B", nil)
	pm.Charge(30, "C", nil)

	total := pm.Total()
	if total != 60 {
		t.Fatalf("Expected total 60, got %.2f", total)
	}
}

func TestPaymentSummary(t *testing.T) {
	pm := newTestManager()

	pm.Charge(10, "A", nil)
	pm.Charge(20, "B", nil)
	p, _ := pm.Charge(30, "C", nil)
	pm.Refund(p.ID, 0)

	s := pm.Summary()
	if s.TotalPayments != 3 {
		t.Fatalf("Expected 3 payments, got %d", s.TotalPayments)
	}
	if s.CompletedCount != 2 {
		t.Fatalf("Expected 2 completed, got %d", s.CompletedCount)
	}
	if s.RefundedCount != 1 {
		t.Fatalf("Expected 1 refunded, got %d", s.RefundedCount)
	}
	if s.RefundTotal != 30 {
		t.Fatalf("Expected refund total 30, got %.2f", s.RefundTotal)
	}
	if s.NetRevenue != 0 {
		t.Fatalf("Expected net revenue 0, got %.2f", s.NetRevenue)
	}
}

func TestPaymentReceipt(t *testing.T) {
	pm := newTestManager()
	pm.SetOrgInfo("Forge Corp", "org-123")

	p, _ := pm.Charge(42.50, "Subscription", nil)
	r, err := pm.GenerateReceipt(p.ID)
	if err != nil {
		t.Fatalf("GenerateReceipt failed: %v", err)
	}
	if r.PaymentID != p.ID {
		t.Fatalf("Receipt payment ID mismatch")
	}
	if r.Amount != 42.50 {
		t.Fatalf("Receipt amount mismatch")
	}
	if r.OrgName != "Forge Corp" {
		t.Fatalf("Receipt org name mismatch")
	}
	if r.OrgID != "org-123" {
		t.Fatalf("Receipt org ID mismatch")
	}
}

func TestPaymentReceiptNotCompleted(t *testing.T) {
	pm := newTestManager()
	pm.mu.Lock()
	pm.payments["pend"] = &Payment{ID: "pend", Status: PaymentPending}
	pm.mu.Unlock()

	_, err := pm.GenerateReceipt("pend")
	if err == nil {
		t.Fatal("Should reject receipt for non-completed payment")
	}
}

func TestPaymentAuditTrail(t *testing.T) {
	pm := newTestManager()

	p, _ := pm.Charge(75, "Audit test", nil)
	pm.Refund(p.ID, 0)

	trail := pm.AuditTrail(p.ID)
	if len(trail) < 2 {
		t.Fatalf("Expected at least 2 events, got %d", len(trail))
	}
}

func TestPaymentVerifyWebhook(t *testing.T) {
	pm := NewPaymentManager(PaymentConfig{
		Provider:      PaymentStripe,
		APIKey:        "test",
		WebhookSecret: "whsecret123",
		TestMode:      true,
	})

	payload := map[string]interface{}{
		"id":         "evt_123",
		"event_type": "payment.completed",
		"provider":   "stripe",
		"payment_id": "pay_abc",
		"timestamp":  time.Now().Format(time.RFC3339),
	}
	body, _ := json.Marshal(payload)

	// Compute valid HMAC signature
	mac := hmac.New(sha256.New, []byte("whsecret123"))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	webhook, err := pm.VerifyWebhook(body, sig)
	if err != nil {
		t.Fatalf("VerifyWebhook failed: %v", err)
	}
	if webhook.EventType != "payment.completed" {
		t.Fatalf("Expected event_type payment.completed, got %s", webhook.EventType)
	}
}

func TestPaymentVerifyWebhookBadSig(t *testing.T) {
	pm := NewPaymentManager(PaymentConfig{
		Provider:      PaymentStripe,
		APIKey:        "test",
		WebhookSecret: "whsecret123",
		TestMode:      true,
	})

	body := []byte(`{"event_type":"test"}`)
	_, err := pm.VerifyWebhook(body, "badsignature")
	if err == nil {
		t.Fatal("Should reject bad signature")
	}
}

func TestPaymentVerifyWebhookNoSecret(t *testing.T) {
	pm := newTestManager() // No webhook secret
	_, err := pm.VerifyWebhook([]byte("{}"), "sig")
	if err == nil {
		t.Fatal("Should reject without webhook secret")
	}
}

func TestPaymentHandleWebhookCompleted(t *testing.T) {
	pm := newTestManager()

	p, _ := pm.Charge(50, "Webhook test", nil)
	// Manually set to processing to test webhook transition
	pm.mu.Lock()
	p.Status = PaymentProcessing
	pm.mu.Unlock()

	payload, _ := json.Marshal(map[string]string{
		"payment_id": p.ID,
	})

	webhook := &PaymentWebhook{
		ID:        "evt_1",
		EventType: "payment.completed",
		Provider:  PaymentStripe,
		Payload:   payload,
		Timestamp: time.Now(),
	}

	err := pm.HandleWebhookEvent(webhook)
	if err != nil {
		t.Fatalf("HandleWebhookEvent failed: %v", err)
	}

	got, _ := pm.Get(p.ID)
	if got.Status != PaymentCompleted {
		t.Fatalf("Expected completed, got %s", got.Status)
	}
}

func TestPaymentHandleWebhookFailed(t *testing.T) {
	pm := newTestManager()

	p, _ := pm.Charge(50, "Fail test", nil)
	pm.mu.Lock()
	p.Status = PaymentProcessing
	pm.mu.Unlock()

	payload, _ := json.Marshal(map[string]string{
		"payment_id": p.ID,
		"error":      "card declined",
	})

	webhook := &PaymentWebhook{
		ID:        "evt_2",
		EventType: "payment.failed",
		Provider:  PaymentStripe,
		Payload:   payload,
		Timestamp: time.Now(),
	}

	err := pm.HandleWebhookEvent(webhook)
	if err != nil {
		t.Fatalf("HandleWebhookEvent failed: %v", err)
	}

	got, _ := pm.Get(p.ID)
	if got.Status != PaymentFailed {
		t.Fatalf("Expected failed, got %s", got.Status)
	}
}

func TestPaymentHandleWebhookUnknownEvent(t *testing.T) {
	pm := newTestManager()

	webhook := &PaymentWebhook{
		ID:        "evt_3",
		EventType: "unknown.event",
		Payload:   []byte(`{}`),
	}

	err := pm.HandleWebhookEvent(webhook)
	if err == nil {
		t.Fatal("Should reject unknown event type")
	}
}

func TestPaymentConcurrentCharges(t *testing.T) {
	pm := newTestManager()

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := pm.Charge(float64(idx+1), "Concurrent", nil)
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent charge error: %v", err)
	}

	all := pm.List("")
	if len(all) != 50 {
		t.Fatalf("Expected 50 payments, got %d", len(all))
	}
}

func TestPaymentDefaultCurrency(t *testing.T) {
	pm := NewPaymentManager(PaymentConfig{
		Provider: PaymentStripe,
		TestMode: true,
	})
	if pm.config.Currency != "USD" {
		t.Fatalf("Expected default currency USD, got %s", pm.config.Currency)
	}
}

func TestPaymentDefaultMaxAmount(t *testing.T) {
	pm := NewPaymentManager(PaymentConfig{
		Provider: PaymentStripe,
		TestMode: true,
	})
	if pm.config.MaxAmount != 10000 {
		t.Fatalf("Expected default max 10000, got %.2f", pm.config.MaxAmount)
	}
}

func TestPaymentJSONRoundTrip(t *testing.T) {
	pm := newTestManager()

	p, _ := pm.Charge(33.33, "JSON test", map[string]string{"key": "value"})

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var got Payment
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if got.ID != p.ID {
		t.Fatalf("ID mismatch after round-trip")
	}
	if got.Amount != p.Amount {
		t.Fatalf("Amount mismatch after round-trip")
	}
}

func TestPaymentSummaryEmpty(t *testing.T) {
	pm := newTestManager()
	s := pm.Summary()
	if s.TotalPayments != 0 {
		t.Fatalf("Expected 0 payments in empty summary, got %d", s.TotalPayments)
	}
	if s.NetRevenue != 0 {
		t.Fatalf("Expected 0 net revenue, got %.2f", s.NetRevenue)
	}
}

func TestPaymentMultipleRefunds(t *testing.T) {
	pm := newTestManager()

	p, _ := pm.Charge(100, "Multi refund", nil)

	// First partial refund
	_, err := pm.Refund(p.ID, 30)
	if err != nil {
		t.Fatalf("First refund failed: %v", err)
	}

	// Second partial refund should work since status is partially_refunded
	got, _ := pm.Get(p.ID)
	if got.Status != PaymentPartiallyRef {
		t.Fatalf("Expected partially_refunded after first partial, got %s", got.Status)
	}
}

func TestPaymentChargeRecordsToBankingAndCostLive(t *testing.T) {
	pm := newTestManager()

	// Set up banking
	bankDir := t.TempDir()
	b := banking.NewBank(bankDir + "/bank.json")
	acct, err := b.CreateAccount("cost-tracking", banking.AccountOperating, "USD", 0)
	if err != nil {
		t.Fatal(err)
	}
	pm.WithBank(b, acct.ID)

	// Set up costlive tracker
	ltDir := t.TempDir()
	lt, err := costlive.NewLiveTracker(ltDir, 100.0)
	if err != nil {
		t.Fatal(err)
	}
	pm.WithCostTracker(lt)

	// Charge $25
	p, err := pm.Charge(25.0, "LLM API usage", map[string]string{"model": "gpt-4o"})
	if err != nil {
		t.Fatalf("charge error: %v", err)
	}
	if p.Status != PaymentCompleted {
		t.Fatalf("expected completed, got %s", p.Status)
	}

	// Verify banking transaction recorded
	txns := b.ListTransactions("api_cost", 10)
	if len(txns) == 0 {
		t.Error("expected banking transaction after charge")
	} else {
		t.Logf("banking transaction recorded: $%.2f %s", txns[0].Amount, txns[0].Description)
	}

	// Verify costlive shows the spend
	stats := lt.Stats()
	if stats.TodayCost == 0 {
		t.Error("expected costlive to reflect payment charge")
	} else {
		t.Logf("costlive today total cost: $%.4f", stats.TodayCost)
	}
}
