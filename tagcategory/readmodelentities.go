package tagcategory

type Entity struct {
	TagId *Id    `json:"tag_id" bson:"_id"`
	Name  string `json:"name" bson:"name"`
	Notes string `json:"notes,omitempty" bson:"notes,omitempty"`
}

type CategoryEntity struct {
	TagCategoryId *CategoryId `json:"tagCategoryId" bson:"_id"`
	Name          string      `json:"name" bson:"name"`
	Notes         string      `json:"notes,omitempty" bson:"notes,omitempty"`
	Tags          []*Entity   `json:"tags" bson:"tags"`
}
