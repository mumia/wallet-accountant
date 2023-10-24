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
	"net/http/httptest"
	"os"
	"testing"
	"walletaccountant/api"
	"walletaccountant/definitions"
	"walletaccountant/movementtype"
	"walletaccountant/queryapis"
)

func TestReadAllMovementTypeApi_Handle(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)
	ctx := context.Background()

	err := os.Setenv("PORT", "59602")
	requires.NoError(err)
	err = os.Setenv("FRONTEND_URL", "http://localhost")
	requires.NoError(err)

	logger := zaptest.NewLogger(t)
	lifecycle := fxtest.NewLifecycle(t)

	movementTypesCalled := 0
	mediator := movementtype.QueryMediatorMock{
		MovementTypesFn: func(ctx *gin.Context) ([]*movementtype.Entity, *definitions.WalletAccountantError) {
			movementTypesCalled++

			switch movementTypesCalled {
			case 1:
				return []*movementtype.Entity{&movementTypeEntity1, &movementTypeWithSourceAccountEntity1}, nil
			case 2:
				return nil, definitions.GenericError(errors.New("an error"), nil)
			}

			t.Log("should not be called more than twice")
			t.Fail()

			return nil, nil
		},
	}

	router := api.NewServer(
		[]definitions.Route{queryapis.NewReadAllMovementTypesApi(&mediator, logger)},
		[]definitions.AggregateFactory{},
		logger,
		lifecycle,
	)
	requires.NoError(lifecycle.Start(ctx))

	t.Run("successfully gets all movement types", func(t *testing.T) {
		w := httptest.NewRecorder()
		request, err := http.NewRequest("GET", "/movement-types", nil)
		requires.NoError(err)

		router.ServeHTTP(w, request)

		expectedMovementTypesResponse, err := json.Marshal(
			[]movementtype.Entity{movementTypeEntity1, movementTypeWithSourceAccountEntity1},
		)
		requires.NoError(err)

		asserts.Equal(http.StatusOK, w.Code)
		asserts.Equal(string(expectedMovementTypesResponse), w.Body.String())
	})

	t.Run("fails to get all movement types", func(t *testing.T) {
		w := httptest.NewRecorder()
		request, err := http.NewRequest("GET", "/movement-types", nil)
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

	asserts.Equal(2, movementTypesCalled)
}
