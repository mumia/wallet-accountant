package accountreadmodel

import (
	"time"
	"walletaccountant/account"
	"walletaccountant/common"
)

type Entity struct {
	AccountId           *account.Id        `json:"accountId" bson:"_id"`
	BankName            account.BankName   `json:"bankName" bson:"bank_name"`
	BankNameExtra       *string            `json:"bankNameExtra,omitempty" bson:"bank_name_extra"`
	Name                string             `json:"name" bson:"name"`
	AccountType         common.AccountType `json:"accountType" bson:"account_type"`
	StartingBalance     int64              `json:"startingBalance" bson:"starting_balance"`
	StartingBalanceDate time.Time          `json:"startingBalanceDate" bson:"starting_balance_date"`
	Currency            account.Currency   `json:"currency" bson:"currency"`
	Notes               *string            `json:"notes,omitempty" bson:"notes,omitempty"`
	ActiveMonth         EntityActiveMonth  `json:"activeMonth" bson:"active_month"`
}

type EntityActiveMonth struct {
	Month time.Month `json:"month" bson:"month"`
	Year  uint       `json:"year" bson:"year"`
}
