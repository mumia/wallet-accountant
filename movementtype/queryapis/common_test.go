package queryapis_test

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"walletaccountant/account"
	"walletaccountant/common"
	"walletaccountant/definitions"
	"walletaccountant/movementtype"
	"walletaccountant/tagcategory"
)

var accountId1 = account.Id(uuid.MustParse("aeea307f-3c57-467c-8954-5f541aef6772"))
var accountId2 = account.Id(uuid.New())

var tagId1 = tagcategory.TagId(uuid.New())
var tagId2 = tagcategory.TagId(uuid.New())
var tagId3 = tagcategory.TagId(uuid.New())

var movementTypeId1 = movementtype.Id(uuid.New())
var movementTypeId2 = movementtype.Id(uuid.New())

var note1 = "movement type notes"
var note2 = "movement type with source account notes"

var movementTypeEntity1 = movementtype.Entity{
	MovementTypeId:  &movementTypeId1,
	Action:          common.Credit,
	AccountId:       &accountId1,
	SourceAccountId: nil,
	Description:     "movement type description",
	Notes:           &note1,
	Tags:            []*tagcategory.TagId{&tagId1},
}

var movementTypeWithSourceAccountEntity1 = movementtype.Entity{
	MovementTypeId:  &movementTypeId2,
	Action:          common.Debit,
	AccountId:       &accountId2,
	SourceAccountId: &accountId1,
	Description:     "movement type with source account description",
	Notes:           &note2,
	Tags:            []*tagcategory.TagId{&tagId3, &tagId2},
}

func executeAndAssertResult(
	asserts *assert.Assertions,
	requires *require.Assertions,
	router *gin.Engine,
	method string,
	url string,
	body io.Reader,
	expectedStatus int,
	expectedResponseBody string,
	isGenericError bool,
) {
	w := httptest.NewRecorder()
	request, err := http.NewRequest(method, url, body)
	requires.NoError(err)

	router.ServeHTTP(w, request)

	asserts.Equal(expectedStatus, w.Code)

	if !isGenericError {
		asserts.Equal(expectedResponseBody, w.Body.String())
	} else {
		assertGenericErrorFromResponse(
			w.Body.Bytes(),
			expectedResponseBody,
			asserts,
			requires,
		)
	}
}

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
