package movementtypereadmodel_test

import (
	"github.com/looplab/eventhorizon/uuid"
	"walletaccountant/account"
	"walletaccountant/common"
	"walletaccountant/movementtype"
	"walletaccountant/movementtypereadmodel"
	"walletaccountant/tagcategory"
)

var movementEventUUID1 = uuid.MustParse("72a196bc-d9b1-4c57-a916-3eabf1bf167b")
var movementEventUUID2 = uuid.MustParse("3bcbfc67-19cd-4eb0-9daf-32daa8769069")
var movementTypeId1 = movementtype.IdFromUUID(movementEventUUID1)
var movementTypeId2 = movementtype.IdFromUUID(movementEventUUID2)
var accountId1 = account.IdFromUUIDString("aeea307f-3c57-467c-8954-5f541aef6772")
var accountId2 = account.IdFromUUIDString("bb44efc3-b02c-4e9b-909f-81780a746b43")
var sourceAccountId = account.IdFromUUIDString("f4081021-adf4-4b04-a6e5-4ad0028b96f9")
var description = "Movement type description"
var description2 = "Movement type description with source account"
var notes = "my movement type notes"
var notes2 = "my movement type notes with source account"
var tagId1 = tagcategory.TagId(uuid.MustParse("07a7ccde-b19c-412a-a054-bc09ac529357"))
var tagId2 = tagcategory.TagId(uuid.MustParse("7ff907ef-76a1-418b-8271-f732a9014f03"))

var movementTypeEntity1 = movementtypereadmodel.Entity{
	MovementTypeId:  movementTypeId1,
	Action:          common.Credit,
	AccountId:       accountId1,
	SourceAccountId: nil,
	Description:     description,
	Notes:           &notes,
	Tags:            []*tagcategory.TagId{&tagId1},
}

var movementTypeEntityWithSourceAccount = movementtypereadmodel.Entity{
	MovementTypeId:  movementTypeId2,
	Action:          common.Debit,
	AccountId:       accountId2,
	SourceAccountId: sourceAccountId,
	Description:     description2,
	Notes:           &notes2,
	Tags:            []*tagcategory.TagId{&tagId2, &tagId1},
}
