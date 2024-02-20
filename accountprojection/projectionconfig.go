package accountprojection

import (
	"github.com/looplab/eventhorizon"
	"walletaccountant/account"
	"walletaccountant/definitions"
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
		account.NewAccountRegistered,
		account.NextMonthStarted,
	}
}

func (p ProjectionConfig) Handler() eventhorizon.EventHandler {
	return p.projection
}
