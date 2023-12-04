package commandapis_test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap/zaptest"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"walletaccountant/api"
	"walletaccountant/commandapis"
	"walletaccountant/definitions"
	"walletaccountant/tagcategory"
)

var tagInCategoryBody = `{
	"tagCategoryId": "` + expectedTagCategoryId.String() + `",
    "tagName": "Tag name",
    "tagNotes": "Tag notes"
}`

var expectedTagInCategoryNotes = "Tag notes"
var expectedTagInCategoryTransferObject = tagcategory.AddNewTagToExistingCategoryTransferObject{
	TagCategoryId: expectedTagCategoryId.String(),
	TagName:       "Tag name",
	TagNotes:      &expectedTagInCategoryNotes,
}

func TestAddNewTagToExistingCategoryApi_Handle(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)
	ctx := context.Background()

	err := os.Setenv("PORT", "59599")
	requires.NoError(err)
	err = os.Setenv("FRONTEND_URL", "http://localhost")
	requires.NoError(err)

	logger := zaptest.NewLogger(t)
	lifecycle := fxtest.NewLifecycle(t)

	addExistingFunctionCalled := 0
	mediator := tagcategory.CommandMediatorMock{
		AddNewTagToExistingCategoryFn: func(
			ctx *gin.Context,
			transferObject tagcategory.AddNewTagToExistingCategoryTransferObject,
		) (*tagcategory.TagId, *definitions.WalletAccountantError) {
			addExistingFunctionCalled++

			switch addExistingFunctionCalled {
			case 1:
				asserts.Equal(expectedTagInCategoryTransferObject, transferObject)

				return &expectedTagId, nil
			case 2:
				return nil, definitions.GenericError(errors.New("an error"), nil)
			}

			t.Log("should not be called more than twice")
			t.Fail()

			return nil, nil
		},
	}

	router := api.NewServer(
		[]definitions.Route{commandapis.NewNewTagWithExistingCategoryApi(&mediator, logger)},
		[]definitions.AggregateFactory{},
		logger,
		lifecycle,
	)
	requires.NoError(lifecycle.Start(ctx))

	t.Run("successfully adds new tag to existing category", func(t *testing.T) {
		w := httptest.NewRecorder()
		request, err := http.NewRequest("POST", "/tag", strings.NewReader(tagInCategoryBody))
		requires.NoError(err)

		request.Header.Add("Content-Type", "application/json")
		router.ServeHTTP(w, request)

		var actualResponse map[string]string
		err = json.Unmarshal(w.Body.Bytes(), &actualResponse)
		requires.NoError(err)

		asserts.Equal(http.StatusCreated, w.Code)
		asserts.Equal(expectedTagId.String(), actualResponse["tagId"])
	})

	t.Run("fails to add new tag to existing category, because of invalid JSON body", func(t *testing.T) {
		w := httptest.NewRecorder()
		request, err := http.NewRequest("POST", "/tag", strings.NewReader("{invalid"))
		requires.NoError(err)

		request.Header.Add("Content-Type", "application/json")
		router.ServeHTTP(w, request)

		asserts.Equal(http.StatusBadRequest, w.Code)
		assertGenericErrorFromResponse(
			w.Body.Bytes(),
			"invalid character 'i' looking for beginning of object key string",
			asserts,
			requires,
		)
	})

	t.Run("fails to add new tag to existing category, because of mediator error", func(t *testing.T) {
		w := httptest.NewRecorder()
		request, err := http.NewRequest("POST", "/tag", strings.NewReader(tagInCategoryBody))
		requires.NoError(err)

		request.Header.Add("Content-Type", "application/json")
		router.ServeHTTP(w, request)

		asserts.Equal(http.StatusInternalServerError, w.Code)
		assertGenericErrorFromResponse(
			w.Body.Bytes(),
			"an error",
			asserts,
			requires,
		)
	})

	asserts.Equal(2, addExistingFunctionCalled)
}
