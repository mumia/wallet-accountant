package movementtype_test

import (
	"github.com/looplab/eventhorizon/uuid"
	"walletaccountant/account"
	"walletaccountant/movementtype"
	"walletaccountant/tagcategory"
)

var movementEventUUID1 = uuid.MustParse("72a196bc-d9b1-4c57-a916-3eabf1bf167b")
var movementEventUUID2 = uuid.MustParse("3bcbfc67-19cd-4eb0-9daf-32daa8769069")
var movementTypeId1 = movementtype.Id(movementEventUUID1)
var movementTypeId2 = movementtype.Id(movementEventUUID2)
var movementType = movementtype.Debit
var accountId1 = account.Id(uuid.MustParse("aeea307f-3c57-467c-8954-5f541aef6772"))
var accountId2 = account.Id(uuid.MustParse("bb44efc3-b02c-4e9b-909f-81780a746b43"))
var sourceAccountId = account.Id(uuid.MustParse("f4081021-adf4-4b04-a6e5-4ad0028b96f9"))
var description = "Movement type description"
var description2 = "Movement type description with source account"
var notes = "my movement type notes"
var notes2 = "my movement type notes with source account"
var tagId1 = tagcategory.TagId(uuid.MustParse("07a7ccde-b19c-412a-a054-bc09ac529357"))
var tagId2 = tagcategory.TagId(uuid.MustParse("7ff907ef-76a1-418b-8271-f732a9014f03"))

var movementTypeEntity1 = movementtype.Entity{
	MovementTypeId:  &movementTypeId1,
	Type:            movementtype.Credit,
	AccountId:       &accountId1,
	SourceAccountId: nil,
	Description:     description,
	Notes:           notes,
	Tags:            []*tagcategory.TagId{&tagId1},
}

var movementTypeEntityWithSourceAccount = movementtype.Entity{
	MovementTypeId:  &movementTypeId2,
	Type:            movementtype.Debit,
	AccountId:       &accountId2,
	SourceAccountId: &sourceAccountId,
	Description:     description2,
	Notes:           notes2,
	Tags:            []*tagcategory.TagId{&tagId2, &tagId1},
}
