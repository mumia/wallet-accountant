package app

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/mumia/wallet-accountant/internal/ingestion/domain"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
)

// SuspectedDuplicatesFound is what Ingestion tells other contexts when a
// Statement import brings Suspected duplicates (spec story 40a). It is read
// from the import's recorded event, so other contexts never decode
// Ingestion's events themselves.
type SuspectedDuplicatesFound struct {
	StatementImportID   uuid.UUID
	ConnectionID        uuid.UUID
	AccountID           uuid.UUID
	SuspectedDuplicates int
}

// SuspectedDuplicatesFoundIn reads ev. It reports false for any event other
// than a recorded Statement import with Suspected duplicates.
func SuspectedDuplicatesFoundIn(ev eventstore.CloudEvent) (SuspectedDuplicatesFound, bool, error) {
	if ev.Type != StatementImportRecordedType {
		return SuspectedDuplicatesFound{}, false, nil
	}

	var recorded domain.StatementImportRecorded
	if err := json.Unmarshal(ev.Data, &recorded); err != nil {
		return SuspectedDuplicatesFound{}, false, fmt.Errorf("read %s %s: %w", ev.Type, ev.ID, err)
	}

	if recorded.SuspectedDuplicates == 0 {
		return SuspectedDuplicatesFound{}, false, nil
	}

	return SuspectedDuplicatesFound{
		StatementImportID:   ev.Subject,
		ConnectionID:        recorded.ConnectionID,
		AccountID:           recorded.AccountID,
		SuspectedDuplicates: recorded.SuspectedDuplicates,
	}, true, nil
}
