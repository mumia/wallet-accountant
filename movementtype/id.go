package movementtype

import (
	"github.com/looplab/eventhorizon/uuid"
)

type Id = uuid.UUID

func IdFromUUID(uuid uuid.UUID) *Id {
	accountId := Id(uuid)

	return &accountId
}

func IdFromUUIDString(uuidString string) *Id {
	return IdFromUUID(uuid.MustParse(uuidString))
}
