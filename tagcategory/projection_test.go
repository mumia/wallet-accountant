package tagcategory_test

import (
	"context"
	"github.com/looplab/eventhorizon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	"walletaccountant/tagcategory"
)

func TestProjection_HandleEvent_NewTagAddedToNewCategory(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)

	newTagAddedToNewCategoryData := tagcategory.NewTagAddedToNewCategoryData{
		TagCategoryId:    &expectedTagCategoryId,
		TagCategoryName:  tagCategoryName,
		TagCategoryNotes: &tagCategoryNotes,
		TagId:            &expectedTagId,
		TagName:          tagName,
		TagNotes:         &tagNotes,
	}

	expectedTagCategoryEntity := tagcategory.CategoryEntity{
		TagCategoryId: &expectedTagCategoryId,
		Name:          tagCategoryName,
		Notes:         &tagCategoryNotes,
		Tags: []*tagcategory.Entity{
			{
				TagId: &expectedTagId,
				Name:  tagName,
				Notes: &tagNotes,
			},
		},
	}

	createCallCount := 0
	repository := &tagcategory.ReadModelRepositoryMock{
		AddNewTagAndCategoryFn: func(ctx context.Context, newTagAndCategory *tagcategory.CategoryEntity) error {
			createCallCount++

			asserts.Equal(&expectedTagCategoryEntity, newTagAndCategory)

			return nil
		},
	}

	projector, err := tagcategory.NewProjection(repository)
	asserts.NoError(err)

	ctx, cancelCtx := context.WithCancel(context.Background())

	type eventCountStruct struct {
		count int
	}

	eventCount := &eventCountStruct{0}
	go func(eventCount *eventCountStruct) {
		keepRunning := true
		for keepRunning {
			select {
			case <-ctx.Done():
				keepRunning = false

			case <-projector.UpdateChannel():
				eventCount.count++
			}
		}
	}(eventCount)

	newTagAddedToNewCategoryEvent := eventhorizon.NewEvent(
		tagcategory.NewTagAddedToNewCategory,
		&newTagAddedToNewCategoryData,
		time.Now(),
		eventhorizon.ForAggregate(tagcategory.AggregateType, expectedTagCategoryId, 1),
	)

	err = projector.HandleEvent(context.Background(), newTagAddedToNewCategoryEvent)
	requires.NoError(err)

	// Wait for all update channel messages to be processed
	time.Sleep(50 * time.Millisecond)
	cancelCtx()

	asserts.Equal(1, createCallCount)
	asserts.Equal(1, eventCount.count)
}

func TestProjection_HandleEvent_NewTagAddedToExistingCategory(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)

	newTagAddedToExistingCategoryData := tagcategory.NewTagAddedToExistingCategoryData{
		TagCategoryId: &expectedTagCategoryId,
		TagId:         &expectedTagId,
		Name:          tagName,
		Notes:         &tagNotes,
	}

	expectedTagEntity := tagcategory.Entity{
		TagId: &expectedTagId,
		Name:  tagName,
		Notes: &tagNotes,
	}

	updateCallCount := 0
	repository := &tagcategory.ReadModelRepositoryMock{
		AddNewTagToCategoryFn: func(ctx context.Context, categoryId *tagcategory.Id, newTag *tagcategory.Entity) error {
			updateCallCount++

			asserts.Equal(&expectedTagCategoryId, categoryId)
			asserts.Equal(&expectedTagEntity, newTag)

			return nil
		},
	}

	projector, err := tagcategory.NewProjection(repository)
	asserts.NoError(err)

	ctx, cancelCtx := context.WithCancel(context.Background())

	type eventCountStruct struct {
		count int
	}

	eventCount := &eventCountStruct{0}
	go func(eventCount *eventCountStruct) {
		keepRunning := true
		for keepRunning {
			select {
			case <-ctx.Done():
				keepRunning = false

			case <-projector.UpdateChannel():
				eventCount.count++
			}
		}
	}(eventCount)

	newTagAddedToExistingCategoryEvent := eventhorizon.NewEvent(
		tagcategory.NewTagAddedToExistingCategory,
		&newTagAddedToExistingCategoryData,
		time.Now(),
		eventhorizon.ForAggregate(tagcategory.AggregateType, expectedTagCategoryId, 1),
	)

	err = projector.HandleEvent(context.Background(), newTagAddedToExistingCategoryEvent)
	requires.NoError(err)

	// Wait for all update channel messages to be processed
	time.Sleep(50 * time.Millisecond)
	cancelCtx()

	asserts.Equal(1, updateCallCount)
	asserts.Equal(1, eventCount.count)
}
