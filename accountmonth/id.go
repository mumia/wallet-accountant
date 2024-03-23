package accountmonth

import (
	"fmt"
	uuid2 "github.com/google/uuid"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/zeebo/xxh3"
	"time"
	"walletaccountant/account"
)

const idFormat string = "%s-%d-%d"

type Id = uuid.UUID

func IdFromUUID(uuid uuid.UUID) *Id {
	accountId := Id(uuid)

	return &accountId
}

func IdFromUUIDString(uuidString string) *Id {
	return IdFromUUID(uuid.MustParse(uuidString))
}

func IdGenerate(accountId *account.Id, month time.Month, year uint) (*Id, error) {
	idData := fmt.Sprintf(idFormat, accountId.String(), month, year)
	hash := xxh3.HashString128(idData).Bytes()
	uuidAux, err := uuid2.FromBytes(hash[:])
	if err != nil {
		return nil, err
	}

	return IdFromUUID(uuidAux), nil
}

type AccountMovementId = uuid.UUID

func AccountMovementIdFromUUID(uuid uuid.UUID) *AccountMovementId {
	accountId := AccountMovementId(uuid)

	return &accountId
}

func AccountMovementIdFromUUIDString(uuidString string) *AccountMovementId {
	return AccountMovementIdFromUUID(uuid.MustParse(uuidString))
}
