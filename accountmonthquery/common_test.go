package accountmonthquery_test

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
	"walletaccountant/accountmonthreadmodel"
	"walletaccountant/accountreadmodel"
	"walletaccountant/definitions"
)

var accountId1 = account.Id(uuid.MustParse("aeea307f-3c57-467c-8954-5f541aef6772"))

var month = time.January
var year = uint(2023)
var accountMonthUUIDString = "46e18992-7977-9f44-4fee-b192d8c5a746"

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

var accountMonthEntity1 = accountmonthreadmodel.Entity{
	AccountMonthId: &accountMonthId,
	AccountId:      &accountId1,
	ActiveMonth: &accountmonthreadmodel.EntityActiveMonth{
		Month: month,
		Year:  year,
	},
	Movements:  []*accountmonthreadmodel.EntityMovement{},
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
