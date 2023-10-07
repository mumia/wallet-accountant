package definitions

import "encoding/json"

var _ json.Marshaler = &WalletAccountantError{}

type WalletAccountantErrorContext map[string]any

type WalletAccountantError struct {
	SourceError   error
	Message       string
	Code          string
	ContextFields WalletAccountantErrorContext
}

func (e WalletAccountantError) Error() string {
	return e.Message
}

func (e WalletAccountantError) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		map[string]any{
			"source_error": e.SourceError.Error(),
			"message":      e.Message,
			"code":         e.Code,
			"context":      e.ContextFields,
		},
	)
}
