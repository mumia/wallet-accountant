package movementtypequery_test

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
	"walletaccountant/movementtypereadmodel"
	"walletaccountant/tagcategory"
)

var movementEventUUID1 = uuid.MustParse("72a196bc-d9b1-4c57-a916-3eabf1bf167b")
var movementEventUUID2 = uuid.MustParse("3bcbfc67-19cd-4eb0-9daf-32daa8769069")
var movementTypeId1 = movementtype.Id(movementEventUUID1)
var movementTypeId2 = movementtype.Id(movementEventUUID2)
var movementType = common.Debit
var accountId1 = account.Id(uuid.MustParse("aeea307f-3c57-467c-8954-5f541aef6772"))
var accountId2 = account.Id(uuid.MustParse("bb44efc3-b02c-4e9b-909f-81780a746b43"))
var sourceAccountId = account.Id(uuid.MustParse("f4081021-adf4-4b04-a6e5-4ad0028b96f9"))
var description = "Movement type description"
var description2 = "Movement type description with source account"
var notes = "my movement type notes"
var notes2 = "my movement type notes with source account"

var note2 = "movement type with source account notes"
var tagId1 = tagcategory.TagId(uuid.MustParse("07a7ccde-b19c-412a-a054-bc09ac529357"))
var tagId2 = tagcategory.TagId(uuid.MustParse("7ff907ef-76a1-418b-8271-f732a9014f03"))
var tagId3 = tagcategory.TagId(uuid.New())

var movementTypeEntity1 = movementtypereadmodel.Entity{
	MovementTypeId:  &movementTypeId1,
	Action:          common.Credit,
	AccountId:       &accountId1,
	SourceAccountId: nil,
	Description:     description,
	Notes:           &notes,
	Tags:            []*tagcategory.TagId{&tagId1},
}

var movementTypeWithSourceAccountEntity1 = movementtypereadmodel.Entity{
	MovementTypeId:  &movementTypeId2,
	Action:          common.Debit,
	AccountId:       &accountId2,
	SourceAccountId: &accountId1,
	Description:     "movement type with source account description",
	Notes:           &note2,
	Tags:            []*tagcategory.TagId{&tagId3, &tagId2},
}

var movementTypeEntityWithSourceAccount = movementtypereadmodel.Entity{
	MovementTypeId:  &movementTypeId2,
	Action:          common.Debit,
	AccountId:       &accountId2,
	SourceAccountId: &sourceAccountId,
	Description:     description2,
	Notes:           &notes2,
	Tags:            []*tagcategory.TagId{&tagId2, &tagId1},
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
