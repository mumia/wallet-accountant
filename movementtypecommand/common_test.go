package movementtypecommand_test

import (
	"encoding/json"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"walletaccountant/account"
	"walletaccountant/common"
	"walletaccountant/definitions"
	"walletaccountant/movementtype"
	"walletaccountant/tagcategory"
)

var movementEventUUID1 = uuid.MustParse("72a196bc-d9b1-4c57-a916-3eabf1bf167b")
var movementEventUUID2 = uuid.MustParse("3bcbfc67-19cd-4eb0-9daf-32daa8769069")
var movementTypeId1 = movementtype.Id(movementEventUUID1)
var movementTypeId2 = movementtype.Id(movementEventUUID2)
var movementType = common.Debit
var accountId1 = account.Id(uuid.MustParse("aeea307f-3c57-467c-8954-5f541aef6772"))
var description = "Movement type description"
var notes = "my movement type notes"
var tagId1 = tagcategory.TagId(uuid.MustParse("07a7ccde-b19c-412a-a054-bc09ac529357"))
var tagId2 = tagcategory.TagId(uuid.MustParse("7ff907ef-76a1-418b-8271-f732a9014f03"))

func assertGenericErrorFromResponse(
	responseBody []byte,
	expectedReason string,
	asserts *assert.Assertions,
	requires *require.Assertions,
) {
	var genericError definitions.WalletAccountantError

	err := json.Unmarshal(responseBody, &genericError)
	requires.NoError(err)

	asserts.Equal(
		definitions.ErrorReason(expectedReason),
		genericError.Reason,
	)
	asserts.Equal(
		definitions.GenericCode,
		genericError.Code,
	)
}
