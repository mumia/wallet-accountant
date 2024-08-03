package importfileprojection

import (
	"github.com/looplab/eventhorizon"
	"walletaccountant/definitions"
	"walletaccountant/importfile"
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
		importfile.NewImportFileRegistered,
		importfile.FileParseStarted,
		importfile.FileParseRestarted,
		importfile.FileParseEnded,
		importfile.FileParseFailed,
		importfile.FileDataRowAdded,
		importfile.FileDataRowMarkedAsVerified,
		importfile.FileDataRowMarkedAsInvalid,
	}
}

func (p ProjectionConfig) Handler() eventhorizon.EventHandler {
	return p.projection
}
