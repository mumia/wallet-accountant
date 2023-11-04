package accountmonth_test

import (
	"github.com/looplab/eventhorizon/uuid"
	"time"
	"walletaccountant/account"
	"walletaccountant/accountmonth"
	"walletaccountant/movementtype"
	"walletaccountant/tagcategory"
)

var month = time.January
var month2 = time.March
var year = uint(2023)
var date = time.Date(int(year), month, 1, 0, 0, 0, 0, time.UTC)
var accountMonthUUIDString = "46e18992-7977-9f44-4fee-b192d8c5a746"
var accountMonthId = accountmonth.Id(uuid.MustParse(accountMonthUUIDString))
var movementEventUUID1 = uuid.MustParse("72a196bc-d9b1-4c57-a916-3eabf1bf167b")
var movementTypeId1 = movementtype.Id(movementEventUUID1)
var accountId1 = account.Id(uuid.MustParse("aeea307f-3c57-467c-8954-5f541aef6772"))
var accountId2 = account.Id(uuid.MustParse("bb44efc3-b02c-4e9b-909f-81780a746b43"))
var description = "Movement type description"
var notes = "my movement type notes"
var tagId1 = tagcategory.TagId(uuid.MustParse("07a7ccde-b19c-412a-a054-bc09ac529357"))

var accountMonthEntity = accountmonth.Entity{
	AccountMonthId: &accountMonthId,
	AccountId:      &accountId1,
	ActiveMonth: &accountmonth.EntityActiveMonth{
		Month: month,
		Year:  year,
	},
	Movements:  []*accountmonth.EntityMovement{},
	Balance:    1030.56,
	MonthEnded: false,
}

var accountMonthEntityEnded = accountmonth.Entity{
	AccountMonthId: &accountMonthId,
	AccountId:      &accountId1,
	ActiveMonth: &accountmonth.EntityActiveMonth{
		Month: month,
		Year:  year,
	},
	Movements:  []*accountmonth.EntityMovement{},
	Balance:    1030,
	MonthEnded: true,
}

var accountEntity = account.Entity{
	AccountId:           &accountId1,
	BankName:            "",
	Name:                "",
	AccountType:         0,
	StartingBalance:     0,
	StartingBalanceDate: time.Time{},
	Currency:            "",
	Notes:               "",
	ActiveMonth: account.EntityActiveMonth{
		Month: month,
		Year:  year,
	},
}

var accountEntity2 = account.Entity{
	AccountId:           &accountId1,
	BankName:            "",
	Name:                "",
	AccountType:         0,
	StartingBalance:     0,
	StartingBalanceDate: time.Time{},
	Currency:            "",
	Notes:               "",
	ActiveMonth: account.EntityActiveMonth{
		Month: month2,
		Year:  year,
	},
}

var movementTypeEntity = movementtype.Entity{
	MovementTypeId:  &movementTypeId1,
	Type:            movementtype.Credit,
	AccountId:       &accountId1,
	SourceAccountId: nil,
	Description:     description,
	Notes:           &notes,
	Tags:            []*tagcategory.TagId{&tagId1},
}

var movementTypeEntity2 = movementtype.Entity{
	MovementTypeId:  &movementTypeId1,
	Type:            movementtype.Credit,
	AccountId:       &accountId2,
	SourceAccountId: nil,
	Description:     description,
	Notes:           &notes,
	Tags:            []*tagcategory.TagId{&tagId1},
}
