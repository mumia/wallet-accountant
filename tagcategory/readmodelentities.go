package tagcategory

type Entity struct {
	TagId *TagId `json:"tag_id" bson:"_id"`
	Name  string `json:"name" bson:"name"`
	Notes string `json:"notes,omitempty" bson:"notes,omitempty"`
}

type CategoryEntity struct {
	TagCategoryId *Id       `json:"tagCategoryId" bson:"_id"`
	Name          string    `json:"name" bson:"name"`
	Notes         string    `json:"notes,omitempty" bson:"notes,omitempty"`
	Tags          []*Entity `json:"tags" bson:"tags"`
}
