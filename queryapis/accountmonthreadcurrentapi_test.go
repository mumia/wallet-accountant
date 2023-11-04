package queryapis_test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap/zaptest"
	"net/http"
	"os"
	"testing"
	"walletaccountant/account"
	"walletaccountant/accountmonth"
	"walletaccountant/api"
	"walletaccountant/definitions"
	"walletaccountant/queryapis"
)

func TestNewReadCurrentAccountMonthApi(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)
	ctx := context.Background()

	err := os.Setenv("PORT", "59606")
	requires.NoError(err)
	err = os.Setenv("FRONTEND_URL", "http://localhost")
	requires.NoError(err)

	logger := zaptest.NewLogger(t)
	lifecycle := fxtest.NewLifecycle(t)

	accountMonthCalled := 0
	mediator := accountmonth.QueryMediatorMock{
		AccountMonthFn: func(
			ctx *gin.Context,
			accountId *account.Id,
		) (*accountmonth.Entity, *definitions.WalletAccountantError) {
			accountMonthCalled++

			asserts.Equal(&accountId1, accountId)

			switch accountMonthCalled {
			case 1:
				return &accountMonthEntity1, nil

			case 2:
				return nil, definitions.GenericError(errors.New("an error"), nil)

			case 3:
				return nil, accountmonth.NonExistentAccountMonthError(
					accountId.String(),
					"",
					int(month),
					int(year),
				)
			}

			t.Log("should not be called more than 3 times")
			t.Fail()

			return nil, nil
		},
	}

	router := api.NewServer(
		[]definitions.Route{queryapis.NewReadCurrentAccountMonthApi(&mediator, logger)},
		[]definitions.AggregateFactory{},
		logger,
		lifecycle,
	)
	requires.NoError(lifecycle.Start(ctx))

	t.Run("successfully get current account month", func(t *testing.T) {
		expectedAccountMonthResponse, err := json.Marshal(accountMonthEntity1)
		requires.NoError(err)

		executeAndAssertResult(
			asserts,
			requires,
			router,
			"GET",
			"/account-month/"+accountId1.String(),
			nil,
			http.StatusOK,
			string(expectedAccountMonthResponse),
			false,
		)
	})

	t.Run("fails to get current account month, because of invalid uuid", func(t *testing.T) {
		executeAndAssertResult(
			asserts,
			requires,
			router,
			"GET",
			"/account-month/invaldid-uuid",
			nil,
			http.StatusBadRequest,
			"Key: 'currentAccountMonthRequest.AccountId' Error:Field validation for 'AccountId' failed on the 'uuid' tag",
			true,
		)
	})

	t.Run("fails to get current account month, because of a generic mediator error", func(t *testing.T) {
		executeAndAssertResult(
			asserts,
			requires,
			router,
			"GET",
			"/account-month/"+accountId1.String(),
			nil,
			http.StatusInternalServerError,
			"an error",
			true,
		)
	})

	t.Run("fails to get all accounts, because of non existent account", func(t *testing.T) {
		executeAndAssertResult(
			asserts,
			requires,
			router,
			"GET",
			"/account-month/"+accountId1.String(),
			nil,
			http.StatusNotFound,
			"{\"error\":\"Account month does not exist\",\"code\":404,\"context\":{\"accountId\":\"aeea307f-3c57-467c-8954-5f541aef6772\",\"month\":1,\"movementTypeId\":\"\",\"year\":2023}}",
			false,
		)
	})

	asserts.Equal(3, accountMonthCalled)
}
