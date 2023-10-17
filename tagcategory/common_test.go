package tagcategory_test

import (
	"github.com/looplab/eventhorizon/uuid"
	"walletaccountant/tagcategory"
)

var newTagCategoryId = uuid.New()
var newTagId = uuid.New()
var expectedTagCategoryId = tagcategory.Id(newTagCategoryId)
var tagCategoryId2 = tagcategory.Id(newTagCategoryId)
var expectedTagId = tagcategory.TagId(newTagId)
var tagId2 = tagcategory.TagId(newTagId)
var tagId3 = tagcategory.TagId(newTagId)
var tagCategoryName = "tag category name"
var tagCategoryNotes = "tag category notes"
var tagName = "tag name"
var tagNotes = "my tag notes"

var tag1 = tagcategory.Entity{
	TagId: &expectedTagId,
	Name:  tagName,
	Notes: tagNotes,
}

var tag2 = tagcategory.Entity{
	TagId: &tagId2,
	Name:  "tag name 2",
	Notes: "tag notes 2",
}

var tag3 = tagcategory.Entity{
	TagId: &tagId3,
	Name:  "tag name 3",
	Notes: "tag notes 3",
}

var tagCategory1 = tagcategory.CategoryEntity{
	TagCategoryId: &expectedTagCategoryId,
	Name:          tagCategoryName,
	Notes:         tagCategoryNotes,
	Tags:          []*tagcategory.Entity{&tag1, &tag3},
}

var tagCategory2 = tagcategory.CategoryEntity{
	TagCategoryId: &tagCategoryId2,
	Name:          "tag category name 2",
	Notes:         "tag category notes 2",
	Tags:          []*tagcategory.Entity{&tag2},
}
