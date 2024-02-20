package movementtypeprojection

import (
	"github.com/looplab/eventhorizon"
	"walletaccountant/definitions"
	"walletaccountant/movementtype"
)

var _ definitions.ProjectionProvider = &ProjectionConfig{}

type ProjectionConfig struct {
	projection eventhorizon.EventHandler
}

func NewProjectionConfig(projection ReadModelProjection) *ProjectionConfig {
	return &ProjectionConfig{projection: projection}
}

func (p ProjectionConfig) Matcher() eventhorizon.MatchEvents {
	return eventhorizon.MatchEvents{movementtype.NewMovementTypeRegistered}
}

func (p ProjectionConfig) Handler() eventhorizon.EventHandler {
	return p.projection
}
