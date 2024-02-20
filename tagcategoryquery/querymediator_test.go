package tagcategoryquery_test

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"walletaccountant/tagcategorycommand"
	"walletaccountant/tagcategoryquery"
	"walletaccountant/tagcategoryreadmodel"
)

func TestQueryMediator_Tags(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)

	expectedTagCategoryEntities := []*tagcategoryreadmodel.CategoryEntity{&tagCategory1, &tagCategory2}

	timesCalled := 0
	repositoryMock := tagcategoryreadmodel.ReadModelRepositoryMock{
		GetAllFn: func(ctx context.Context) ([]*tagcategoryreadmodel.CategoryEntity, error) {
			timesCalled++

			return []*tagcategoryreadmodel.CategoryEntity{&tagCategory1, &tagCategory2}, nil
		},
	}

	queryMediator := tagcategoryquery.NewQueryMediator(&repositoryMock)

	ctx := gin.Context{}
	actualAccount, err := queryMediator.Tags(&ctx, tagcategorycommand.FiltersTransferObject{})
	requires.Nil(err)

	asserts.Equal(expectedTagCategoryEntities, actualAccount)

	asserts.Equal(1, timesCalled)
}
