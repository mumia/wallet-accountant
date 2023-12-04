package tagcategory

import (
	"context"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/aggregatestore/events"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	"walletaccountant/clock"
)

func setupTagTest(instants []clock.Instant) func(id uuid.UUID) eventhorizon.Aggregate {
	tagFactory := NewFactory()
	tagFactory.clock = clock.Freeze(instants, nil)

	return tagFactory.Factory()
}

func TestTag_HandleCommand_AddNewTagToNewCategoryCommand(t *testing.T) {
	instants := []clock.Instant{
		{"add tag to new tag category", time.Now()},
	}
	newAggregateFunc := setupTagTest(instants)

	asserts := assert.New(t)
	requires := require.New(t)
	tagId := TagId(uuid.New())
	tagCategoryId := uuid.New()

	// for register
	tagCategoryAggregate := newAggregateFunc(tagCategoryId).(*TagCategory)
	tagCategory := createTagCategory(tagCategoryAggregate.CategoryId())
	tag := createTag(&tagId, "")

	categoryNotes := "My notes on my tag category"
	tagNotes := "Notes on the first tag"

	command := createAddNewTagToNewCategoryCommand(tagCategory, categoryNotes, tag, tagNotes)
	expectedEvent := createAddNewTagToNewCategoryEvent(tagCategory, categoryNotes, tag, tagNotes, instants[0].Instant)

	err := tagCategoryAggregate.HandleCommand(context.Background(), command)
	requires.NoError(err)

	t.Run("successfully added new tag to new tag category", func(t *testing.T) {
		uncommittedEvents := tagCategoryAggregate.UncommittedEvents()
		asserts.Equal(1, len(uncommittedEvents))
		asserts.Equal(expectedEvent, uncommittedEvents[0])
	})

	t.Run("fails to add new tag to new tag category, because already registered", func(t *testing.T) {
		tagCategoryAggregate.SetAggregateVersion(1)

		err = tagCategoryAggregate.HandleCommand(context.Background(), command)
		asserts.Error(err)
	})
}

func TestAccount_HandleCommand_AddNewTagToExistingCategory(t *testing.T) {
	instants := []clock.Instant{
		{"add new tag to existing tag category", time.Now()},
		{"add another new tag to existing tag category", time.Now().Add(2 * time.Second)},
	}
	newAggregateFunc := setupTagTest(instants)

	asserts := assert.New(t)
	requires := require.New(t)

	type testCase struct {
		testName      string
		category      *TagCategory
		command       eventhorizon.Command
		expectedEvent eventhorizon.Event
	}

	tagCategoryId := uuid.New()
	tagId1 := TagId(uuid.New())
	tagId2 := TagId(uuid.New())
	tagId3 := TagId(uuid.New())
	tag1 := createTag(&tagId1, "1")
	tag2 := createTag(&tagId2, "2")
	tag3 := createTag(&tagId3, "3")
	//note1 := "Notes on the first tag 1"
	note2 := "Notes on the first tag 2"
	note3 := "Notes on the first tag 3"

	baseTagCategoryAggregate := newAggregateFunc(tagCategoryId).(*TagCategory)
	baseTagCategoryAggregate.tags = append(baseTagCategoryAggregate.tags, tag1)
	baseTagCategoryAggregate.SetAggregateVersion(1)
	baseTagCategory := createTagCategory(baseTagCategoryAggregate.CategoryId())

	twoTagsTagCategoryAggregate := newAggregateFunc(tagCategoryId).(*TagCategory)
	twoTagsTagCategoryAggregate.tags = append(twoTagsTagCategoryAggregate.tags, tag1, tag2)
	twoTagsTagCategoryAggregate.SetAggregateVersion(2)
	twoTagsTagCategory := createTagCategory(twoTagsTagCategoryAggregate.CategoryId())

	testCases := []testCase{
		{
			testName:      "successfully add second tag to base tag category category",
			category:      baseTagCategoryAggregate,
			command:       createAddNewTagToExistingCategoryCommand(baseTagCategory.CategoryId(), tag2, note2),
			expectedEvent: createAddNewTagToExistingCategoryEvent(baseTagCategory, tag2, note2, instants[0].Instant, 2),
		},
		{
			testName:      "successfully add third tag to existing tag category category",
			category:      twoTagsTagCategoryAggregate,
			command:       createAddNewTagToExistingCategoryCommand(baseTagCategory.CategoryId(), tag3, note3),
			expectedEvent: createAddNewTagToExistingCategoryEvent(twoTagsTagCategory, tag3, note3, instants[1].Instant, 3),
		},
	}

	for _, testCaseData := range testCases {
		t.Run(testCaseData.testName, func(t *testing.T) {
			err := testCaseData.category.HandleCommand(context.Background(), testCaseData.command)
			requires.NoError(err)

			uncommittedEvents := testCaseData.category.UncommittedEvents()
			asserts.Equal(1, len(uncommittedEvents))
			asserts.Equal(testCaseData.expectedEvent, uncommittedEvents[0])
		})
	}

	t.Run("fails to add new tag, because category not registered", func(t *testing.T) {
		category := newAggregateFunc(tagCategoryId).(*TagCategory)

		command := createAddNewTagToExistingCategoryCommand(baseTagCategory.CategoryId(), tag2, note2)

		err := category.HandleCommand(context.Background(), command)
		asserts.Error(err)
	})
}

//func TestAccount_ApplyEvent_RegisterNewAccount(t *testing.T) {
//	instants := []clock.Instant{
//		{"register account", time.Now()},
//	}
//	newAggregateFunc, _, _ := setupAccountTest(instants)
//
//	asserts := assert.New(t)
//
//	t.Run("Correctly applies register new account event", func(t *testing.T) {
//		accountId := uuid.New()
//
//		account := newAggregateFunc(accountId).(*Account)
//		accountData := createAccountData(accountId, nil)
//
//		newAccountRegisteredEvent := createRegisterNewAccountEvent(accountData, instants[0].Instant)
//
//		err := account.ApplyEvent(context.Background(), newAccountRegisteredEvent)
//		asserts.NoError(err)
//
//		assetAccountValues(account, accountData, accountData.ActiveMonth(), asserts)
//	})
//}
//
//func TestAccount_ApplyEvent_StartNextMonth(t *testing.T) {
//	instants := []clock.Instant{
//		{"start next month", time.Now().Add(1 * time.Second)},
//	}
//	newAggregateFunc, nextMonth, _ := setupAccountTest(instants)
//
//	asserts := assert.New(t)
//
//	t.Run("Correctly applies start next month event", func(t *testing.T) {
//		accountId := uuid.New()
//
//		account := newAggregateFunc(accountId).(*Account)
//		accountData := createAccountData(accountId, &nextMonth)
//
//		newAccountRegisteredEvent := createRegisterNewAccountEvent(accountData, time.Now())
//		startNextMonthEvent := createStartNextMonthEvent(accountId, nextMonth, time.Now(), 1)
//
//		err := account.ApplyEvent(context.Background(), newAccountRegisteredEvent)
//		asserts.NoError(err)
//		err = account.ApplyEvent(context.Background(), startNextMonthEvent)
//		asserts.NoError(err)
//
//		assetAccountValues(account, accountData, accountData.ActiveMonth(), asserts)
//	})
//}

func createTagCategory(tagCategoryId *Id) *TagCategory {
	return &TagCategory{
		AggregateBase: events.NewAggregateBase(AggregateType, uuid.UUID(*tagCategoryId)),
		name:          "TagCategory name",
		tags:          []*Tag{},
	}
}

func createTag(tagId *TagId, suffix string) *Tag {
	return &Tag{
		tagId: tagId,
		name:  "Tag name " + suffix,
	}
}

func createAddNewTagToNewCategoryCommand(
	tagCategory *TagCategory,
	tagCategoryNotes string, // "My notes on my tagcategory category",
	tag *Tag,
	tagNotes string, // "Notes on the first tagcategory" + suffix,
) eventhorizon.Command {
	return &AddNewTagToNewCategory{
		TagCategoryId: *tagCategory.CategoryId(),
		Name:          tagCategory.Name(),
		Notes:         &tagCategoryNotes,
		Tag: NewTag{
			TagId: *tag.TagId(),
			Name:  tag.Name(),
			Notes: &tagNotes,
		},
	}
}

func createAddNewTagToExistingCategoryCommand(tagCategoryId *Id, tag *Tag, tagNotes string) eventhorizon.Command {
	return &AddNewTagToExistingCategory{
		TagCategoryId: *tagCategoryId,
		TagId:         *tag.tagId,
		Name:          tag.Name(),
		Notes:         &tagNotes,
	}
}

func createAddNewTagToNewCategoryEvent(
	tagCategoryData *TagCategory,
	tagCategoryNotes string, // "My notes on my tagcategory category",
	tag *Tag,
	tagNotes string, // "Notes on the first tagcategory" + suffix,
	createdAt time.Time,
) eventhorizon.Event {
	return eventhorizon.NewEvent(
		NewTagAddedToNewCategory,
		&NewTagAddedToNewCategoryData{
			TagCategoryId:    tagCategoryData.CategoryId(),
			TagCategoryName:  tagCategoryData.Name(),
			TagCategoryNotes: &tagCategoryNotes,
			TagId:            tag.tagId,
			TagName:          tag.Name(),
			TagNotes:         &tagNotes,
		},
		createdAt,
		eventhorizon.ForAggregate(AggregateType, tagCategoryData.EntityID(), 1),
	)
}

func createAddNewTagToExistingCategoryEvent(
	tagCategory *TagCategory,
	newTag *Tag,
	newTagNotes string, // "Notes on the first tagcategory" + suffix,
	createdAt time.Time,
	version int,
) eventhorizon.Event {
	return eventhorizon.NewEvent(
		NewTagAddedToExistingCategory,
		&NewTagAddedToExistingCategoryData{
			TagCategoryId: tagCategory.CategoryId(),
			TagId:         newTag.tagId,
			Name:          newTag.Name(),
			Notes:         &newTagNotes,
		},
		createdAt,
		eventhorizon.ForAggregate(AggregateType, tagCategory.EntityID(), version),
	)
}
