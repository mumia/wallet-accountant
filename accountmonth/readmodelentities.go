package accountmonth

import (
	"time"
	"walletaccountant/account"
	"walletaccountant/movementtype"
)

type Entity struct {
	AccountMonthId *Id                `json:"accountMonthId" bson:"_id"`
	AccountId      *account.Id        `json:"accountId" bson:"account_id"`
	ActiveMonth    *EntityActiveMonth `json:"activeMonth" bson:"active_month"`
	Movements      []*EntityMovement  `json:"movements" bson:"movements"`
	Balance        float64            `json:"balance" bson:"balance"`
	MonthEnded     bool               `json:"monthEnded" bson:"month_ended"`
}

type EntityActiveMonth struct {
	Month time.Month `json:"month" bson:"month"`
	Year  uint       `json:"year" bson:"year"`
}

type EntityMovement struct {
	MovementTypeId *movementtype.Id `json:"movementTypeId" bson:"movement_type_id"`
	Amount         float64          `json:"amount" bson:"amount"`
	Date           time.Time        `json:"date" bson:"date"`
}
