package saga_test

import (
	"github.com/looplab/eventhorizon/uuid"
	"time"
	"walletaccountant/account"
	"walletaccountant/accountmonth"
)

var month = time.January
var year = uint(2023)
var date = time.Date(int(year), month, 1, 0, 0, 0, 0, time.UTC)
var accountMonthUUIDString = "46e18992-7977-9f44-4fee-b192d8c5a746"
var accountMonthId = accountmonth.Id(uuid.MustParse(accountMonthUUIDString))
var accountId1 = account.Id(uuid.MustParse("aeea307f-3c57-467c-8954-5f541aef6772"))
