package accountmonth

import (
	"time"
	"walletaccountant/account"
	"walletaccountant/movementtype"
)

type Entity struct {
	AccountMonthId *Id                `bson:"_id"`
	AccountId      *account.Id        `bson:"account_id"`
	ActiveMonth    *EntityActiveMonth `bson:"active_month"`
	Movements      []*EntityMovement  `bson:"movements"`
	Balance        float64            `bson:"balance"`
	MonthEnded     bool               `bson:"month_ended"`
}

type EntityActiveMonth struct {
	Month time.Month `bson:"month"`
	Year  uint       `bson:"year"`
}

type EntityMovement struct {
	MovementTypeId *movementtype.Id `bson:"movement_type_id"`
	Amount         float64          `bson:"amount"`
	Date           time.Time        `bson:"date"`
}
