package account

import "time"

type RegisterNewAccountTransferObject struct {
	BankName            string    `json:"bank_name" binding:"required"`
	Name                string    `json:"name" binding:"required"`
	AccountType         int       `json:"account_type" binding:"required"`
	StartingBalance     float64   `json:"starting_balance" binding:"required"`
	StartingBalanceDate time.Time `json:"starting_balance_date" binding:"required"` // format needs to be 2018-08-26T00:00:00Z
	Currency            string    `json:"currency" binding:"required"`
	Notes               string    `json:"notes" binding:"required"`
}
