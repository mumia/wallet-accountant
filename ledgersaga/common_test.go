package ledgersaga_test

import (
	"time"
	"walletaccountant/account"
	"walletaccountant/ledger"
)

var month = time.January
var year = uint(2023)
var accountMonthUUIDString = "46e18992-7977-9f44-4fee-b192d8c5a746"
var accountMonthId = ledger.IdFromUUIDString(accountMonthUUIDString)
var accountId1 = account.IdFromUUIDString("aeea307f-3c57-467c-8954-5f541aef6772")
