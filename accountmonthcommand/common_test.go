package accountmonthcommand_test

import (
	"encoding/json"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"time"
	"walletaccountant/account"
	"walletaccountant/accountmonth"
	"walletaccountant/accountmonthreadmodel"
	"walletaccountant/accountreadmodel"
	"walletaccountant/common"
	"walletaccountant/definitions"
	"walletaccountant/movementtype"
	"walletaccountant/movementtypereadmodel"
	"walletaccountant/tagcategory"
)

var movementEventUUID1 = uuid.MustParse("72a196bc-d9b1-4c57-a916-3eabf1bf167b")
var movementTypeId1 = movementtype.Id(movementEventUUID1)
var accountId1 = account.Id(uuid.MustParse("aeea307f-3c57-467c-8954-5f541aef6772"))
var accountMonthId1 = account.Id(uuid.MustParse("2313be27-50f6-9a11-3f38-d7715ec16903"))
var tagId1 = uuid.MustParse("b6e4fa72-a603-4226-857f-1f11d2af9f44")
var tagId2 = uuid.MustParse("99a2b571-152e-65f4-c9ef-0bd08751519c")

var month = time.January
var month2 = time.March
var year = uint(2023)
var accountMonthUUIDString = "46e18992-7977-9f44-4fee-b192d8c5a746"

var date = time.Date(int(year), month, 1, 0, 0, 0, 0, time.UTC)
var accountMonthId = accountmonth.Id(uuid.MustParse(accountMonthUUIDString))
var accountMovementUUIDString = "bbbcfa83-d879-4c24-b77d-a44e8ee572b2"

var accountId2 = account.Id(uuid.MustParse("bb44efc3-b02c-4e9b-909f-81780a746b43"))
var description = "Movement type description"
var notes = "my movement type notes"

var accountMovementId = accountmonth.AccountMovementId(uuid.MustParse(accountMovementUUIDString))

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

var accountMonthEntityEnded = accountmonthreadmodel.Entity{
	AccountMonthId: &accountMonthId,
	AccountId:      &accountId1,
	ActiveMonth: &accountmonthreadmodel.EntityActiveMonth{
		Month: month,
		Year:  year,
	},
	Movements:  []*accountmonthreadmodel.EntityMovement{},
	Balance:    1030,
	MonthEnded: true,
}

var accountEntity = accountreadmodel.Entity{
	AccountId:           &accountId1,
	BankName:            "",
	Name:                "",
	AccountType:         "checking",
	StartingBalance:     0,
	StartingBalanceDate: time.Time{},
	Currency:            "",
	Notes:               nil,
	ActiveMonth: accountreadmodel.EntityActiveMonth{
		Month: month,
		Year:  year,
	},
}

var accountEntity2 = accountreadmodel.Entity{
	AccountId:           &accountId1,
	BankName:            "",
	Name:                "",
	AccountType:         "checking",
	StartingBalance:     0,
	StartingBalanceDate: time.Time{},
	Currency:            "",
	Notes:               nil,
	ActiveMonth: accountreadmodel.EntityActiveMonth{
		Month: month2,
		Year:  year,
	},
}

var movementTypeEntity = movementtypereadmodel.Entity{
	MovementTypeId:  &movementTypeId1,
	Action:          common.Credit,
	AccountId:       &accountId1,
	SourceAccountId: nil,
	Description:     description,
	Notes:           &notes,
	Tags:            []*tagcategory.TagId{&tagId1},
}

var movementTypeEntity2 = movementtypereadmodel.Entity{
	MovementTypeId:  &movementTypeId1,
	Action:          common.Credit,
	AccountId:       &accountId2,
	SourceAccountId: nil,
	Description:     description,
	Notes:           &notes,
	Tags:            []*tagcategory.TagId{&tagId1},
}

func stringPtr(value string) *string {
	return &value
}

func assertGenericErrorFromResponse(
	responseBody []byte,
	expectedReason string,
	asserts *assert.Assertions,
	requires *require.Assertions,
) {
	assertErrorFromResponse(responseBody, expectedReason, definitions.GenericCode, nil, asserts, requires)
}

func assertErrorFromResponse(
	responseBody []byte,
	expectedReason string,
	expectedErrorCode definitions.ErrorCode,
	expectedErrorContext *definitions.ErrorContext,
	asserts *assert.Assertions,
	requires *require.Assertions,
) {
	var walletAccountantError definitions.WalletAccountantError

	err := json.Unmarshal(responseBody, &walletAccountantError)
	requires.NoError(err)

	asserts.Equal(definitions.ErrorReason(expectedReason), walletAccountantError.Reason)
	asserts.Equal(expectedErrorCode, walletAccountantError.Code)

	if expectedErrorContext != nil {
		asserts.Equal(*expectedErrorContext, walletAccountantError.Context)
	}
}
