package tagcategory_test

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"walletaccountant/tagcategory"
)

func TestQueryMediator_Tags(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)

	expectedTagCategoryEntities := []*tagcategory.CategoryEntity{&tagCategory1, &tagCategory2}

	timesCalled := 0
	repositoryMock := tagcategory.ReadModelRepositoryMock{
		GetAllFn: func(ctx context.Context) ([]*tagcategory.CategoryEntity, error) {
			timesCalled++

			return []*tagcategory.CategoryEntity{&tagCategory1, &tagCategory2}, nil
		},
	}

	queryMediator := tagcategory.NewQueryMediator(&repositoryMock)

	ctx := gin.Context{}
	actualAccount, err := queryMediator.Tags(&ctx, tagcategory.FiltersTransferObject{})
	requires.Nil(err)

	asserts.Equal(expectedTagCategoryEntities, actualAccount)

	asserts.Equal(1, timesCalled)
}
