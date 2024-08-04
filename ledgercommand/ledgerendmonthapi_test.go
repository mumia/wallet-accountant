package ledgercommand_test

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
	"walletaccountant/api"
	"walletaccountant/definitions"
	"walletaccountant/ledger"
	commandapis2 "walletaccountant/ledgercommand"
)

var endMonthBody = `{
	"accountId": "aeea307f-3c57-467c-8954-5f541aef6772",
	"endBalance": 106950,
	"month": 1,
	"year": 2023
}`

var endBalance int64 = 106950
var expectedEndAccountMonthTransferObject = commandapis2.EndAccountMonthTransferObject{
	AccountId:  accountId1.String(),
	EndBalance: &endBalance,
	Month:      time.January,
	Year:       2023,
}

func TestEndAccountMonthApi_Handle(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)
	ctx := context.Background()

	err := os.Setenv("PORT", "59604")
	requires.NoError(err)
	err = os.Setenv("FRONTEND_URL", "http://localhost")
	requires.NoError(err)

	logger := zaptest.NewLogger(t)
	lifecycle := fxtest.NewLifecycle(t)

	endMonthCalled := 0
	mediator := commandapis2.CommandMediatorMock{
		EndAccountMonthFn: func(
			ctx *gin.Context,
			transferObject commandapis2.EndAccountMonthTransferObject,
		) *definitions.WalletAccountantError {
			endMonthCalled++

			switch endMonthCalled {
			case 1:
				asserts.Equal(expectedEndAccountMonthTransferObject, transferObject)

				return nil
			case 2:
				return definitions.GenericError(errors.New("an error"), nil)

			case 3:
				return ledger.NonExistentAccountError(
					accountId1.String(),
					1,
					2023,
				)

			case 4:
				return ledger.MismatchedActiveMonthError(
					accountId1.String(),
					movementTypeId1.String(),
					12,
					2022,
					1,
					2023,
				)

			case 5:
				return ledger.NonExistentAccountMonthError(
					accountId1.String(),
					movementTypeId1.String(),
					1,
					2023,
				)

			case 6:
				return ledger.AlreadyEndedError(
					accountMonthId1.String(),
					accountId1.String(),
					1,
					2023,
				)

			case 7:
				return ledger.MismatchedEndBalanceError(
					accountMonthId1.String(),
					100000,
					106950,
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
		[]definitions.Route{commandapis2.NewEndAccountMonthApi(&mediator, logger)},
		[]definitions.AggregateFactory{},
		logger,
		lifecycle,
	)
	requires.NoError(lifecycle.Start(ctx))

	t.Run("successfully end account month", func(t *testing.T) {
		w := httptest.NewRecorder()
		request, err := http.NewRequest("PUT", "/ledger", strings.NewReader(endMonthBody))
		requires.NoError(err)

		request.Header.Add("Content-Type", "application/json")
		router.ServeHTTP(w, request)

		asserts.Equal(http.StatusNoContent, w.Code)
		asserts.Equal("", w.Body.String())
	})

	t.Run("fails to end account month, because of invalid JSON body", func(t *testing.T) {
		w := httptest.NewRecorder()
		request, err := http.NewRequest("PUT", "/ledger", strings.NewReader("{invalid"))
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

	t.Run("fails to end account month, because of generic mediator error", func(t *testing.T) {
		w := httptest.NewRecorder()
		request, err := http.NewRequest("PUT", "/ledger", strings.NewReader(endMonthBody))
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
			testName:  "fails to end account month, because of NonExistentAccountError",
			errorCode: ledger.NonExistentAccountErrorCode,
			errorContext: &definitions.ErrorContext{
				"accountId": accountId1.String(),
				"month":     float64(1),
				"year":      float64(2023),
			},
			reason: "Account for account month does not exist",
		},
		{
			testName:  "fails to end account month, because of MismatchedActiveMonthError",
			errorCode: ledger.MismatchedActiveMonthErrorCode,
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
			testName:  "fails to end account month, because of NonExistentAccountMonthError",
			errorCode: ledger.NonExistentAccountMonthErrorCode,
			errorContext: &definitions.ErrorContext{
				"accountId":      accountId1.String(),
				"movementTypeId": movementTypeId1.String(),
				"month":          float64(1),
				"year":           float64(2023),
			},
			reason: "Account month does not exist",
		},
		{
			testName:  "fails to end account month, because of AlreadyEndedError",
			errorCode: ledger.AlreadyEndedErrorCode,
			errorContext: &definitions.ErrorContext{
				"accountMonthId": accountMonthId1.String(),
				"accountId":      accountId1.String(),
				"month":          float64(1),
				"year":           float64(2023),
			},
			reason: "Account month already ended",
		},
		{
			testName:  "fails to end account month, because of MismatchedEndBalanceError",
			errorCode: ledger.MismatchedEndBalanceErrorCode,
			errorContext: &definitions.ErrorContext{
				"accountMonthId":      accountMonthId1.String(),
				"accountMonthBalance": float64(100000),
				"endMonthBalance":     float64(106950),
				"month":               float64(1),
				"year":                float64(2023),
			},
			reason: "Account month does not match end balance",
		},
	}
	for _, testCase := range failureTestCases {
		t.Run(testCase.testName, func(t *testing.T) {
			w := httptest.NewRecorder()
			request, err := http.NewRequest("PUT", "/ledger", strings.NewReader(endMonthBody))
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

	asserts.Equal(7, endMonthCalled)
}
