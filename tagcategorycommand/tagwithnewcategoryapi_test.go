package tagcategorycommand_test

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
	"walletaccountant/definitions"
	"walletaccountant/tagcategory"
	"walletaccountant/tagcategorycommand"
)

var tagWithCategoryBody = `{
	"categoryName": "Category name",
    "categoryNotes": "Category notes",
    "tagName": "Tag name",
    "tagNotes": "Tag notes"
}`

var expectedTagWithCategoryCategoryNotes = "Category notes"
var expectedTagWithCategoryTagNotes = "Tag notes"
var expectedTagWithCategoryTransferObject = tagcategorycommand.AddNewTagToNewCategoryTransferObject{
	CategoryName:  "Category name",
	CategoryNotes: &expectedTagWithCategoryCategoryNotes,
	TagName:       "Tag name",
	TagNotes:      &expectedTagWithCategoryTagNotes,
}

func TestAddNewTagToNewCategoryApi_Handle(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)
	ctx := context.Background()

	err := os.Setenv("PORT", "59598")
	requires.NoError(err)
	err = os.Setenv("FRONTEND_URL", "http://localhost")
	requires.NoError(err)

	logger := zaptest.NewLogger(t)
	lifecycle := fxtest.NewLifecycle(t)

	addFunctionCalled := 0
	mediator := tagcategorycommand.CommandMediatorMock{
		AddNewTagToNewCategoryFn: func(
			ctx *gin.Context,
			transferObject tagcategorycommand.AddNewTagToNewCategoryTransferObject,
		) (*tagcategory.TagId, *tagcategory.Id, *definitions.WalletAccountantError) {
			addFunctionCalled++

			switch addFunctionCalled {
			case 1:
				asserts.Equal(expectedTagWithCategoryTransferObject, transferObject)

				return &expectedTagId, expectedTagCategoryId, nil
			case 2:
				return nil, nil, definitions.GenericError(errors.New("an error"), nil)
			}

			t.Log("should not be called more than twice")
			t.Fail()

			return nil, nil, nil
		},
	}

	router := api.NewServer(
		[]definitions.Route{tagcategorycommand.NewNewTagAndCategoryApi(&mediator, logger)},
		[]definitions.AggregateFactory{},
		logger,
		lifecycle,
	)
	requires.NoError(lifecycle.Start(ctx))

	t.Run("successfully adds new tag to new category", func(t *testing.T) {
		w := httptest.NewRecorder()
		request, err := http.NewRequest("POST", "/tag-category", strings.NewReader(tagWithCategoryBody))
		requires.NoError(err)

		request.Header.Add("Content-Type", "application/json")
		router.ServeHTTP(w, request)

		var actualResponse map[string]string
		err = json.Unmarshal(w.Body.Bytes(), &actualResponse)
		requires.NoError(err)

		asserts.Equal(http.StatusCreated, w.Code)
		asserts.Equal(expectedTagId.String(), actualResponse["tagId"])
		asserts.Equal(expectedTagCategoryId.String(), actualResponse["tagCategoryId"])
	})

	t.Run("fails to add new tag to new category, because of invalid JSON body", func(t *testing.T) {
		w := httptest.NewRecorder()
		request, err := http.NewRequest("POST", "/tag-category", strings.NewReader("{invalid"))
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

	t.Run("fails to add new tag to new category, because of mediator error", func(t *testing.T) {
		w := httptest.NewRecorder()
		request, err := http.NewRequest("POST", "/tag-category", strings.NewReader(tagWithCategoryBody))
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

	asserts.Equal(2, addFunctionCalled)
}
