package movementtypereadmodel

import (
	"walletaccountant/account"
	"walletaccountant/common"
	"walletaccountant/movementtype"
	"walletaccountant/tagcategory"
)

type Entity struct {
	MovementTypeId  *movementtype.Id      `json:"movementTypeId" bson:"_id"`
	Action          common.MovementAction `json:"action" bson:"action"`
	AccountId       *account.Id           `json:"accountId" bson:"account_id"`
	SourceAccountId *account.Id           `json:"sourceAccountId" bson:"source_account_id,omitempty"`
	Description     string                `json:"description" bson:"description"`
	Notes           *string               `json:"notes,omitempty" bson:"notes,omitempty"`
	Tags            []*tagcategory.TagId  `json:"tags" bson:"tags"`
}
