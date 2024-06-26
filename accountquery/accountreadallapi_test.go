package accountquery_test

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
	"net/http/httptest"
	"os"
	"testing"
	"walletaccountant/accountquery"
	"walletaccountant/accountreadmodel"
	"walletaccountant/api"
	"walletaccountant/definitions"
)

func TestReadAllAccountsApi_Handle(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)
	ctx := context.Background()

	err := os.Setenv("PORT", "59596")
	requires.NoError(err)
	err = os.Setenv("FRONTEND_URL", "http://localhost")
	requires.NoError(err)

	logger := zaptest.NewLogger(t)
	lifecycle := fxtest.NewLifecycle(t)

	accountsCalled := 0
	mediator := accountquery.QueryMediatorMock{
		AccountsFn: func(ctx *gin.Context) ([]*accountreadmodel.Entity, *definitions.WalletAccountantError) {
			accountsCalled++

			switch accountsCalled {
			case 1:
				return []*accountreadmodel.Entity{&accountEntity1, &accountEntity2}, nil
			case 2:
				return nil, definitions.GenericError(errors.New("an error"), nil)
			}

			t.Log("should not be called more than twice")
			t.Fail()

			return nil, nil
		},
	}

	router := api.NewServer(
		[]definitions.Route{accountquery.NewReadAllAccountsApi(&mediator, logger)},
		[]definitions.AggregateFactory{},
		logger,
		lifecycle,
	)
	requires.NoError(lifecycle.Start(ctx))

	t.Run("successfully gets all accounts", func(t *testing.T) {
		w := httptest.NewRecorder()
		request, err := http.NewRequest("GET", "/accounts", nil)
		requires.NoError(err)

		router.ServeHTTP(w, request)

		expectedAccountsResponse, err := json.Marshal([]accountreadmodel.Entity{accountEntity1, accountEntity2})
		requires.NoError(err)

		asserts.Equal(http.StatusOK, w.Code)
		asserts.Equal(string(expectedAccountsResponse), w.Body.String())
	})

	t.Run("fails to get all accounts", func(t *testing.T) {
		w := httptest.NewRecorder()
		request, err := http.NewRequest("GET", "/accounts", nil)
		requires.NoError(err)

		router.ServeHTTP(w, request)

		asserts.Equal(http.StatusInternalServerError, w.Code)
		assertGenericErrorFromResponse(
			w.Body.Bytes(),
			"an error",
			asserts,
			requires,
		)
	})

	asserts.Equal(2, accountsCalled)
}
