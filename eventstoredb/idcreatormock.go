package eventstoredb

import (
	"github.com/looplab/eventhorizon/uuid"
)

var _ IdGenerator = &IdCreator{}

type IdCreatorMock struct {
	NewFn func() uuid.UUID
}

func (idCreator *IdCreatorMock) New() uuid.UUID {
	if idCreator != nil && idCreator.NewFn != nil {
		return idCreator.NewFn()
	}

	return uuid.New()
}
