package ledgerprojection

import (
	"github.com/looplab/eventhorizon"
	"walletaccountant/definitions"
	"walletaccountant/ledger"
)

var _ definitions.ProjectionProvider = &ProjectionConfig{}

type ProjectionConfig struct {
	projection eventhorizon.EventHandler
}

func NewProjectionConfig(projection ReadModelProjection) *ProjectionConfig {
	return &ProjectionConfig{projection: projection}
}

func (p ProjectionConfig) Matcher() eventhorizon.MatchEvents {
	return eventhorizon.MatchEvents{
		ledger.MonthStarted,
		ledger.MonthEnded,
		ledger.NewAccountMovementRegistered,
	}
}

func (p ProjectionConfig) Handler() eventhorizon.EventHandler {
	return p.projection
}
