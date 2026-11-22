//go:build integration

package http_test

import (
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

// Each line of an Account's Transaction list names the Imported transaction
// whose Accept recorded it; the Opening balance Transaction and a reversal
// name none (spec story 20).
func TestAccountTransactionsNameTheirImportedTransaction(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")

	salary := f.accepted(t, "Salary", nil)
	rent := f.accepted(t, "Rent", nil)
	r := f.reversed(t, rent.TransactionID)

	byID := map[string]listedTransaction{}
	for _, l := range f.listed(t, "") {
		byID[l.TransactionID] = l
	}

	if l := byID[salary.TransactionID]; l.ImportedTransactionID != salary.ImportedTransactionID {
		t.Errorf("listed salary = %+v, want it to name %s", l, salary.ImportedTransactionID)
	}

	if l := byID[rent.TransactionID]; l.ImportedTransactionID != rent.ImportedTransactionID {
		t.Errorf("listed rent = %+v, want it to still name %s once reversed", l, rent.ImportedTransactionID)
	}

	if l, ok := byID[r.TransactionID]; !ok || l.ImportedTransactionID != "" {
		t.Errorf("listed reversal = %+v, want it to name no Imported transaction", l)
	}

	if l, ok := byID[f.openingBalanceTransaction(t)]; !ok || l.ImportedTransactionID != "" {
		t.Errorf("listed Opening balance = %+v, want it to name no Imported transaction", l)
	}
}
