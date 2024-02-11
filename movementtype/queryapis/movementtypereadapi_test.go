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
	"walletaccountant/api"
	"walletaccountant/definitions"
	"walletaccountant/movementtype"
	"walletaccountant/movementtype/queryapis"
	queryapis2 "walletaccountant/queryapis"
)

func TestReadMovementTypeApi_Handle(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)
	ctx := context.Background()

	err := os.Setenv("PORT", "59603")
	requires.NoError(err)
	err = os.Setenv("FRONTEND_URL", "http://localhost")
	requires.NoError(err)

	logger := zaptest.NewLogger(t)
	lifecycle := fxtest.NewLifecycle(t)

	movementTypeCalled := 0
	mediator := movementtype.QueryMediatorMock{
		MovementTypeFn: func(
			ctx *gin.Context,
			movementTypeId *movementtype.Id,
		) (*movementtype.Entity, *definitions.WalletAccountantError) {
			movementTypeCalled++

			switch movementTypeCalled {
			case 1:
				asserts.Equal(&queryapis2.movementTypeId2, movementTypeId)

				return &queryapis2.movementTypeWithSourceAccountEntity1, nil

			case 2:
				return nil, definitions.GenericError(errors.New("an error"), nil)

			case 3:
				asserts.Equal(&queryapis2.movementTypeId1, movementTypeId)

				return nil, movementtype.NonExistentMovementTypeError(movementTypeId.String())
			}

			t.Log("should not be called more than thrice")
			t.Fail()

			return nil, nil
		},
	}

	router := api.NewServer(
		[]definitions.Route{queryapis.NewReadMovementTypeApi(&mediator, logger)},
		[]definitions.AggregateFactory{},
		logger,
		lifecycle,
	)
	requires.NoError(lifecycle.Start(ctx))

	t.Run("successfully gets a specific movement type", func(t *testing.T) {
		expectedMovementTypeResponse, err := json.Marshal(queryapis2.movementTypeWithSourceAccountEntity1)
		requires.NoError(err)

		queryapis2.executeAndAssertResult(
			asserts,
			requires,
			router,
			"GET",
			"/movement-type/"+queryapis2.movementTypeId2.String(),
			nil,
			http.StatusOK,
			string(expectedMovementTypeResponse),
			false,
		)
	})

	t.Run("fails to get all accounts, because of invalid uuid", func(t *testing.T) {
		queryapis2.executeAndAssertResult(
			asserts,
			requires,
			router,
			"GET",
			"/movement-type/invaldid-uuid",
			nil,
			http.StatusBadRequest,
			"Key: 'movementTypeRequest.MovementTypeId' Error:Field validation for 'MovementTypeId' failed on the 'uuid' tag",
			true,
		)
	})

	t.Run("fails to get all accounts, because of an unspecified mediator error", func(t *testing.T) {
		queryapis2.executeAndAssertResult(
			asserts,
			requires,
			router,
			"GET",
			"/movement-type/"+queryapis2.movementTypeId1.String(),
			nil,
			http.StatusInternalServerError,
			"an error",
			true,
		)
	})

	t.Run("fails to get all accounts, because of non existent account", func(t *testing.T) {
		queryapis2.executeAndAssertResult(
			asserts,
			requires,
			router,
			"GET",
			"/movement-type/"+queryapis2.movementTypeId1.String(),
			nil,
			http.StatusNotFound,
			"{\"error\":\"Movement type does not exist\",\"code\":300,\"context\":{\"movementTypeId\":\""+queryapis2.movementTypeId1.String()+"\"}}",
			false,
		)
	})

	asserts.Equal(3, movementTypeCalled)
}
