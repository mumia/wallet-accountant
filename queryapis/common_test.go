package queryapis_test

import (
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"time"
	"walletaccountant/account"
)

var accountId1 = account.Id(uuid.New())
var accountEntity1 = account.Entity{
	AccountId:           &accountId1,
	BankName:            "a bank name",
	Name:                "an account name",
	AccountType:         account.Checking,
	StartingBalance:     5069,
	StartingBalanceDate: time.Now(),
	Currency:            account.EUR,
	Notes:               "a set of notes",
	ActiveMonth: account.EntityActiveMonth{
		Month: time.August,
		Year:  2023,
	},
}

var accountId2 = account.Id(uuid.New())
var accountEntity2 = account.Entity{
	AccountId:           &accountId2,
	BankName:            "another bank name",
	Name:                "annother account name",
	AccountType:         account.Savings,
	StartingBalance:     6069,
	StartingBalanceDate: time.Now().Add(1 * time.Minute),
	Currency:            account.USD,
	Notes:               "another set of notes",
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
) {
	w := httptest.NewRecorder()
	request, err := http.NewRequest(method, url, body)
	requires.NoError(err)

	router.ServeHTTP(w, request)

	asserts.Equal(expectedStatus, w.Code)
	asserts.Equal(expectedResponseBody, w.Body.String())
}
