package account

import (
	"time"
)

type Entity struct {
	AccountId           *Id               `json:"accountId" bson:"_id"`
	BankName            string            `json:"bankName" bson:"bank_name"`
	Name                string            `json:"name" bson:"name"`
	AccountType         Type              `json:"accountType" bson:"account_type"`
	StartingBalance     float64           `json:"startingBalance" bson:"starting_balance"`
	StartingBalanceDate time.Time         `json:"startingBalanceDate" bson:"starting_balance_date"`
	Currency            Currency          `json:"currency" bson:"currency"`
	Notes               string            `json:"notes,omitempty" bson:"notes,omitempty"`
	ActiveMonth         EntityActiveMonth `json:"activeMonth" bson:"active_month"`
}

type EntityActiveMonth struct {
	Month time.Month `bson:"month"`
	Year  uint       `bson:"year"`
}
