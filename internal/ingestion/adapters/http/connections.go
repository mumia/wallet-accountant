// Package http is the Ingestion HTTP adapter: the ogen operations for
// Connections and their CSV mappings.
package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/mumia/wallet-accountant/internal/ingestion/app"
	"github.com/mumia/wallet-accountant/internal/ingestion/domain"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi/oas"
)

// Problem types for a Connection or a CSV mapping the domain refuses.
const (
	InvalidConnectionType   = "/problems/invalid-connection"
	ConnectionNameTakenType = "/problems/connection-name-taken"
	InvalidCSVMappingType   = "/problems/invalid-csv-mapping"
)

// The API's spellings of the domain's values. A value the API doesn't know
// is passed on prefixed, so the domain refuses it as unknown even when it
// happens to spell a domain value. BANK_FEED is listed so that it gets its
// own refusal.
var (
	connectionKindsIn = map[oas.ConnectionKind]domain.ConnectionKind{
		"STATEMENT_FILE_IMPORT": domain.ConnectionKindStatementFileImport,
		"BANK_FEED":             "bank_feed",
	}
	connectionKindsOut = map[domain.ConnectionKind]oas.ConnectionKind{
		domain.ConnectionKindStatementFileImport: "STATEMENT_FILE_IMPORT",
	}
	amountLayoutsIn = map[string]domain.AmountLayout{
		"SIGNED":       domain.AmountLayoutSigned,
		"DEBIT_CREDIT": domain.AmountLayoutDebitCredit,
	}
	amountLayoutsOut = map[domain.AmountLayout]string{
		domain.AmountLayoutSigned:      "SIGNED",
		domain.AmountLayoutDebitCredit: "DEBIT_CREDIT",
	}
	encodingsIn = map[string]domain.Encoding{
		"UTF_8":        domain.EncodingUTF8,
		"WINDOWS_1252": domain.EncodingWindows1252,
		"UTF_16":       domain.EncodingUTF16,
	}
	encodingsOut = map[domain.Encoding]string{
		"":                         "UTF_8",
		domain.EncodingUTF8:        "UTF_8",
		domain.EncodingWindows1252: "WINDOWS_1252",
		domain.EncodingUTF16:       "UTF_16",
	}
	amountSignsIn = map[string]domain.AmountSign{
		"MONEY_IN":  domain.MoneyIn,
		"MONEY_OUT": domain.MoneyOut,
	}
	amountSignsOut = map[domain.AmountSign]string{
		domain.MoneyIn:  "MONEY_IN",
		domain.MoneyOut: "MONEY_OUT",
	}
)

// ConnectionHandler implements the Connection operations of oas.Handler.
type ConnectionHandler struct {
	connections *app.Connections
}

// NewConnectionHandler returns the Connection operations of the Ingestion
// HTTP adapter.
func NewConnectionHandler(connections *app.Connections) *ConnectionHandler {
	return &ConnectionHandler{connections: connections}
}

// CreateConnection implements createConnection.
func (h *ConnectionHandler) CreateConnection(ctx context.Context, req *oas.ConnectionCreation,
	params oas.CreateConnectionParams,
) (oas.CreateConnectionRes, error) {
	kind, ok := connectionKindsIn[req.Kind]
	if !ok {
		kind = domain.ConnectionKind("api:" + string(req.Kind))
	}

	var mapping *domain.CSVMapping

	if m, ok := req.CsvMapping.Get(); ok {
		in := mappingIn(m)
		mapping = &in
	}

	created, err := h.connections.Create(ctx, params.HouseholdID, params.AccountID, app.NewConnection{
		Name: string(req.Name), Kind: kind, CSVMapping: mapping,
	})

	invalid := func(problemType, title, detail string) (oas.CreateConnectionRes, error) {
		p := httpapi.Problem(http.StatusUnprocessableEntity, problemType, title, detail)

		return new(oas.CreateConnectionUnprocessableEntity(p)), nil
	}

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound), errors.Is(err, app.ErrAccountNotFound):
		return new(oas.CreateConnectionNotFound(httpapi.NotFoundProblem())), nil
	case errors.Is(err, domain.ErrConnectionNameInvalid):
		return invalid(InvalidConnectionType, "Invalid Connection", connectionNameInvalidDetail)
	case errors.Is(err, app.ErrConnectionNameTaken):
		return invalid(ConnectionNameTakenType, connectionNameTakenTitle, connectionNameTakenDetail(string(req.Name)))
	case errors.Is(err, domain.ErrBankFeedsNotSupported):
		return invalid(InvalidConnectionType, "Invalid Connection", "Bank feed Connections are not supported yet.")
	case errors.Is(err, domain.ErrUnknownConnectionKind):
		return invalid(InvalidConnectionType, "Invalid Connection", "The kind must be STATEMENT_FILE_IMPORT.")
	case errors.Is(err, app.ErrCashAccountConnection):
		return invalid(InvalidConnectionType, "Invalid Connection",
			"A cash Account is kept by hand and takes no Connection: record its Transactions directly.")
	case errors.Is(err, domain.ErrInvalidCSVMapping):
		return invalid(InvalidCSVMappingType, "Invalid CSV mapping", mappingDetail(err))
	case err != nil:
		return nil, fmt.Errorf("create connection: %w", err)
	}

	return connectionResponse(created), nil
}

// ListConnections implements listConnections.
func (h *ConnectionHandler) ListConnections(ctx context.Context, params oas.ListConnectionsParams) (oas.ListConnectionsRes, error) {
	list, err := h.connections.List(ctx, params.HouseholdID, params.AccountID)

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound), errors.Is(err, app.ErrAccountNotFound):
		return new(oas.ListConnectionsNotFound(httpapi.NotFoundProblem())), nil
	case err != nil:
		return nil, fmt.Errorf("list connections: %w", err)
	}

	items := make([]oas.Connection, 0, len(list))
	for _, c := range list {
		items = append(items, *connectionResponse(c))
	}

	return &oas.ConnectionList{Items: items}, nil
}

// ChangeConnection implements changeConnection: it renames the Connection.
func (h *ConnectionHandler) ChangeConnection(ctx context.Context, req *oas.ConnectionChange,
	params oas.ChangeConnectionParams,
) (oas.ChangeConnectionRes, error) {
	renamed, err := h.connections.Rename(ctx, params.HouseholdID, params.AccountID, params.ConnectionID, string(req.Name))

	invalid := func(problemType, title, detail string) (oas.ChangeConnectionRes, error) {
		p := httpapi.Problem(http.StatusUnprocessableEntity, problemType, title, detail)

		return new(oas.ChangeConnectionUnprocessableEntity(p)), nil
	}

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound), errors.Is(err, app.ErrConnectionNotFound):
		return new(oas.ChangeConnectionNotFound(httpapi.NotFoundProblem())), nil
	case errors.Is(err, domain.ErrConnectionNameInvalid):
		return invalid(InvalidConnectionType, "Invalid Connection", connectionNameInvalidDetail)
	case errors.Is(err, app.ErrConnectionNameTaken):
		return invalid(ConnectionNameTakenType, connectionNameTakenTitle, connectionNameTakenDetail(string(req.Name)))
	case err != nil:
		return nil, fmt.Errorf("rename connection: %w", err)
	}

	return connectionResponse(renamed), nil
}

// SaveCsvMapping implements saveCsvMapping.
func (h *ConnectionHandler) SaveCsvMapping(ctx context.Context, req *oas.CsvMapping, params oas.SaveCsvMappingParams) (oas.SaveCsvMappingRes, error) {
	saved, err := h.connections.SaveCSVMapping(ctx, params.HouseholdID, params.AccountID, params.ConnectionID, mappingIn(*req))

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound), errors.Is(err, app.ErrConnectionNotFound):
		return new(oas.SaveCsvMappingNotFound(httpapi.NotFoundProblem())), nil
	case errors.Is(err, domain.ErrInvalidCSVMapping):
		p := httpapi.Problem(http.StatusUnprocessableEntity, InvalidCSVMappingType, "Invalid CSV mapping", mappingDetail(err))

		return new(oas.SaveCsvMappingUnprocessableEntity(p)), nil
	case err != nil:
		return nil, fmt.Errorf("save CSV mapping: %w", err)
	}

	return connectionResponse(saved), nil
}

// The copy of a refused Connection name.
const (
	connectionNameInvalidDetail = "The name must be 1 to 100 characters, not counting spaces around it."
	connectionNameTakenTitle    = "Connection name taken"
)

// connectionNameTakenDetail names the refused name as the domain keeps it,
// trimmed.
func connectionNameTakenDetail(name string) string {
	return `This Account already has a Connection named "` + strings.TrimSpace(name) + `".`
}

// mappingDetail lists every problem of an invalid mapping.
func mappingDetail(err error) string {
	var invalid *domain.InvalidMappingError
	if !errors.As(err, &invalid) {
		return "The CSV mapping is invalid."
	}

	problems := make([]string, 0, len(invalid.Problems))
	for _, p := range invalid.Problems {
		problems = append(problems, p.Error())
	}

	return "The CSV mapping is invalid: " + strings.Join(problems, "; ") + "."
}

func mappingIn(m oas.CsvMapping) domain.CSVMapping {
	out := domain.CSVMapping{
		Delimiter:          m.Delimiter.Value,
		HasHeaderRow:       m.HasHeaderRow.Value,
		SkipLeadingLines:   int(m.SkipLeadingLines.Value),
		SkipTrailingLines:  int(m.SkipTrailingLines.Value),
		DateFormat:         m.DateFormat.Value,
		DecimalSeparator:   m.DecimalSeparator.Value,
		ThousandsSeparator: m.ThousandsSeparator.Value,
	}

	if v, ok := m.Encoding.Get(); ok {
		out.Encoding, ok = encodingsIn[v]
		if !ok {
			out.Encoding = domain.Encoding("api:" + v)
		}
	}

	if v, ok := m.AmountLayout.Get(); ok {
		out.AmountLayout, ok = amountLayoutsIn[v]
		if !ok {
			out.AmountLayout = domain.AmountLayout("api:" + v)
		}
	}

	if v, ok := m.PositiveAmountMeans.Get(); ok {
		out.PositiveAmountMeans, ok = amountSignsIn[v]
		if !ok {
			out.PositiveAmountMeans = domain.AmountSign("api:" + v)
		}
	}

	c := m.Columns.Value
	out.BookingDate = columnIn(c.BookingDate)
	out.Description = columnIn(c.Description)
	out.Amount = columnIn(c.Amount)
	out.Debit = columnIn(c.Debit)
	out.Credit = columnIn(c.Credit)
	out.TransactionDate = columnIn(c.TransactionDate)
	out.ValueDate = columnIn(c.ValueDate)
	out.Payee = columnIn(c.Payee)
	out.BankTransactionID = columnIn(c.BankTransactionID)
	out.Balance = columnIn(c.Balance)

	return out
}

func columnIn(c oas.OptCsvColumn) domain.Column {
	return domain.Column{Header: c.Value.Header.Value, Position: int(c.Value.Position.Value)}
}

func mappingOut(m domain.CSVMapping) oas.CsvMapping {
	out := oas.CsvMapping{
		Encoding:           oas.NewOptString(encodingsOut[m.Encoding]),
		Delimiter:          oas.NewOptString(m.Delimiter),
		HasHeaderRow:       oas.NewOptBool(m.HasHeaderRow),
		SkipLeadingLines:   oas.NewOptInt32(int32(m.SkipLeadingLines)),  //nolint:gosec // a mapping skips a handful of lines
		SkipTrailingLines:  oas.NewOptInt32(int32(m.SkipTrailingLines)), //nolint:gosec // a mapping skips a handful of lines
		DateFormat:         oas.NewOptString(m.DateFormat),
		DecimalSeparator:   oas.NewOptString(m.DecimalSeparator),
		ThousandsSeparator: oas.NewOptString(m.ThousandsSeparator),
		AmountLayout:       oas.NewOptString(amountLayoutsOut[m.AmountLayout]),
		Columns: oas.NewOptCsvColumns(oas.CsvColumns{
			BookingDate:       columnOut(m.BookingDate),
			Description:       columnOut(m.Description),
			Amount:            columnOut(m.Amount),
			Debit:             columnOut(m.Debit),
			Credit:            columnOut(m.Credit),
			TransactionDate:   columnOut(m.TransactionDate),
			ValueDate:         columnOut(m.ValueDate),
			Payee:             columnOut(m.Payee),
			BankTransactionID: columnOut(m.BankTransactionID),
			Balance:           columnOut(m.Balance),
		}),
	}

	if sign, ok := amountSignsOut[m.PositiveAmountMeans]; ok {
		out.PositiveAmountMeans = oas.NewOptString(sign)
	}

	return out
}

func columnOut(c domain.Column) oas.OptCsvColumn {
	switch {
	case c.Header != "":
		return oas.NewOptCsvColumn(oas.CsvColumn{Header: oas.NewOptString(c.Header)})
	case c.Position != 0:
		return oas.NewOptCsvColumn(oas.CsvColumn{Position: oas.NewOptInt32(int32(c.Position))}) //nolint:gosec // a column position is small
	default:
		return oas.OptCsvColumn{}
	}
}

func connectionResponse(c app.Connection) *oas.Connection {
	out := &oas.Connection{
		ConnectionID: c.ID,
		AccountID:    c.AccountID,
		Name:         oas.ConnectionName(c.Name),
		Kind:         connectionKindsOut[c.Kind],
	}

	if c.CSVMapping != nil {
		out.CsvMapping = oas.NewOptCsvMapping(mappingOut(*c.CSVMapping))
		out.CsvMappingVersion = oas.NewOptInt32(int32(c.MappingVersion)) //nolint:gosec // one version per save
	}

	return out
}
