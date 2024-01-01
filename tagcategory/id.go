package tagcategory

import "github.com/looplab/eventhorizon/uuid"

type Id = uuid.UUID
type TagId = uuid.UUID

func IdBuilder(id uuid.UUID) *Id {
	tagCategoryId := Id(id)

	return &tagCategoryId
}

func TagIdBuilder(id uuid.UUID) *TagId {
	tagId := Id(id)

	return &tagId
}

func TagIdsFromStrings(stringIds []string) []*TagId {
	var tagIds []*TagId
	for _, stringId := range stringIds {
		tagIds = append(tagIds, TagIdBuilder(uuid.MustParse(stringId)))
	}

	return tagIds
}
