//go:build integration

package http_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

// A cash Transaction takes an Expense Category only for money out and an
// Income Category only for money in; a refused one records nothing.
func TestRecordingCashWithACategoryOfTheWrongKindIsRefused(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t, apitest.WithClock(march10))
	token := app.Register(t, "https://issuer.example", "alice")
	household := newHousehold(t, app, token)
	wallet := addWallet(t, app, token, household, "40.00")
	food := create(t, app, token, household, map[string]any{"name": "Food", "kind": "EXPENSE"})
	gifts := create(t, app, token, household, map[string]any{"name": "Gifts", "kind": "INCOME"})

	for name, tc := range map[string]struct {
		amount    string
		category  category
		kind      string
		direction string
	}{
		"Expense on money in": {"5.00", food, "EXPENSE", "IN"},
		"Income on money out": {"-2.50", gifts, "INCOME", "OUT"},
	} {
		t.Run(name, func(t *testing.T) {
			res := recordCash(t, app, token, household, wallet.AccountID, map[string]any{
				"date": "2026-02-05", "amount": eur(tc.amount), "description": name, "category_id": tc.category.CategoryID,
			})
			res.AssertProblem(t, http.StatusUnprocessableEntity, "/problems/category-direction")

			var p struct {
				apitest.ProblemBody

				CategoryID   string `json:"category_id"`
				CategoryKind string `json:"category_kind"`
				Direction    string `json:"direction"`
			}

			if err := json.Unmarshal(res.Body, &p); err != nil {
				t.Fatal(err)
			}

			if p.CategoryID != tc.category.CategoryID || p.CategoryKind != tc.kind || p.Direction != tc.direction ||
				p.Title != "Category doesn't fit the amount" || p.Detail == "" {
				t.Errorf("problem = %+v, want %s %s on money %s", p, tc.category.CategoryID, tc.kind, tc.direction)
			}
		})
	}

	if b := walletBalance(t, app, token, household, wallet.AccountID); b != "40.00" {
		t.Errorf("after refusals: balance = %s, want 40.00", b)
	}

	// Each kind fits its own direction.
	for amount, c := range map[string]category{"-2.50": food, "5.00": gifts} {
		recordCash(t, app, token, household, wallet.AccountID, map[string]any{
			"date": "2026-02-05", "amount": eur(amount), "description": c.Name, "category_id": c.CategoryID,
		}).AssertJSON(t, http.StatusCreated, &transaction{})
	}

	if b := walletBalance(t, app, token, household, wallet.AccountID); b != "42.50" {
		t.Errorf("balance = %s, want 42.50", b)
	}
}
