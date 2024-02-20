package accountmonthcommand_test

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
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
	"walletaccountant/accountmonth"
	commandapis2 "walletaccountant/accountmonthcommand"
	"walletaccountant/api"
	"walletaccountant/common"
	"walletaccountant/definitions"
)

var registerNewMovementBody = `{
	"accountId": "aeea307f-3c57-467c-8954-5f541aef6772",
	"action": "credit",
	"movementTypeId": "72a196bc-d9b1-4c57-a916-3eabf1bf167b",
	"amount": 200,
	"date": "2023-01-01T01:00:00Z",
	"description": "mov type desc",
    "notes": "mov type notes",
    "tagIds": ["b6e4fa72-a603-4226-857f-1f11d2af9f44", "99a2b571-152e-65f4-c9ef-0bd08751519c"]
}`

var expectedRegisterNewMovementTransferObject = commandapis2.RegisterNewAccountMovementTransferObject{
	AccountId:      accountId1.String(),
	Action:         string(common.Credit),
	MovementTypeId: stringPtr(movementTypeId1.String()),
	Amount:         200,
	Date:           time.Date(2023, time.January, 1, 1, 0, 0, 0, time.UTC),
	Description:    "mov type desc",
	Notes:          stringPtr("mov type notes"),
	TagIds:         []string{tagId1.String(), tagId2.String()},
}

func TestRegisterNewAccountMovementApi_Handle(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)
	ctx := context.Background()

	err := os.Setenv("PORT", "59605")
	requires.NoError(err)
	err = os.Setenv("FRONTEND_URL", "http://localhost")
	requires.NoError(err)

	logger := zaptest.NewLogger(t)
	lifecycle := fxtest.NewLifecycle(t)

	registerNewAccountMovementCalled := 0
	mediator := commandapis2.CommandMediatorMock{
		RegisterNewAccountMovementFn: func(
			ctx *gin.Context,
			transferObject commandapis2.RegisterNewAccountMovementTransferObject,
		) *definitions.WalletAccountantError {
			registerNewAccountMovementCalled++

			switch registerNewAccountMovementCalled {
			case 1:
				asserts.Equal(expectedRegisterNewMovementTransferObject, transferObject)

				return nil
			case 2:
				return definitions.GenericError(errors.New("an error"), nil)

			case 3:
				return accountmonth.NonExistentAccountError(
					accountId1.String(),
					1,
					2023,
				)

			case 4:
				return accountmonth.MismatchedActiveMonthError(
					accountId1.String(),
					movementTypeId1.String(),
					12,
					2022,
					1,
					2023,
				)

			case 5:
				return accountmonth.NonExistentMovementTypeError(
					accountId1.String(),
					stringPtr(movementTypeId1.String()),
					1,
					2023,
				)

			case 6:
				return accountmonth.MismatchedAccountIdError(
					accountId1.String(),
					movementTypeId1.String(),
					1,
					2023,
				)
			}

			t.Log("should not be called more than twice")
			t.Fail()

			return nil
		},
	}

	router := api.NewServer(
		[]definitions.Route{commandapis2.NewAccountMonthRegisterNewMovementApi(&mediator, logger)},
		[]definitions.AggregateFactory{},
		logger,
		lifecycle,
	)
	requires.NoError(lifecycle.Start(ctx))

	t.Run("successfully register new account movement", func(t *testing.T) {
		w := httptest.NewRecorder()
		request, err := http.NewRequest("POST", "/account-month/account-movement", strings.NewReader(registerNewMovementBody))
		requires.NoError(err)

		request.Header.Add("Content-Type", "application/json")
		router.ServeHTTP(w, request)

		asserts.Equal(http.StatusCreated, w.Code)
		asserts.Equal("", w.Body.String())
	})

	t.Run("fails to register new account movement, because of invalid JSON body", func(t *testing.T) {
		w := httptest.NewRecorder()
		request, err := http.NewRequest("POST", "/account-month/account-movement", strings.NewReader("{invalid"))
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

	t.Run("fails to register new account movement, because of generic mediator error", func(t *testing.T) {
		w := httptest.NewRecorder()
		request, err := http.NewRequest("POST", "/account-month/account-movement", strings.NewReader(registerNewMovementBody))
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

	failureTestCases := [...]struct {
		testName     string
		errorCode    definitions.ErrorCode
		errorContext *definitions.ErrorContext
		reason       string
	}{
		{
			testName:  "fails to register new account movement, because of NonExistentAccountError",
			errorCode: accountmonth.NonExistentAccountErrorCode,
			errorContext: &definitions.ErrorContext{
				"accountId": accountId1.String(),
				"month":     float64(1),
				"year":      float64(2023),
			},
			reason: "Account for account month does not exist",
		},
		{
			testName:  "fails to register new account movement, because of MismatchedActiveMonthError",
			errorCode: accountmonth.MismatchedActiveMonthErrorCode,
			errorContext: &definitions.ErrorContext{
				"accountId":      accountId1.String(),
				"movementTypeId": movementTypeId1.String(),
				"accountMonth":   float64(12),
				"accountYear":    float64(2022),
				"month":          float64(1),
				"year":           float64(2023),
			},
			reason: "Active month is different",
		},
		{
			testName:  "fails to register new account movement, because of NonExistentAccountMonthError",
			errorCode: accountmonth.NonExistentMovementTypeErrorCode,
			errorContext: &definitions.ErrorContext{
				"accountId":      accountId1.String(),
				"movementTypeId": movementTypeId1.String(),
				"month":          float64(1),
				"year":           float64(2023),
			},
			reason: "Movement type for account movement does not exist",
		},
		{
			testName:  "fails to register new account movement, because of AlreadyEndedError",
			errorCode: accountmonth.MismatchedAccountIdErrorCode,
			errorContext: &definitions.ErrorContext{
				"accountId":      accountId1.String(),
				"movementTypeId": movementTypeId1.String(),
				"month":          float64(1),
				"year":           float64(2023),
			},
			reason: "Movement type and account have different ids",
		},
	}
	for _, testCase := range failureTestCases {
		t.Run(testCase.testName, func(t *testing.T) {
			w := httptest.NewRecorder()
			request, err := http.NewRequest("POST", "/account-month/account-movement", strings.NewReader(registerNewMovementBody))
			requires.NoError(err)

			request.Header.Add("Content-Type", "application/json")
			router.ServeHTTP(w, request)

			asserts.Equal(http.StatusBadRequest, w.Code)
			assertErrorFromResponse(
				w.Body.Bytes(),
				testCase.reason,
				testCase.errorCode,
				testCase.errorContext,
				asserts,
				requires,
			)
		})
	}

	asserts.Equal(6, registerNewAccountMovementCalled)
}
