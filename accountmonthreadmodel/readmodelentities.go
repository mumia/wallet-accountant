package accountmonthreadmodel

import (
	"time"
	"walletaccountant/account"
	"walletaccountant/accountmonth"
	"walletaccountant/common"
	"walletaccountant/movementtype"
	"walletaccountant/tagcategory"
)

type Entity struct {
	AccountMonthId *accountmonth.Id   `json:"accountMonthId" bson:"_id"`
	AccountId      *account.Id        `json:"accountId" bson:"account_id"`
	ActiveMonth    *EntityActiveMonth `json:"activeMonth" bson:"active_month"`
	Movements      []*EntityMovement  `json:"movements" bson:"movements"`
	Balance        float32            `json:"balance" bson:"balance"`
	InitialBalance float32            `json:"initialBalance" bson:"initial_balance"`
	MonthEnded     bool               `json:"monthEnded" bson:"month_ended"`
}

type EntityActiveMonth struct {
	Month time.Month `json:"month" bson:"month"`
	Year  uint       `json:"year" bson:"year"`
}

type EntityMovement struct {
	MovementTypeId  *movementtype.Id      `json:"movementTypeId" bson:"movement_type_id"`
	Action          common.MovementAction `json:"action" bson:"action"`
	Amount          float32               `json:"amount" bson:"amount"`
	Date            time.Time             `json:"date" bson:"date"`
	SourceAccountId *account.Id           `json:"sourceAccountId" bson:"source_account_id"`
	Description     string                `json:"description" bson:"description"`
	Notes           *string               `json:"notes" bson:"notes"`
	TagIds          []*tagcategory.TagId  `json:"tagIds" bson:"tag_ids"`
}
