package tagcategoryprojection

import (
	"github.com/looplab/eventhorizon"
	"walletaccountant/definitions"
	"walletaccountant/tagcategory"
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
		tagcategory.NewTagAddedToNewCategory,
		tagcategory.NewTagAddedToExistingCategory,
	}
}

func (p ProjectionConfig) Handler() eventhorizon.EventHandler {
	return p.projection
}
