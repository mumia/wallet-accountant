package tagcategory

import "github.com/looplab/eventhorizon/uuid"

type Id = uuid.UUID

func IdFromUUID(id uuid.UUID) *Id {
	tagCategoryId := Id(id)

	return &tagCategoryId
}

func IdFromUUIDString(uuidString string) *Id {
	return IdFromUUID(uuid.MustParse(uuidString))
}

type TagId = uuid.UUID

func TagIdFromUUID(id uuid.UUID) *TagId {
	tagId := Id(id)

	return &tagId
}

func TagIdFromUUIDString(uuidString string) *TagId {
	return IdFromUUID(uuid.MustParse(uuidString))
}

func TagIdsFromUUIDStrings(stringIds []string) []*TagId {
	var tagIds []*TagId
	for _, stringId := range stringIds {
		tagIds = append(tagIds, TagIdFromUUIDString(stringId))
	}

	return tagIds
}
