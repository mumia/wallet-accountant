package commandapis_test

import (
	"encoding/json"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"walletaccountant/definitions"
)

var tagId1 = uuid.MustParse("b6e4fa72-a603-4226-857f-1f11d2af9f44")

var expectedTagCategoryId = uuid.New()
var expectedTagId = tagId1

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
