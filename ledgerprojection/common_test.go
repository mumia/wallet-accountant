package ledgerprojection_test

import (
	"github.com/looplab/eventhorizon/uuid"
	"time"
	"walletaccountant/account"
	"walletaccountant/ledger"
	"walletaccountant/ledgerreadmodel"
	"walletaccountant/movementtype"
)

var movementEventUUID1 = uuid.MustParse("72a196bc-d9b1-4c57-a916-3eabf1bf167b")
var movementTypeId1 = movementtype.IdFromUUID(movementEventUUID1)
var accountId1 = account.IdFromUUIDString("aeea307f-3c57-467c-8954-5f541aef6772")
var tagId1 = uuid.MustParse("b6e4fa72-a603-4226-857f-1f11d2af9f44")
var month = time.January
var year = uint(2023)
var accountMonthUUIDString = "46e18992-7977-9f44-4fee-b192d8c5a746"

var date = time.Date(int(year), month, 1, 0, 0, 0, 0, time.UTC)
var accountMonthId = ledger.IdFromUUIDString(accountMonthUUIDString)

var accountMonthEntity = ledgerreadmodel.Entity{
	AccountMonthId: accountMonthId,
	AccountId:      accountId1,
	ActiveMonth: &ledgerreadmodel.EntityActiveMonth{
		Month: month,
		Year:  year,
	},
	Movements:  []*ledgerreadmodel.EntityMovement{},
	Balance:    103056,
	MonthEnded: false,
}
