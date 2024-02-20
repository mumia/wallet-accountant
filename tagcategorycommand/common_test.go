package tagcategorycommand_test

import (
	"encoding/json"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"walletaccountant/definitions"
	"walletaccountant/tagcategory"
	"walletaccountant/tagcategoryreadmodel"
)

var tagId1 = uuid.MustParse("b6e4fa72-a603-4226-857f-1f11d2af9f44")
var expectedTagId = tagId1

var newTagCategoryId = uuid.New()
var newTagId = uuid.New()
var expectedTagCategoryId = tagcategory.Id(newTagCategoryId)
var tagCategoryId2 = tagcategory.Id(newTagCategoryId)
var tagId2 = tagcategory.TagId(newTagId)
var tagId3 = tagcategory.TagId(newTagId)
var tagCategoryName = "tag category name"
var tagCategoryNotes = "tag category notes"
var tagCategoryNotes2 = "tag category notes 2"
var tagName = "tag name"
var tagNotes = "my tag notes"
var tagNotes2 = "tag notes 2"
var tagNotes3 = "tag notes 3"

var tag1 = tagcategoryreadmodel.Entity{
	TagId: &expectedTagId,
	Name:  tagName,
	Notes: &tagNotes,
}

var tag2 = tagcategoryreadmodel.Entity{
	TagId: &tagId2,
	Name:  "tag name 2",
	Notes: &tagNotes2,
}

var tag3 = tagcategoryreadmodel.Entity{
	TagId: &tagId3,
	Name:  "tag name 3",
	Notes: &tagNotes3,
}

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
