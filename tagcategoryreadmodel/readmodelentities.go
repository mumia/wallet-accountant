package tagcategoryreadmodel

import "walletaccountant/tagcategory"

type Entity struct {
	TagId *tagcategory.TagId `json:"tagId" bson:"_id"`
	Name  string             `json:"name" bson:"name"`
	Notes *string            `json:"notes,omitempty" bson:"notes,omitempty"`
}

type CategoryEntity struct {
	TagCategoryId *tagcategory.Id `json:"tagCategoryId" bson:"_id"`
	Name          string          `json:"name" bson:"name"`
	Notes         *string         `json:"notes,omitempty" bson:"notes,omitempty"`
	Tags          []*Entity       `json:"tags" bson:"tags"`
}
