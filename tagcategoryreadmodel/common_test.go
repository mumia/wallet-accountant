package tagcategoryreadmodel_test

import (
	"github.com/looplab/eventhorizon/uuid"
	"walletaccountant/tagcategory"
	"walletaccountant/tagcategoryreadmodel"
)

var newTagCategoryId = uuid.New()
var newTagId = uuid.New()
var expectedTagCategoryId = tagcategory.IdFromUUID(newTagCategoryId)
var tagCategoryId2 = tagcategory.IdFromUUID(newTagCategoryId)
var expectedTagId = tagcategory.TagIdFromUUID(newTagId)
var tagId2 = tagcategory.TagIdFromUUID(newTagId)
var tagId3 = tagcategory.TagIdFromUUID(newTagId)
var tagCategoryName = "tag category name"
var tagCategoryNotes = "tag category notes"
var tagCategoryNotes2 = "tag category notes 2"
var tagName = "tag name"
var tagNotes = "my tag notes"
var tagNotes2 = "tag notes 2"
var tagNotes3 = "tag notes 3"

var tag1 = tagcategoryreadmodel.Entity{
	TagId: expectedTagId,
	Name:  tagName,
	Notes: &tagNotes,
}

var tag2 = tagcategoryreadmodel.Entity{
	TagId: tagId2,
	Name:  "tag name 2",
	Notes: &tagNotes2,
}

var tag3 = tagcategoryreadmodel.Entity{
	TagId: tagId3,
	Name:  "tag name 3",
	Notes: &tagNotes3,
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
