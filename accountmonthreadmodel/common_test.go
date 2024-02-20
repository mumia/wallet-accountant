package accountmonthreadmodel_test

import (
	"github.com/looplab/eventhorizon/uuid"
	"time"
	"walletaccountant/account"
	"walletaccountant/accountmonth"
	"walletaccountant/accountmonthreadmodel"
	"walletaccountant/movementtype"
)

var movementEventUUID1 = uuid.MustParse("72a196bc-d9b1-4c57-a916-3eabf1bf167b")
var movementTypeId1 = movementtype.Id(movementEventUUID1)
var accountId1 = account.Id(uuid.MustParse("aeea307f-3c57-467c-8954-5f541aef6772"))

var month = time.January

var year = uint(2023)
var accountMonthUUIDString = "46e18992-7977-9f44-4fee-b192d8c5a746"

var date = time.Date(int(year), month, 1, 0, 0, 0, 0, time.UTC)
var accountMonthId = accountmonth.Id(uuid.MustParse(accountMonthUUIDString))

var accountMonthEntity = accountmonthreadmodel.Entity{
	AccountMonthId: &accountMonthId,
	AccountId:      &accountId1,
	ActiveMonth: &accountmonthreadmodel.EntityActiveMonth{
		Month: month,
		Year:  year,
	},
	Movements:  []*accountmonthreadmodel.EntityMovement{},
	Balance:    1030.56,
	MonthEnded: false,
}
