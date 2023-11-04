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
	"walletaccountant/api"
	"walletaccountant/commandapis"
	"walletaccountant/definitions"
	"walletaccountant/movementtype"
)

var accountId2 = uuid.New()

var movementTypeBody = `{
	"type": "credit",
    "accountId": "` + accountId1.String() + `",
    "description": "mov type desc",
    "notes": "mov type notes",
    "tags": ["` + tagId1.String() + `", "` + tagId2.String() + `"]
}`

var movementTypeWithSourceAccountBody = `{
	"type": "debit",
    "accountId": "` + accountId2.String() + `",
    "sourceAccountId": "` + accountId1.String() + `",
    "description": "mov type desc with source",
    "notes": "mov type notes with source",
    "tags": ["` + tagId2.String() + `"]
}`

var expectedMovementTypeId1 = uuid.New()
var expectedMovementTypeId2 = uuid.New()

var notes1 = "mov type notes"
var notes2 = "mov type notes with source"

var expectedMovementTypeTransferObject = movementtype.RegisterNewMovementTypeTransferObject{
	Type:            "credit",
	AccountId:       accountId1.String(),
	SourceAccountId: nil,
	Description:     "mov type desc",
	Notes:           &notes1,
	TagIds:          []string{tagId1.String(), tagId2.String()},
}

var sourceAccountIdString = accountId1.String()
var expectedMovementTypeWithSourceTransferObject = movementtype.RegisterNewMovementTypeTransferObject{
	Type:            "debit",
	AccountId:       accountId2.String(),
	SourceAccountId: &sourceAccountIdString,
	Description:     "mov type desc with source",
	Notes:           &notes2,
	TagIds:          []string{tagId2.String()},
}

func TestRegisterNewMovementTypeApi_Handle(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)
	ctx := context.Background()

	err := os.Setenv("PORT", "59601")
	requires.NoError(err)
	err = os.Setenv("FRONTEND_URL", "http://localhost")
	requires.NoError(err)

	logger := zaptest.NewLogger(t)
	lifecycle := fxtest.NewLifecycle(t)

	registerCalled := 0
	mediator := movementtype.CommandMediatorMock{
		RegisterNewMovementTypeFn: func(
			ctx *gin.Context,
			transferObject movementtype.RegisterNewMovementTypeTransferObject,
		) (*movementtype.Id, *definitions.WalletAccountantError) {
			registerCalled++

			switch registerCalled {
			case 1:
				asserts.Equal(expectedMovementTypeTransferObject, transferObject)

				return &expectedMovementTypeId1, nil
			case 2:
				asserts.Equal(expectedMovementTypeWithSourceTransferObject, transferObject)

				return &expectedMovementTypeId2, nil
			case 3:
				return nil, definitions.GenericError(errors.New("an error"), nil)
			}

			t.Log("should not be called more than thrice")
			t.Fail()

			return nil, nil
		},
	}

	router := api.NewServer(
		[]definitions.Route{commandapis.NewRegisterNewMovementTypeApi(&mediator, logger)},
		[]definitions.AggregateFactory{},
		logger,
		lifecycle,
	)
	requires.NoError(lifecycle.Start(ctx))

	t.Run("successful movement type registration", func(t *testing.T) {
		w := httptest.NewRecorder()
		request, err := http.NewRequest("POST", "/movement-type", strings.NewReader(movementTypeBody))
		requires.NoError(err)

		request.Header.Add("Content-Type", "application/json")
		router.ServeHTTP(w, request)

		asserts.Equal(http.StatusCreated, w.Code)
		asserts.Equal("{\"movementTypeId\":\""+expectedMovementTypeId1.String()+"\"}", w.Body.String())
	})

	t.Run("successful movement type with source account registration", func(t *testing.T) {
		w := httptest.NewRecorder()
		request, err := http.NewRequest(
			"POST",
			"/movement-type",
			strings.NewReader(movementTypeWithSourceAccountBody),
		)
		requires.NoError(err)

		request.Header.Add("Content-Type", "application/json")
		router.ServeHTTP(w, request)

		asserts.Equal(http.StatusCreated, w.Code)
		asserts.Equal("{\"movementTypeId\":\""+expectedMovementTypeId2.String()+"\"}", w.Body.String())
	})

	t.Run("fails to register movement type, because of invalid JSON body", func(t *testing.T) {
		w := httptest.NewRecorder()
		request, err := http.NewRequest("POST", "/movement-type", strings.NewReader("{invalid"))
		requires.NoError(err)

		request.Header.Add("Content-Type", "application/json")
		router.ServeHTTP(w, request)

		asserts.Equal(http.StatusBadRequest, w.Code)
		assertGenericErrorFromResponse(
			w.Body.Bytes(),
			"invalid character 'i' looking for beginning of object key string",
			asserts,
			requires,
		)
	})

	t.Run("fails to register movement type, because of mediator error", func(t *testing.T) {
		w := httptest.NewRecorder()
		request, err := http.NewRequest("POST", "/movement-type", strings.NewReader(movementTypeBody))
		requires.NoError(err)

		request.Header.Add("Content-Type", "application/json")
		router.ServeHTTP(w, request)

		asserts.Equal(http.StatusInternalServerError, w.Code)
		assertGenericErrorFromResponse(
			w.Body.Bytes(),
			"an error",
			asserts,
			requires,
		)
	})

	asserts.Equal(3, registerCalled)
}
