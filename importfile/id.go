package importfile

import (
	"fmt"
	uuid2 "github.com/google/uuid"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/zeebo/xxh3"
)

const idFormat string = "%s-%d"

type Id = uuid.UUID

func IdFromUUID(id uuid.UUID) *Id {
	importFileId := Id(id)

	return &importFileId
}

func IdFromUUIDString(uuidString string) *Id {
	return IdFromUUID(uuid.MustParse(uuidString))
}

type DataRowId = uuid.UUID

func DataRowIdFromUUID(id uuid.UUID) *DataRowId {
	importFileId := DataRowId(id)

	return &importFileId
}

func DataRowIdFromUUIDString(uuidString string) *DataRowId {
	return DataRowIdFromUUID(uuid.MustParse(uuidString))
}

func DataRowIdGenerate(importFileId *Id, rowHash uint64) (*DataRowId, error) {
	idData := fmt.Sprintf(idFormat, importFileId.String(), rowHash)
	hash := xxh3.HashString128(idData).Bytes()
	uuidAux, err := uuid2.FromBytes(hash[:])
	if err != nil {
		return nil, err
	}

	return DataRowIdFromUUID(uuidAux), nil
}
