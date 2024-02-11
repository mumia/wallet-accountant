package account

import (
	"time"
	"walletaccountant/common"
)

type Entity struct {
	AccountId           *Id                `json:"accountId" bson:"_id"`
	BankName            BankName           `json:"bankName" bson:"bank_name"`
	BankNameExtra       *string            `json:"bankNameExtra,omitempty" bson:"bank_name_extra"`
	Name                string             `json:"name" bson:"name"`
	AccountType         common.AccountType `json:"accountType" bson:"account_type"`
	StartingBalance     float32            `json:"startingBalance" bson:"starting_balance"`
	StartingBalanceDate time.Time          `json:"startingBalanceDate" bson:"starting_balance_date"`
	Currency            Currency           `json:"currency" bson:"currency"`
	Notes               *string            `json:"notes,omitempty" bson:"notes,omitempty"`
	ActiveMonth         EntityActiveMonth  `json:"activeMonth" bson:"active_month"`
}

type EntityActiveMonth struct {
	Month time.Month `json:"month" bson:"month"`
	Year  uint       `json:"year" bson:"year"`
}
