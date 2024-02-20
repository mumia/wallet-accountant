package tagcategorycommand_test

import (
	"encoding/json"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"walletaccountant/definitions"
	"walletaccountant/tagcategory"
)

var tagId1 = uuid.MustParse("b6e4fa72-a603-4226-857f-1f11d2af9f44")
var expectedTagId = tagId1

var newTagCategoryId = uuid.New()
var newTagId = tagId1
var expectedTagCategoryId = tagcategory.Id(newTagCategoryId)
var tagCategoryName = "tag category name"
var tagCategoryNotes = "tag category notes"
var tagName = "tag name"
var tagNotes = "my tag notes"

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
