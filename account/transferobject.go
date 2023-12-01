package account

import "time"

type RegisterNewAccountTransferObject struct {
	BankName            string    `json:"bankName" binding:"required"`
	Name                string    `json:"name" binding:"required"`
	AccountType         int       `json:"accountType" binding:"required"`
	StartingBalance     float64   `json:"startingBalance" binding:"required"`
	StartingBalanceDate time.Time `json:"startingBalanceDate" binding:"required"` // format needs to be 2018-08-26T00:00:00Z
	Currency            string    `json:"currency" binding:"required"`
	Notes               *string   `json:"notes"`
}
