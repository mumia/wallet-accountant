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
	"walletaccountant/common"
	"walletaccountant/definitions"
)

var accountId1 = account.Id(uuid.MustParse("aeea307f-3c57-467c-8954-5f541aef6772"))
var notes1 = "a set of notes"
var accountEntity1 = account.Entity{
	AccountId:           &accountId1,
	BankName:            "a bank name",
	Name:                "an account name",
	AccountType:         common.Checking,
	StartingBalance:     float32(5069),
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
	AccountType:         common.Savings,
	StartingBalance:     6069,
	StartingBalanceDate: time.Now().Add(1 * time.Minute),
	Currency:            account.USD,
	Notes:               &notes2,
	ActiveMonth: account.EntityActiveMonth{
		Month: time.April,
		Year:  2022,
	},
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
