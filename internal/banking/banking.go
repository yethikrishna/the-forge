// Package banking provides autonomous financial operations:
// hold money, make payments, receive revenue, budget planning.
package banking

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// AccountType categorizes financial accounts.
type AccountType string

const (
	AccountOperating AccountType = "operating"
	AccountReserve   AccountType = "reserve"
	AccountRevenue   AccountType = "revenue"
	AccountBudget    AccountType = "budget"
)

// TxType categorizes transactions.
type TxType string

const (
	TxDebit  TxType = "debit"
	TxCredit TxType = "credit"
)

// Account represents a financial account.
type Account struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Type      AccountType `json:"type"`
	Balance   float64     `json:"balance"`
	Currency  string      `json:"currency"`
	CreatedAt time.Time   `json:"created_at"`
}

// Transaction records a financial movement.
type Transaction struct {
	ID          string    `json:"id"`
	FromAccount string   `json:"from_account"`
	ToAccount   string   `json:"to_account"`
	Amount      float64  `json:"amount"`
	Currency    string   `json:"currency"`
	Type        TxType   `json:"type"`
	Category    string   `json:"category"` // api_cost, infra, revenue, salary, misc
	Description string   `json:"description"`
	Reference   string   `json:"reference,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// BudgetAllocation assigns budget to a division or purpose.
type BudgetAllocation struct {
	ID         string    `json:"id"`
	TargetID   string    `json:"target_id"` // division or purpose ID
	Amount     float64   `json:"amount"`
	Currency   string    `json:"currency"`
	Period     string    `json:"period"` // monthly, quarterly, yearly
	Spent      float64   `json:"spent"`
	Remaining  float64   `json:"remaining"`
	CreatedAt  time.Time `json:"created_at"`
}

// Bank manages financial operations.
type Bank struct {
	mu          sync.RWMutex
	accounts    map[string]*Account
	transactions []*Transaction
	allocations map[string]*BudgetAllocation
	path        string
}

// NewBank creates a new banking system.
func NewBank(persistPath string) *Bank {
	b := &Bank{
		accounts:    make(map[string]*Account),
		transactions: make([]*Transaction, 0),
		allocations: make(map[string]*BudgetAllocation),
		path:        persistPath,
	}
	b.load()
	return b
}

// CreateAccount creates a new financial account.
func (b *Bank) CreateAccount(name string, acctType AccountType, currency string, initialBalance float64) (*Account, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	acct := &Account{
		ID:        genID("acct"),
		Name:      name,
		Type:      acctType,
		Balance:   initialBalance,
		Currency:  currency,
		CreatedAt: time.Now().UTC(),
	}

	b.accounts[acct.ID] = acct
	b.persist()
	return acct, nil
}

// GetBalance returns the balance of an account.
func (b *Bank) GetBalance(accountID string) (float64, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	acct, ok := b.accounts[accountID]
	if !ok {
		return 0, fmt.Errorf("account %s not found", accountID)
	}
	return acct.Balance, nil
}

// Transfer moves money between accounts.
func (b *Bank) Transfer(from, to string, amount float64, category, description, reference string) (*Transaction, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	fromAcct, ok := b.accounts[from]
	if !ok {
		return nil, fmt.Errorf("source account %s not found", from)
	}
	toAcct, ok := b.accounts[to]
	if !ok {
		return nil, fmt.Errorf("destination account %s not found", to)
	}
	if fromAcct.Balance < amount {
		return nil, fmt.Errorf("insufficient funds: have %.2f, need %.2f", fromAcct.Balance, amount)
	}

	fromAcct.Balance -= amount
	toAcct.Balance += amount

	tx := &Transaction{
		ID:          genID("tx"),
		FromAccount: from,
		ToAccount:   to,
		Amount:      amount,
		Currency:    fromAcct.Currency,
		Type:        TxDebit,
		Category:    category,
		Description: description,
		Reference:   reference,
		Timestamp:   time.Now().UTC(),
	}

	b.transactions = append(b.transactions, tx)

	// Update budget allocation if applicable
	for _, alloc := range b.allocations {
		if alloc.TargetID == from {
			alloc.Spent += amount
			alloc.Remaining -= amount
		}
	}

	b.persist()
	return tx, nil
}

// ReceivePayment records incoming revenue.
func (b *Bank) ReceivePayment(toAccount string, amount float64, category, description, reference string) (*Transaction, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	acct, ok := b.accounts[toAccount]
	if !ok {
		return nil, fmt.Errorf("account %s not found", toAccount)
	}

	acct.Balance += amount

	tx := &Transaction{
		ID:          genID("tx"),
		FromAccount: "external",
		ToAccount:   toAccount,
		Amount:      amount,
		Currency:    acct.Currency,
		Type:        TxCredit,
		Category:    category,
		Description: description,
		Reference:   reference,
		Timestamp:   time.Now().UTC(),
	}

	b.transactions = append(b.transactions, tx)
	b.persist()
	return tx, nil
}

// AllocateBudget assigns budget to a division.
func (b *Bank) AllocateBudget(targetID string, amount float64, currency, period string) (*BudgetAllocation, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	alloc := &BudgetAllocation{
		ID:        genID("alloc"),
		TargetID:  targetID,
		Amount:    amount,
		Currency:  currency,
		Period:    period,
		Spent:     0,
		Remaining: amount,
		CreatedAt: time.Now().UTC(),
	}

	b.allocations[alloc.ID] = alloc
	b.persist()
	return alloc, nil
}

// ListTransactions returns recent transactions.
func (b *Bank) ListTransactions(category string, limit int) []*Transaction {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var result []*Transaction
	for i := len(b.transactions) - 1; i >= 0; i-- {
		tx := b.transactions[i]
		if category == "" || tx.Category == category {
			result = append(result, tx)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result
}

// FinancialSummary returns a financial overview.
type FinancialSummary struct {
	TotalBalance float64 `json:"total_balance"`
	Currency     string  `json:"currency"`
	Accounts     int     `json:"accounts"`
	TotalSpent   float64 `json:"total_spent"`
	TotalRevenue float64 `json:"total_revenue"`
	TxCount      int     `json:"tx_count"`
}

// GetSummary returns a financial summary.
func (b *Bank) GetSummary() *FinancialSummary {
	b.mu.RLock()
	defer b.mu.RUnlock()

	summary := &FinancialSummary{Accounts: len(b.accounts)}
	for _, acct := range b.accounts {
		summary.TotalBalance += acct.Balance
		summary.Currency = acct.Currency
	}
	for _, tx := range b.transactions {
		summary.TxCount++
		if tx.Type == TxDebit {
			summary.TotalSpent += tx.Amount
		} else {
			summary.TotalRevenue += tx.Amount
		}
	}
	return summary
}

func (b *Bank) persist() {
	if b.path == "" { return }
	data := struct {
		Accounts    map[string]*Account    `json:"accounts"`
		Transactions []*Transaction        `json:"transactions"`
		Allocations map[string]*BudgetAllocation `json:"allocations"`
	}{b.accounts, b.transactions, b.allocations}
	raw, _ := json.MarshalIndent(data, "", "  ")
	os.MkdirAll(filepath.Dir(b.path), 0755)
	os.WriteFile(b.path, raw, 0644)
}

func (b *Bank) load() {
	if b.path == "" { return }
	raw, err := os.ReadFile(b.path)
	if err != nil { return }
	var data struct {
		Accounts    map[string]*Account    `json:"accounts"`
		Transactions []*Transaction        `json:"transactions"`
		Allocations map[string]*BudgetAllocation `json:"allocations"`
	}
	if json.Unmarshal(raw, &data) == nil {
		if data.Accounts != nil { b.accounts = data.Accounts }
		if data.Transactions != nil { b.transactions = data.Transactions }
		if data.Allocations != nil { b.allocations = data.Allocations }
	}
}

func genID(prefix string) string { return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()) }

// Ensure sort import is used.
var _ = sort.Sort
