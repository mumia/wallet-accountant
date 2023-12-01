package movementtype

import (
	"walletaccountant/account"
	"walletaccountant/tagcategory"
)

type Entity struct {
	MovementTypeId  *Id                  `json:"movementTypeId" bson:"_id"`
	Type            Type                 `json:"Type" bson:"type"`
	AccountId       *account.Id          `json:"accountId" bson:"account_id"`
	SourceAccountId *account.Id          `json:"sourceAccountId" bson:"source_account_id,omitempty"`
	Description     string               `json:"description" bson:"description"`
	Notes           *string              `json:"notes,omitempty" bson:"notes,omitempty"`
	Tags            []*tagcategory.TagId `json:"tags" bson:"tags"`
}
