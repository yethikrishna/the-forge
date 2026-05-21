package banking

import (
	"path/filepath"
	"testing"
)

func TestCreateAccount(t *testing.T) {
	b := NewBank(filepath.Join(t.TempDir(), "bank.json"))

	acct, err := b.CreateAccount("Operating", AccountOperating, "USD", 10000)
	if err != nil {
		t.Fatal(err)
	}
	if acct.Balance != 10000 {
		t.Errorf("expected 10000, got %f", acct.Balance)
	}

	balance, _ := b.GetBalance(acct.ID)
	if balance != 10000 {
		t.Error("balance mismatch")
	}
}

func TestTransfer(t *testing.T) {
	b := NewBank(filepath.Join(t.TempDir(), "bank.json"))

	from, _ := b.CreateAccount("Operating", AccountOperating, "USD", 10000)
	to, _ := b.CreateAccount("Engineering", AccountBudget, "USD", 0)

	tx, err := b.Transfer(from.ID, to.ID, 3000, "budget", "Q3 engineering budget", "BUD-001")
	if err != nil {
		t.Fatal(err)
	}
	if tx.Amount != 3000 {
		t.Error("amount mismatch")
	}

	fromBal, _ := b.GetBalance(from.ID)
	if fromBal != 7000 {
		t.Errorf("expected 7000, got %f", fromBal)
	}

	toBal, _ := b.GetBalance(to.ID)
	if toBal != 3000 {
		t.Errorf("expected 3000, got %f", toBal)
	}
}

func TestInsufficientFunds(t *testing.T) {
	b := NewBank(filepath.Join(t.TempDir(), "bank.json"))
	from, _ := b.CreateAccount("Small", AccountOperating, "USD", 100)
	to, _ := b.CreateAccount("Big", AccountBudget, "USD", 0)

	_, err := b.Transfer(from.ID, to.ID, 200, "test", "overspend", "")
	if err == nil {
		t.Error("should fail with insufficient funds")
	}
}

func TestReceiveRevenue(t *testing.T) {
	b := NewBank(filepath.Join(t.TempDir(), "bank.json"))
	acct, _ := b.CreateAccount("Revenue", AccountRevenue, "USD", 0)

	tx, err := b.ReceivePayment(acct.ID, 5000, "revenue", "Monthly subscription", "INV-001")
	if err != nil {
		t.Fatal(err)
	}
	if tx.Type != TxCredit {
		t.Error("should be credit")
	}

	bal, _ := b.GetBalance(acct.ID)
	if bal != 5000 {
		t.Errorf("expected 5000, got %f", bal)
	}
}

func TestBudgetAllocation(t *testing.T) {
	b := NewBank(filepath.Join(t.TempDir(), "bank.json"))
	from, _ := b.CreateAccount("Operating", AccountOperating, "USD", 50000)

	alloc, err := b.AllocateBudget("eng-div", 10000, "USD", "monthly")
	if err != nil {
		t.Fatal(err)
	}
	if alloc.Remaining != 10000 {
		t.Error("remaining should equal initial amount")
	}

	// Transfer from operating to track against allocation
	b.Transfer(from.ID, from.ID, 3000, "budget", "eng spend", "")
	// Allocation tracking via target ID
	_ = alloc
}

func TestListTransactions(t *testing.T) {
	b := NewBank(filepath.Join(t.TempDir(), "bank.json"))
	acct, _ := b.CreateAccount("Main", AccountOperating, "USD", 10000)

	b.ReceivePayment(acct.ID, 1000, "revenue", "sale 1", "")
	b.ReceivePayment(acct.ID, 2000, "revenue", "sale 2", "")

	all := b.ListTransactions("", 0)
	if len(all) != 2 {
		t.Errorf("expected 2 txs, got %d", len(all))
	}

	revenue := b.ListTransactions("revenue", 1)
	if len(revenue) != 1 {
		t.Errorf("expected 1 filtered tx, got %d", len(revenue))
	}
}

func TestFinancialSummary(t *testing.T) {
	b := NewBank(filepath.Join(t.TempDir(), "bank.json"))
	acct, _ := b.CreateAccount("Main", AccountOperating, "USD", 10000)
	b.ReceivePayment(acct.ID, 5000, "revenue", "sales", "")
	b.Transfer(acct.ID, acct.ID, 1000, "infra", "cloud costs", "")

	summary := b.GetSummary()
	if summary.TotalRevenue != 5000 {
		t.Errorf("expected 5000 revenue, got %f", summary.TotalRevenue)
	}
	if summary.TxCount != 2 {
		t.Errorf("expected 2 txs, got %d", summary.TxCount)
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bank.json")

	b1 := NewBank(path)
	acct, _ := b1.CreateAccount("Main", AccountOperating, "USD", 10000)
	b1.ReceivePayment(acct.ID, 100, "rev", "test", "")

	b2 := NewBank(path)
	if len(b2.accounts) != 1 {
		t.Errorf("expected 1 account loaded, got %d", len(b2.accounts))
	}
	if len(b2.transactions) != 1 {
		t.Errorf("expected 1 tx loaded, got %d", len(b2.transactions))
	}
}
