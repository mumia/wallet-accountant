package eventstoredb

import "github.com/looplab/eventhorizon/uuid"

type IdGenerator interface {
	New() uuid.UUID
}

type IdCreator struct {
}

func NewIdCreator() *IdCreator {
	return &IdCreator{}
}

func (idCreator *IdCreator) New() uuid.UUID {
	return uuid.New()
}
