package movementtype

import (
	"github.com/looplab/eventhorizon"
	"walletaccountant/definitions"
)

var _ definitions.EventMatcherHandleProvider = &ProjectionConfig{}

type ProjectionConfig struct {
	projection eventhorizon.EventHandler
}

func NewProjectionConfig(projection ReadModelProjection) *ProjectionConfig {
	return &ProjectionConfig{projection: projection}
}

func (p ProjectionConfig) Matcher() eventhorizon.MatchEvents {
	return eventhorizon.MatchEvents{NewMovementTypeRegistered}
}

func (p ProjectionConfig) Handler() eventhorizon.EventHandler {
	return p.projection
}
