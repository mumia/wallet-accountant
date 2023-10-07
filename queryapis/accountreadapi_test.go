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
	"walletaccountant/api"
	"walletaccountant/definitions"
	"walletaccountant/queryapis"
)

func TestReadAccountsApi_Handle(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)
	ctx := context.Background()

	err := os.Setenv("PORT", "59595")
	requires.NoError(err)
	err = os.Setenv("FRONTEND_URL", "http://localhost")
	requires.NoError(err)

	logger := zaptest.NewLogger(t)
	lifecycle := fxtest.NewLifecycle(t)

	accountCalled := 0
	mediator := account.QueryMediatorMock{
		AccountFn: func(ctx *gin.Context, accountId *account.Id) (*account.Entity, error) {
			accountCalled++

			asserts.Equal(&accountId1, accountId)

			switch accountCalled {
			case 1:
				return &accountEntity1, nil

			case 2:
				return nil, errors.New("an error")

			case 3:
				return nil, account.ErrorAccountEntityNotFound{
					SourceError:   errors.New("not found error"),
					Message:       account.ErrorAccountEntityNotFoundMessage,
					Code:          account.ErrorCodeAccountEntityNotFound,
					ContextFields: definitions.WalletAccountantErrorContext{"_id": accountId},
				}
			}

			t.Log("should not be called more than twice")
			t.Fail()

			return nil, nil
		},
	}

	router := api.NewServer(
		[]definitions.Route{queryapis.NewReadAccountsApi(&mediator, logger)},
		[]definitions.AggregateFactory{},
		logger,
		lifecycle,
	)
	requires.NoError(lifecycle.Start(ctx))

	t.Run("successfully gets a specific account", func(t *testing.T) {
		expectedAccountResponse, err := json.Marshal(accountEntity1)
		requires.NoError(err)

		executeAndAssertResult(
			asserts,
			requires,
			router,
			"GET",
			"/account/"+accountId1.String(),
			nil,
			http.StatusOK,
			string(expectedAccountResponse),
		)
	})

	t.Run("fails to get all accounts, because of invalid uuid", func(t *testing.T) {
		executeAndAssertResult(
			asserts,
			requires,
			router,
			"GET",
			"/account/invaldid-uuid",
			nil,
			http.StatusBadRequest,
			"{\"error\":\"Key: 'request.AccountId' Error:Field validation for 'AccountId' failed on the 'uuid' tag\"}",
		)
	})

	t.Run("fails to get all accounts, because of an unspecified mediator error", func(t *testing.T) {
		executeAndAssertResult(
			asserts,
			requires,
			router,
			"GET",
			"/account/"+accountId1.String(),
			nil,
			http.StatusInternalServerError,
			"{\"error\":\"an error\"}",
		)
	})

	t.Run("fails to get all accounts, because of non existent account", func(t *testing.T) {
		executeAndAssertResult(
			asserts,
			requires,
			router,
			"GET",
			"/account/"+accountId1.String(),
			nil,
			http.StatusNotFound,
			"{\"error\":\"account not found\"}",
		)
	})

	asserts.Equal(3, accountCalled)
}
