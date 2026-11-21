package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/mumia/wallet-accountant/internal/ingestion/domain"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider/decidertest"
)

var accountID = sharedkernel.MustParseID("0196a000-0000-7000-8000-000000000001")

// signedMapping is a valid mapping of a headed file with one signed amount
// column.
func signedMapping() domain.CSVMapping {
	return domain.CSVMapping{
		Delimiter:           ";",
		HasHeaderRow:        true,
		DateFormat:          "DD/MM/YYYY",
		DecimalSeparator:    ",",
		ThousandsSeparator:  ".",
		AmountLayout:        domain.AmountLayoutSigned,
		PositiveAmountMeans: domain.MoneyIn,
		BookingDate:         domain.Column{Header: "Date"},
		Description:         domain.Column{Header: "Description"},
		Amount:              domain.Column{Header: "Amount"},
	}
}

// debitCreditMapping is a valid mapping of a headerless file with separate
// debit and credit columns, mapped by position.
func debitCreditMapping() domain.CSVMapping {
	return domain.CSVMapping{
		Delimiter:        ",",
		DateFormat:       "YYYY-MM-DD",
		DecimalSeparator: ".",
		AmountLayout:     domain.AmountLayoutDebitCredit,
		BookingDate:      domain.Column{Position: 1},
		Description:      domain.Column{Position: 2},
		Debit:            domain.Column{Position: 3},
		Credit:           domain.Column{Position: 4},
	}
}

func created() domain.ConnectionEvent {
	return domain.ConnectionCreated{AccountID: accountID, Name: "Statement file", Kind: domain.ConnectionKindStatementFileImport}
}

func TestCreatingAStatementFileImportConnection(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Connection).
		When(domain.CreateConnection{AccountID: accountID, Name: "Statement file", Kind: domain.ConnectionKindStatementFileImport}).
		Then(created())
}

func TestAConnectionNameIsTrimmed(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Connection).
		When(domain.CreateConnection{AccountID: accountID, Name: "  Statement file ", Kind: domain.ConnectionKindStatementFileImport}).
		Then(created())
}

func TestCreatingAConnectionWithItsMapping(t *testing.T) {
	t.Parallel()

	m := signedMapping()

	decidertest.Given(t, domain.Connection).
		When(domain.CreateConnection{AccountID: accountID, Name: "Statement file", Kind: domain.ConnectionKindStatementFileImport, CSVMapping: &m}).
		Then(created(), domain.CSVMappingSaved{CSVMapping: m, MappingVersion: 1})
}

func TestCreatingAConnectionRefusals(t *testing.T) {
	t.Parallel()

	invalid := signedMapping()
	invalid.DateFormat = "someday"

	for name, tc := range map[string]struct {
		cmd  domain.CreateConnection
		want error
	}{
		"bank feed":    {domain.CreateConnection{AccountID: accountID, Name: "Statement file", Kind: "bank_feed"}, domain.ErrBankFeedsNotSupported},
		"unknown kind": {domain.CreateConnection{AccountID: accountID, Name: "Statement file", Kind: "carrier_pigeon"}, domain.ErrUnknownConnectionKind},
		"no Account":   {domain.CreateConnection{Name: "Statement file", Kind: domain.ConnectionKindStatementFileImport}, domain.ErrConnectionAccountRequired},
		"blank name": {
			domain.CreateConnection{AccountID: accountID, Name: "   ", Kind: domain.ConnectionKindStatementFileImport},
			domain.ErrConnectionNameInvalid,
		},
		"name too long": {
			domain.CreateConnection{AccountID: accountID, Name: strings.Repeat("é", 101), Kind: domain.ConnectionKindStatementFileImport},
			domain.ErrConnectionNameInvalid,
		},
		"invalid mapping": {
			domain.CreateConnection{AccountID: accountID, Name: "Statement file", Kind: domain.ConnectionKindStatementFileImport, CSVMapping: &invalid},
			domain.ErrUnknownDateFormat,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			decidertest.Given(t, domain.Connection).When(tc.cmd).ThenError(tc.want)
		})
	}

	t.Run("already created", func(t *testing.T) {
		t.Parallel()

		decidertest.Given(t, domain.Connection, created()).
			When(domain.CreateConnection{AccountID: accountID, Name: "Statement file", Kind: domain.ConnectionKindStatementFileImport}).
			ThenError(domain.ErrConnectionAlreadyCreated)
	})
}

func TestEverySaveOfTheMappingChangesItsVersion(t *testing.T) {
	t.Parallel()

	first, second := signedMapping(), debitCreditMapping()

	decidertest.Given(t, domain.Connection, created()).
		When(domain.SaveCSVMapping{CSVMapping: first}).
		Then(domain.CSVMappingSaved{CSVMapping: first, MappingVersion: 1})

	saved := decidertest.Given(t, domain.Connection, created(),
		domain.ConnectionEvent(domain.CSVMappingSaved{CSVMapping: first, MappingVersion: 1}))

	saved.When(domain.SaveCSVMapping{CSVMapping: second}).
		Then(domain.CSVMappingSaved{CSVMapping: second, MappingVersion: 2})

	// Saving the same mapping again is still a new version.
	saved.When(domain.SaveCSVMapping{CSVMapping: first}).
		Then(domain.CSVMappingSaved{CSVMapping: first, MappingVersion: 2})

	s := saved.State()
	if s.CSVMapping == nil || *s.CSVMapping != first || s.MappingVersion != 1 || s.AccountID != accountID {
		t.Errorf("state = %+v, want the first mapping at version 1 for the Account", s)
	}
}

func TestSavingAMappingOfAConnectionNotCreated(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Connection).
		When(domain.SaveCSVMapping{CSVMapping: signedMapping()}).
		ThenError(domain.ErrConnectionNotCreated)
}

func TestValidMappings(t *testing.T) {
	t.Parallel()

	for name, mutate := range map[string]func(*domain.CSVMapping){
		"signed by header":         func(*domain.CSVMapping) {},
		"inverted sign":            func(m *domain.CSVMapping) { m.PositiveAmountMeans = domain.MoneyOut },
		"positional with a header": func(m *domain.CSVMapping) { m.Amount = domain.Column{Position: 5} },
		"tab delimiter":            func(m *domain.CSVMapping) { m.Delimiter = "\t" },
		"no thousands separator":   func(m *domain.CSVMapping) { m.ThousandsSeparator = "" },
		"skipped leading lines":    func(m *domain.CSVMapping) { m.SkipLeadingLines = 3 },
		"every optional field":     everyOptionalField,
		"compact date format":      func(m *domain.CSVMapping) { m.DateFormat = "YYYYMMDD" },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			m := signedMapping()
			mutate(&m)

			decidertest.Given(t, domain.Connection, created()).
				When(domain.SaveCSVMapping{CSVMapping: m}).
				Then(domain.CSVMappingSaved{CSVMapping: m, MappingVersion: 1})
		})
	}

	t.Run("debit and credit by position", func(t *testing.T) {
		t.Parallel()

		m := debitCreditMapping()
		decidertest.Given(t, domain.Connection, created()).
			When(domain.SaveCSVMapping{CSVMapping: m}).
			Then(domain.CSVMappingSaved{CSVMapping: m, MappingVersion: 1})
	})
}

func everyOptionalField(m *domain.CSVMapping) {
	m.TransactionDate = domain.Column{Header: "Transaction date"}
	m.ValueDate = domain.Column{Header: "Value date"}
	m.Payee = domain.Column{Header: "Payee"}
	m.BankTransactionID = domain.Column{Header: "Reference"}
}

func TestInvalidMappings(t *testing.T) {
	t.Parallel()

	type mutation = func(*domain.CSVMapping)

	for name, tc := range map[string]struct {
		base   func() domain.CSVMapping
		mutate mutation
		want   error
	}{
		"no Booking date": {signedMapping, func(m *domain.CSVMapping) { m.BookingDate = domain.Column{} }, domain.ErrMissingMappedField},
		"no description":  {signedMapping, func(m *domain.CSVMapping) { m.Description = domain.Column{} }, domain.ErrMissingMappedField},
		"no amount":       {signedMapping, func(m *domain.CSVMapping) { m.Amount = domain.Column{} }, domain.ErrMissingMappedField},
		"no sign":         {signedMapping, func(m *domain.CSVMapping) { m.PositiveAmountMeans = "" }, domain.ErrMissingMappedField},
		"unknown sign":    {signedMapping, func(m *domain.CSVMapping) { m.PositiveAmountMeans = "sideways" }, domain.ErrInvalidCSVMapping},
		"no debit":        {debitCreditMapping, func(m *domain.CSVMapping) { m.Debit = domain.Column{} }, domain.ErrMissingMappedField},
		"no credit":       {debitCreditMapping, func(m *domain.CSVMapping) { m.Credit = domain.Column{} }, domain.ErrMissingMappedField},
		"signed and debit": {
			signedMapping, func(m *domain.CSVMapping) { m.Debit = domain.Column{Position: 9} }, domain.ErrInvalidCSVMapping,
		},
		"debit and credit and signed": {
			debitCreditMapping, func(m *domain.CSVMapping) { m.Amount = domain.Column{Position: 9} }, domain.ErrInvalidCSVMapping,
		},
		"sign with debit and credit": {
			debitCreditMapping, func(m *domain.CSVMapping) { m.PositiveAmountMeans = domain.MoneyOut }, domain.ErrInvalidCSVMapping,
		},
		"no amount layout":      {signedMapping, func(m *domain.CSVMapping) { m.AmountLayout = "" }, domain.ErrMissingMappedField},
		"unknown amount layout": {signedMapping, func(m *domain.CSVMapping) { m.AmountLayout = "abacus" }, domain.ErrInvalidCSVMapping},
		"no date format":        {signedMapping, func(m *domain.CSVMapping) { m.DateFormat = "" }, domain.ErrUnknownDateFormat},
		"unknown date format":   {signedMapping, func(m *domain.CSVMapping) { m.DateFormat = "02/01/2006" }, domain.ErrUnknownDateFormat},
		"same header twice": {
			signedMapping, func(m *domain.CSVMapping) { m.Description = domain.Column{Header: " date "} }, domain.ErrColumnUsedTwice,
		},
		"same position twice": {
			debitCreditMapping, func(m *domain.CSVMapping) { m.Payee = domain.Column{Position: 3} }, domain.ErrColumnUsedTwice,
		},
		"header without a header row": {
			debitCreditMapping, func(m *domain.CSVMapping) { m.Payee = domain.Column{Header: "Payee"} }, domain.ErrInvalidCSVMapping,
		},
		"header and position": {
			signedMapping, func(m *domain.CSVMapping) { m.Amount = domain.Column{Header: "Amount", Position: 3} }, domain.ErrInvalidCSVMapping,
		},
		"negative position": {
			debitCreditMapping, func(m *domain.CSVMapping) { m.Payee = domain.Column{Position: -1} }, domain.ErrInvalidCSVMapping,
		},
		"blank header":    {signedMapping, func(m *domain.CSVMapping) { m.Payee = domain.Column{Header: "  "} }, domain.ErrInvalidCSVMapping},
		"no delimiter":    {signedMapping, func(m *domain.CSVMapping) { m.Delimiter = "" }, domain.ErrInvalidCSVMapping},
		"long delimiter":  {signedMapping, func(m *domain.CSVMapping) { m.Delimiter = ";;" }, domain.ErrInvalidCSVMapping},
		"quote delimiter": {signedMapping, func(m *domain.CSVMapping) { m.Delimiter = `"` }, domain.ErrInvalidCSVMapping},
		"unknown decimal separator": {
			signedMapping, func(m *domain.CSVMapping) { m.DecimalSeparator = "·" }, domain.ErrInvalidCSVMapping,
		},
		"no decimal separator": {signedMapping, func(m *domain.CSVMapping) { m.DecimalSeparator = "" }, domain.ErrInvalidCSVMapping},
		"unknown thousands separator": {
			signedMapping, func(m *domain.CSVMapping) { m.ThousandsSeparator = "_" }, domain.ErrInvalidCSVMapping,
		},
		"same separators":        {signedMapping, func(m *domain.CSVMapping) { m.ThousandsSeparator = "," }, domain.ErrInvalidCSVMapping},
		"negative skipped lines": {signedMapping, func(m *domain.CSVMapping) { m.SkipLeadingLines = -1 }, domain.ErrInvalidCSVMapping},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			m := tc.base()
			tc.mutate(&m)

			decidertest.Given(t, domain.Connection, created()).
				When(domain.SaveCSVMapping{CSVMapping: m}).
				ThenError(tc.want)

			// Every refusal is also an invalid mapping.
			decidertest.Given(t, domain.Connection, created()).
				When(domain.SaveCSVMapping{CSVMapping: m}).
				ThenError(domain.ErrInvalidCSVMapping)
		})
	}
}

func TestAMappingIsValidatedAsAWhole(t *testing.T) {
	t.Parallel()

	m := signedMapping()
	m.BookingDate = domain.Column{}
	m.DateFormat = "whenever"
	m.Payee = domain.Column{Header: "amount"}

	err := m.Validate()
	for _, want := range []error{domain.ErrMissingMappedField, domain.ErrUnknownDateFormat, domain.ErrColumnUsedTwice} {
		if !errors.Is(err, want) {
			t.Errorf("Validate() = %v, want it to include %v", err, want)
		}
	}
}
