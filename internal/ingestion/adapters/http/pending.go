package http

import (
	"context"

	"github.com/mumia/wallet-accountant/internal/platform/httpapi"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi/oas"
)

// The operations below are in the contract but not implemented yet: each
// answers 501 until its ticket in .scratch/ui-first-slice-backend lands.

// TestCsvMapping implements testCsvMapping (ticket 12).
func (h *ConnectionHandler) TestCsvMapping(context.Context, *oas.CsvMappingTestUploadMultipart,
	oas.TestCsvMappingParams,
) (oas.TestCsvMappingRes, error) {
	return nil, httpapi.ErrNotImplemented
}
