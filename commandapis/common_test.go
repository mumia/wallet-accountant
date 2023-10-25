package commandapis_test

import (
	"encoding/json"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"walletaccountant/definitions"
)

var tagId1 = uuid.New()
var tagId2 = uuid.New()

var expectedTagCategoryId = uuid.New()
var expectedTagId = tagId1

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
