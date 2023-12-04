package tagcategory

type AddNewTagToNewCategoryTransferObject struct {
	CategoryName  string  `json:"categoryName" binding:"required"`
	CategoryNotes *string `json:"categoryNotes"`
	TagName       string  `json:"tagName" binding:"required"`
	TagNotes      *string `json:"tagNotes"`
}

type AddNewTagToExistingCategoryTransferObject struct {
	TagCategoryId string  `json:"tagCategoryId" binding:"required"`
	TagName       string  `json:"tagName" binding:"required"`
	TagNotes      *string `json:"tagNotes"`
}
