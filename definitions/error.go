package definitions

import "encoding/json"

var _ error = &WalletAccountantError{}

type ErrorCode int
type ErrorReason string
type ErrorContext map[string]any

type WalletAccountantError struct {
	Reason  ErrorReason  `json:"error"`
	Code    ErrorCode    `json:"code"`
	Context ErrorContext `json:"context"`
}

func (error WalletAccountantError) Error() string {
	response, err := json.Marshal(error)
	if err != nil {
		panic(err)
	}

	return string(response)
}
