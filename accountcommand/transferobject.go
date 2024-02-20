package accountcommand

import "time"

type RegisterNewAccountTransferObject struct {
	BankName            string    `json:"bankName" binding:"required"`
	Name                string    `json:"name" binding:"required"`
	AccountType         string    `json:"accountType" binding:"required"`
	StartingBalance     float32   `json:"startingBalance" binding:"required"`
	StartingBalanceDate time.Time `json:"startingBalanceDate" binding:"required"` // format needs to be 2018-08-26T00:00:00Z  time_format:"2006-01-02"
	Currency            string    `json:"currency" binding:"required"`
	Notes               *string   `json:"notes"`
}
