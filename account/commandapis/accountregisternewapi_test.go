package commandapis_test

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap/zaptest"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
	"walletaccountant/account"
	commandapis2 "walletaccountant/account/commandapis"
	"walletaccountant/api"
	"walletaccountant/commandapis"
	"walletaccountant/common"
	"walletaccountant/definitions"
)

var accountBody1 = `{
	"bankName": "a bank name",
	"name": "the bank account",
	"accountType": "savings",
	"startingBalance": 10069.5,
	"startingBalanceDate": "2018-08-26T00:00:00Z",
	"currency": "USD",
	"notes": "some notes of the account"
}`

var accountBody2 = `{
	"bankName": "a bank name",
	"name": "the bank account",
	"accountType": "savings",
	"startingBalance": 10069.5,
	"startingBalanceDate": "2018-08-26T00:00:00Z",
	"currency": "USD",
	"notes": ""
}`

var accountBody3 = `{
	"bankName": "a bank name",
	"name": "the bank account",
	"accountType": "savings",
	"startingBalance": 10069.5,
	"startingBalanceDate": "2018-08-26T00:00:00Z",
	"currency": "USD"
}`

var expectedAccountId = uuid.New()
var notes1 = "some notes of the account"
var expectedTransferObject1 = account.RegisterNewAccountTransferObject{
	BankName:            "a bank name",
	Name:                "the bank account",
	AccountType:         string(common.Savings),
	StartingBalance:     10069.5,
	StartingBalanceDate: time.Date(2018, time.August, 26, 0, 0, 0, 0, time.UTC),
	Currency:            "USD",
	Notes:               &notes1,
}

var notes2 = ""
var expectedTransferObject2 = account.RegisterNewAccountTransferObject{
	BankName:            "a bank name",
	Name:                "the bank account",
	AccountType:         string(common.Savings),
	StartingBalance:     10069.5,
	StartingBalanceDate: time.Date(2018, time.August, 26, 0, 0, 0, 0, time.UTC),
	Currency:            "USD",
	Notes:               &notes2,
}

var expectedTransferObject3 = account.RegisterNewAccountTransferObject{
	BankName:            "a bank name",
	Name:                "the bank account",
	AccountType:         string(common.Savings),
	StartingBalance:     10069.5,
	StartingBalanceDate: time.Date(2018, time.August, 26, 0, 0, 0, 0, time.UTC),
	Currency:            "USD",
	Notes:               nil,
}

func TestRegisterNewAccountApi_Handle(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)
	ctx := context.Background()

	err := os.Setenv("PORT", "59597")
	requires.NoError(err)
	err = os.Setenv("FRONTEND_URL", "http://localhost")
	requires.NoError(err)

	logger := zaptest.NewLogger(t)
	lifecycle := fxtest.NewLifecycle(t)

	registerCalled := 0
	mediator := account.CommandMediatorMock{
		RegisterNewAccountFn: func(
			ctx *gin.Context,
			transferObject account.RegisterNewAccountTransferObject,
		) (*account.Id, *definitions.WalletAccountantError) {
			registerCalled++

			switch registerCalled {
			case 1:
				asserts.Equal(expectedTransferObject1, transferObject)

				return &expectedAccountId, nil
			case 2:
				asserts.Equal(expectedTransferObject2, transferObject)

				return &expectedAccountId, nil
			case 3:
				asserts.Equal(expectedTransferObject3, transferObject)

				return &expectedAccountId, nil
			case 4:
				return nil, definitions.GenericError(errors.New("an error"), nil)
			}

			t.Log("should not be called more than twice")
			t.Fail()

			return nil, nil
		},
	}

	router := api.NewServer(
		[]definitions.Route{commandapis2.NewRegisterNewAccountApi(&mediator, logger)},
		[]definitions.AggregateFactory{},
		logger,
		lifecycle,
	)
	requires.NoError(lifecycle.Start(ctx))

	t.Run("sucssessful account registration", func(t *testing.T) {
		w := httptest.NewRecorder()
		request, err := http.NewRequest("POST", "/account", strings.NewReader(accountBody1))
		requires.NoError(err)

		request.Header.Add("Content-Type", "application/json")
		router.ServeHTTP(w, request)

		asserts.Equal(http.StatusCreated, w.Code)
		asserts.Equal("{\"accountId\":\""+expectedAccountId.String()+"\"}", w.Body.String())
	})

	t.Run("sucssessful account registration with empty notes", func(t *testing.T) {
		w := httptest.NewRecorder()
		request, err := http.NewRequest("POST", "/account", strings.NewReader(accountBody2))
		requires.NoError(err)

		request.Header.Add("Content-Type", "application/json")
		router.ServeHTTP(w, request)

		asserts.Equal(http.StatusCreated, w.Code)
		asserts.Equal("{\"accountId\":\""+expectedAccountId.String()+"\"}", w.Body.String())
	})

	t.Run("sucssessful account registration with no notes field", func(t *testing.T) {
		w := httptest.NewRecorder()
		request, err := http.NewRequest("POST", "/account", strings.NewReader(accountBody3))
		requires.NoError(err)

		request.Header.Add("Content-Type", "application/json")
		router.ServeHTTP(w, request)

		asserts.Equal(http.StatusCreated, w.Code)
		asserts.Equal("{\"accountId\":\""+expectedAccountId.String()+"\"}", w.Body.String())
	})

	t.Run("fails to register account, because of invalid JSON body", func(t *testing.T) {
		w := httptest.NewRecorder()
		request, err := http.NewRequest("POST", "/account", strings.NewReader("{invalid"))
		requires.NoError(err)

		request.Header.Add("Content-Type", "application/json")
		router.ServeHTTP(w, request)

		asserts.Equal(http.StatusBadRequest, w.Code)
		commandapis.assertGenericErrorFromResponse(
			w.Body.Bytes(),
			"invalid character 'i' looking for beginning of object key string",
			asserts,
			requires,
		)
	})

	t.Run("fails to register account, because of mediator error", func(t *testing.T) {
		w := httptest.NewRecorder()
		request, err := http.NewRequest("POST", "/account", strings.NewReader(accountBody1))
		requires.NoError(err)

		request.Header.Add("Content-Type", "application/json")
		router.ServeHTTP(w, request)

		asserts.Equal(http.StatusInternalServerError, w.Code)
		commandapis.assertGenericErrorFromResponse(
			w.Body.Bytes(),
			"an error",
			asserts,
			requires,
		)
	})

	asserts.Equal(4, registerCalled)
}
