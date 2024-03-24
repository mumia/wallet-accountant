package tagcategoryquery_test

import (
	"encoding/json"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"walletaccountant/definitions"
	"walletaccountant/tagcategory"
	"walletaccountant/tagcategoryreadmodel"
)

var newTagCategoryId = uuid.New()
var expectedTagCategoryId = tagcategory.IdFromUUID(newTagCategoryId)
var tagCategoryId1 = tagcategory.IdFromUUID(uuid.New())
var tagCategoryId2 = tagcategory.IdFromUUID(newTagCategoryId)
var tagId2 = tagcategory.TagIdFromUUID(uuid.New())
var tagId3 = tagcategory.TagIdFromUUID(uuid.New())
var tagCategoryName = "tag category name"
var tagCategoryNotes = "tag category notes"
var tagCategoryNotes1 = "tag category 1 notes"
var tagCategoryNotes2 = "tag category notes 2"
var tagNotes1 = "tag 1 notes"
var tagNotes2 = "tag 2 notes"
var tagNotes3 = "tag 3 notes"

var tagId1 = tagcategory.TagIdFromUUID(uuid.New())

var tag1 = tagcategoryreadmodel.Entity{
	TagId: tagId1,
	Name:  "tag 1 name",
	Notes: &tagNotes1,
}

var tag2 = tagcategoryreadmodel.Entity{
	TagId: tagId2,
	Name:  "tag 2 name",
	Notes: &tagNotes2,
}

var tag3 = tagcategoryreadmodel.Entity{
	TagId: tagId3,
	Name:  "tag 3 name",
	Notes: &tagNotes3,
}

var tagCategoryEntity1 = tagcategoryreadmodel.CategoryEntity{
	TagCategoryId: tagCategoryId1,
	Name:          "tag category 1 name",
	Notes:         &tagCategoryNotes1,
	Tags:          []*tagcategoryreadmodel.Entity{&tag2, &tag1},
}

var tagCategoryEntity2 = tagcategoryreadmodel.CategoryEntity{
	TagCategoryId: tagCategoryId2,
	Name:          "tag category 2 name",
	Notes:         &tagCategoryNotes2,
	Tags:          []*tagcategoryreadmodel.Entity{&tag3},
}

var tagCategory1 = tagcategoryreadmodel.CategoryEntity{
	TagCategoryId: expectedTagCategoryId,
	Name:          tagCategoryName,
	Notes:         &tagCategoryNotes,
	Tags:          []*tagcategoryreadmodel.Entity{&tag1, &tag3},
}

var tagCategory2 = tagcategoryreadmodel.CategoryEntity{
	TagCategoryId: tagCategoryId2,
	Name:          "tag category name 2",
	Notes:         &tagCategoryNotes2,
	Tags:          []*tagcategoryreadmodel.Entity{&tag2},
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
