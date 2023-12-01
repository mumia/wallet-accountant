package queryapis_test

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"time"
	"walletaccountant/account"
	"walletaccountant/accountmonth"
	"walletaccountant/definitions"
	"walletaccountant/movementtype"
	"walletaccountant/tagcategory"
)

var accountId1 = account.Id(uuid.MustParse("aeea307f-3c57-467c-8954-5f541aef6772"))
var notes1 = "a set of notes"
var accountEntity1 = account.Entity{
	AccountId:           &accountId1,
	BankName:            "a bank name",
	Name:                "an account name",
	AccountType:         account.Checking,
	StartingBalance:     5069,
	StartingBalanceDate: time.Now(),
	Currency:            account.EUR,
	Notes:               &notes1,
	ActiveMonth: account.EntityActiveMonth{
		Month: time.August,
		Year:  2023,
	},
}

var accountId2 = account.Id(uuid.New())
var notes2 = "another set of notes"
var accountEntity2 = account.Entity{
	AccountId:           &accountId2,
	BankName:            "another bank name",
	Name:                "annother account name",
	AccountType:         account.Savings,
	StartingBalance:     6069,
	StartingBalanceDate: time.Now().Add(1 * time.Minute),
	Currency:            account.USD,
	Notes:               &notes2,
	ActiveMonth: account.EntityActiveMonth{
		Month: time.April,
		Year:  2022,
	},
}

var tagCategoryId1 = tagcategory.Id(uuid.New())
var tagCategoryId2 = tagcategory.Id(uuid.New())
var tagId1 = tagcategory.TagId(uuid.New())
var tagId2 = tagcategory.TagId(uuid.New())
var tagId3 = tagcategory.TagId(uuid.New())

var tag1 = tagcategory.Entity{
	TagId: &tagId1,
	Name:  "tag 1 name",
	Notes: "tag 1 notes",
}

var tag2 = tagcategory.Entity{
	TagId: &tagId2,
	Name:  "tag 2 name",
	Notes: "tag 2 notes",
}

var tag3 = tagcategory.Entity{
	TagId: &tagId3,
	Name:  "tag 3 name",
	Notes: "tag 3 notes",
}

var tagCategoryEntity1 = tagcategory.CategoryEntity{
	TagCategoryId: &tagCategoryId1,
	Name:          "tag category 1 name",
	Notes:         "tag category 1 notes",
	Tags:          []*tagcategory.Entity{&tag2, &tag1},
}

var tagCategoryEntity2 = tagcategory.CategoryEntity{
	TagCategoryId: &tagCategoryId2,
	Name:          "tag category 2 name",
	Notes:         "tag category 2 notes",
	Tags:          []*tagcategory.Entity{&tag3},
}

var movementTypeId1 = movementtype.Id(uuid.New())
var movementTypeId2 = movementtype.Id(uuid.New())

var note1 = "movement type notes"
var note2 = "movement type with source account notes"

var movementTypeEntity1 = movementtype.Entity{
	MovementTypeId:  &movementTypeId1,
	Type:            movementtype.Credit,
	AccountId:       &accountId1,
	SourceAccountId: nil,
	Description:     "movement type description",
	Notes:           &note1,
	Tags:            []*tagcategory.TagId{&tagId1},
}

var movementTypeWithSourceAccountEntity1 = movementtype.Entity{
	MovementTypeId:  &movementTypeId2,
	Type:            movementtype.Debit,
	AccountId:       &accountId2,
	SourceAccountId: &accountId1,
	Description:     "movement type with source account description",
	Notes:           &note2,
	Tags:            []*tagcategory.TagId{&tagId3, &tagId2},
}

var month = time.January
var year = uint(2023)
var accountMonthUUIDString = "46e18992-7977-9f44-4fee-b192d8c5a746"
var accountMonthId = accountmonth.Id(uuid.MustParse(accountMonthUUIDString))

var accountMonthEntity1 = accountmonth.Entity{
	AccountMonthId: &accountMonthId,
	AccountId:      &accountId1,
	ActiveMonth: &accountmonth.EntityActiveMonth{
		Month: month,
		Year:  year,
	},
	Movements:  []*accountmonth.EntityMovement{},
	Balance:    1000.45,
	MonthEnded: false,
}

func executeAndAssertResult(
	asserts *assert.Assertions,
	requires *require.Assertions,
	router *gin.Engine,
	method string,
	url string,
	body io.Reader,
	expectedStatus int,
	expectedResponseBody string,
	isGenericError bool,
) {
	w := httptest.NewRecorder()
	request, err := http.NewRequest(method, url, body)
	requires.NoError(err)

	router.ServeHTTP(w, request)

	asserts.Equal(expectedStatus, w.Code)

	if !isGenericError {
		asserts.Equal(expectedResponseBody, w.Body.String())
	} else {
		assertGenericErrorFromResponse(
			w.Body.Bytes(),
			expectedResponseBody,
			asserts,
			requires,
		)
	}
}

func assertGenericErrorFromResponse(
	responseBody []byte,
	expectedReason string,
	asserts *assert.Assertions,
	requires *require.Assertions,
) {
	var genericError definitions.WalletAccountantError

	err := json.Unmarshal(responseBody, &genericError)
	requires.NoError(err)

	asserts.Equal(
		definitions.ErrorReason(expectedReason),
		genericError.Reason,
	)
	asserts.Equal(
		definitions.GenericCode,
		genericError.Code,
	)
}
