package http

import (
	"context"

	"github.com/mumia/wallet-accountant/internal/platform/httpapi"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi/oas"
)

// The operations below are in the contract but not implemented yet: each
// answers 501 until its ticket in .scratch/ui-first-slice-backend lands.

// ChangeConnection implements changeConnection (ticket 10).
func (h *ConnectionHandler) ChangeConnection(context.Context, *oas.ConnectionChange, oas.ChangeConnectionParams,
) (oas.ChangeConnectionRes, error) {
	return nil, httpapi.ErrNotImplemented
}

// TestCsvMapping implements testCsvMapping (ticket 12).
func (h *ConnectionHandler) TestCsvMapping(context.Context, *oas.CsvMappingTestUploadMultipart,
	oas.TestCsvMappingParams,
) (oas.TestCsvMappingRes, error) {
	return nil, httpapi.ErrNotImplemented
}
