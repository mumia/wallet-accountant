package tagcategoryprojection_test

import (
	"github.com/looplab/eventhorizon/uuid"
	"walletaccountant/tagcategory"
)

var newTagCategoryId = uuid.New()
var newTagId = uuid.New()
var expectedTagCategoryId = tagcategory.IdFromUUID(newTagCategoryId)
var expectedTagId = tagcategory.TagIdFromUUID(newTagId)
var tagCategoryName = "tag category name"
var tagCategoryNotes = "tag category notes"
var tagName = "tag name"
var tagNotes = "my tag notes"
