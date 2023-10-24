package tagcategory

import (
	"context"
)

var _ ReadModeler = &ReadModelRepositoryMock{}

type ReadModelRepositoryMock struct {
	AddNewTagAndCategoryFn func(ctx context.Context, newTagAndCategory *CategoryEntity) error
	AddNewTagToCategoryFn  func(ctx context.Context, categoryId *Id, newTag *Entity) error
	ExistsByIdFn           func(ctx context.Context, tagId *TagId) (bool, error)
	ExistsByNameFn         func(ctx context.Context, name string) (bool, error)
	GetAllFn               func(ctx context.Context) ([]*CategoryEntity, error)
	CategoryExistsByIdFn   func(ctx context.Context, id *Id) (bool, error)
	CategoryExistsByNameFn func(ctx context.Context, name string) (bool, error)
}

func (mock *ReadModelRepositoryMock) AddNewTagAndCategory(ctx context.Context, newTagAndCategory *CategoryEntity) error {
	if mock != nil && mock.AddNewTagAndCategoryFn != nil {
		return mock.AddNewTagAndCategoryFn(ctx, newTagAndCategory)
	}

	return nil
}

func (mock *ReadModelRepositoryMock) AddNewTagToCategory(ctx context.Context, categoryId *Id, newTag *Entity) error {
	if mock != nil && mock.AddNewTagToCategoryFn != nil {
		return mock.AddNewTagToCategoryFn(ctx, categoryId, newTag)
	}

	return nil
}

func (mock *ReadModelRepositoryMock) ExistsById(ctx context.Context, tagId *TagId) (bool, error) {
	if mock != nil && mock.ExistsByIdFn != nil {
		return mock.ExistsByIdFn(ctx, tagId)
	}

	return false, nil
}

func (mock *ReadModelRepositoryMock) ExistsByName(ctx context.Context, name string) (bool, error) {
	if mock != nil && mock.ExistsByNameFn != nil {
		return mock.ExistsByNameFn(ctx, name)
	}

	return false, nil
}

func (mock *ReadModelRepositoryMock) GetAll(ctx context.Context) ([]*CategoryEntity, error) {
	if mock != nil && mock.GetAllFn != nil {
		return mock.GetAllFn(ctx)
	}

	return nil, nil
}

func (mock *ReadModelRepositoryMock) CategoryExistsById(ctx context.Context, id *Id) (bool, error) {
	if mock != nil && mock.CategoryExistsByIdFn != nil {
		return mock.CategoryExistsByIdFn(ctx, id)
	}

	return false, nil
}

func (mock *ReadModelRepositoryMock) CategoryExistsByName(ctx context.Context, name string) (bool, error) {
	if mock != nil && mock.CategoryExistsByNameFn != nil {
		return mock.CategoryExistsByNameFn(ctx, name)
	}

	return false, nil
}
