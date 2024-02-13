package commandapis_test

import (
	"encoding/json"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"walletaccountant/account"
	"walletaccountant/definitions"
	"walletaccountant/movementtype"
)

var movementTypeId1 = movementtype.Id(uuid.MustParse("72a196bc-d9b1-4c57-a916-3eabf1bf167b"))
var accountId1 = account.Id(uuid.MustParse("aeea307f-3c57-467c-8954-5f541aef6772"))
var accountMonthId1 = account.Id(uuid.MustParse("2313be27-50f6-9a11-3f38-d7715ec16903"))
var tagId1 = uuid.MustParse("b6e4fa72-a603-4226-857f-1f11d2af9f44")
var tagId2 = uuid.MustParse("99a2b571-152e-65f4-c9ef-0bd08751519c")

func assertGenericErrorFromResponse(
	responseBody []byte,
	expectedReason string,
	asserts *assert.Assertions,
	requires *require.Assertions,
) {
	assertErrorFromResponse(responseBody, expectedReason, definitions.GenericCode, nil, asserts, requires)
}

func assertErrorFromResponse(
	responseBody []byte,
	expectedReason string,
	expectedErrorCode definitions.ErrorCode,
	expectedErrorContext *definitions.ErrorContext,
	asserts *assert.Assertions,
	requires *require.Assertions,
) {
	var walletAccountantError definitions.WalletAccountantError

	err := json.Unmarshal(responseBody, &walletAccountantError)
	requires.NoError(err)

	asserts.Equal(definitions.ErrorReason(expectedReason), walletAccountantError.Reason)
	asserts.Equal(expectedErrorCode, walletAccountantError.Code)

	if expectedErrorContext != nil {
		asserts.Equal(*expectedErrorContext, walletAccountantError.Context)
	}
}
