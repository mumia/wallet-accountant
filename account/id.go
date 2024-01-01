package account

import "github.com/looplab/eventhorizon/uuid"

type Id = uuid.UUID

func IdBuilder(id uuid.UUID) *Id {
	accountId := Id(id)

	return &accountId
}
