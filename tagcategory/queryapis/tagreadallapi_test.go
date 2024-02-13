package queryapis_test

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
	"testing"
	"walletaccountant/api"
	"walletaccountant/definitions"
	"walletaccountant/tagcategory"
	"walletaccountant/tagcategory/queryapis"
)

func TestReadAllTagsApi_Handle(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)
	ctx := context.Background()

	err := os.Setenv("PORT", "59600")
	requires.NoError(err)
	err = os.Setenv("FRONTEND_URL", "http://localhost")
	requires.NoError(err)

	logger := zaptest.NewLogger(t)
	lifecycle := fxtest.NewLifecycle(t)

	tagsCalled := 0
	mediator := tagcategory.QueryMediatorMock{
		TagsFn: func(ctx *gin.Context, filters tagcategory.FiltersTransferObject) ([]*tagcategory.CategoryEntity, *definitions.WalletAccountantError) {
			tagsCalled++

			switch tagsCalled {
			case 1:
				return []*tagcategory.CategoryEntity{&tagCategoryEntity1, &tagCategoryEntity2}, nil
			case 2:
				return nil, definitions.GenericError(errors.New("an error"), nil)
			}

			t.Log("should not be called more than twice")
			t.Fail()

			return nil, nil
		},
	}

	router := api.NewServer(
		[]definitions.Route{queryapis.NewReadAllTagsApi(&mediator, logger)},
		[]definitions.AggregateFactory{},
		logger,
		lifecycle,
	)
	requires.NoError(lifecycle.Start(ctx))

	t.Run("successfully gets all tags", func(t *testing.T) {
		w := httptest.NewRecorder()
		request, err := http.NewRequest("GET", "/tags", nil)
		requires.NoError(err)

		router.ServeHTTP(w, request)

		expectedTagsResponse, err := json.Marshal(
			[]tagcategory.CategoryEntity{tagCategoryEntity1, tagCategoryEntity2},
		)
		requires.NoError(err)

		asserts.Equal(http.StatusOK, w.Code)
		asserts.Equal(string(expectedTagsResponse), w.Body.String())
	})

	t.Run("fails to get all tags", func(t *testing.T) {
		w := httptest.NewRecorder()
		request, err := http.NewRequest("GET", "/tags", nil)
		requires.NoError(err)

		router.ServeHTTP(w, request)

		asserts.Equal(http.StatusInternalServerError, w.Code)
		assertGenericErrorFromResponse(
			w.Body.Bytes(),
			"an error",
			asserts,
			requires,
		)
	})

	asserts.Equal(2, tagsCalled)
}
