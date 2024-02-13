package queryapis_test

import (
	"encoding/json"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"walletaccountant/definitions"
	"walletaccountant/tagcategory"
)

var tagCategoryId1 = tagcategory.Id(uuid.New())
var tagCategoryId2 = tagcategory.Id(uuid.New())
var tagId1 = tagcategory.TagId(uuid.New())
var tagId2 = tagcategory.TagId(uuid.New())
var tagId3 = tagcategory.TagId(uuid.New())

var tagCategoryNotes1 = "tag category 1 notes"
var tagCategoryNotes2 = "tag category 2 notes"

var tagNotes1 = "tag 1 notes"
var tagNotes2 = "tag 2 notes"
var tagNotes3 = "tag 3 notes"

var tag1 = tagcategory.Entity{
	TagId: &tagId1,
	Name:  "tag 1 name",
	Notes: &tagNotes1,
}

var tag2 = tagcategory.Entity{
	TagId: &tagId2,
	Name:  "tag 2 name",
	Notes: &tagNotes2,
}

var tag3 = tagcategory.Entity{
	TagId: &tagId3,
	Name:  "tag 3 name",
	Notes: &tagNotes3,
}

var tagCategoryEntity1 = tagcategory.CategoryEntity{
	TagCategoryId: &tagCategoryId1,
	Name:          "tag category 1 name",
	Notes:         &tagCategoryNotes1,
	Tags:          []*tagcategory.Entity{&tag2, &tag1},
}

var tagCategoryEntity2 = tagcategory.CategoryEntity{
	TagCategoryId: &tagCategoryId2,
	Name:          "tag category 2 name",
	Notes:         &tagCategoryNotes2,
	Tags:          []*tagcategory.Entity{&tag3},
}

func assertGenericErrorFromResponse(
	responseBody []byte,
	expectedReason string,
	asserts *assert.Assertions,
	requires *require.Assertions,
) {
	var genericError definitions.WalletAccountantError

	err := json.Unmarshal(responseBody, &genericError)
	requires.NoError(err)

	asserts.Equal(
		definitions.ErrorReason(expectedReason),
		genericError.Reason,
	)
	asserts.Equal(
		definitions.GenericCode,
		genericError.Code,
	)
}
