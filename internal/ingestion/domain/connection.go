package domain

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider"
)

// ConnectionKind is where a Connection's transactions come from.
type ConnectionKind string

// The Connection kinds. Only statement file imports exist in this slice;
// live bank feeds are refused.
const (
	ConnectionKindStatementFileImport ConnectionKind = "statement_file_import"

	connectionKindBankFeed ConnectionKind = "bank_feed"
)

var (
	// ErrConnectionAlreadyCreated is returned for creating a Connection twice.
	ErrConnectionAlreadyCreated = errors.New("connection already created")
	// ErrConnectionNotCreated is returned for a command to a Connection that
	// doesn't exist.
	ErrConnectionNotCreated = errors.New("connection not created")
	// ErrConnectionAccountRequired is returned for a Connection without an
	// Account.
	ErrConnectionAccountRequired = errors.New("connection account required")
	// ErrUnknownConnectionKind is returned for a kind that is not a
	// Connection kind.
	ErrUnknownConnectionKind = errors.New("unknown connection kind")
	// ErrConnectionNameInvalid is returned for a Connection name that is
	// blank or longer than ConnectionNameMaxLength once trimmed.
	ErrConnectionNameInvalid = errors.New("connection name must be 1 to 100 characters")
	// ErrBankFeedsNotSupported is returned for creating a live bank feed.
	ErrBankFeedsNotSupported = errors.New("bank feed connections are not supported yet")
)

// ConnectionState is a Connection's fold. MappingVersion counts the saves of
// its CSV mapping, so it changes on every save; it is 0 while the Connection
// has no mapping.
type ConnectionState struct {
	Created        bool
	AccountID      sharedkernel.ID
	Name           string
	Kind           ConnectionKind
	CSVMapping     *CSVMapping
	MappingVersion int
}

type (
	// ConnectionCommand is a command to a Connection.
	ConnectionCommand interface{ isConnectionCommand() }

	// CreateConnection links an Account to a source of its transactions,
	// optionally with the CSV mapping of its statement files.
	CreateConnection struct {
		AccountID  sharedkernel.ID
		Name       string
		Kind       ConnectionKind
		CSVMapping *CSVMapping
	}

	// SaveCSVMapping replaces the Connection's CSV mapping.
	SaveCSVMapping struct {
		CSVMapping CSVMapping
	}
)

func (CreateConnection) isConnectionCommand() {}
func (SaveCSVMapping) isConnectionCommand()   {}

type (
	// ConnectionEvent is a Connection's event.
	ConnectionEvent interface{ isConnectionEvent() }

	// ConnectionCreated is recorded when a Connection is created.
	ConnectionCreated struct {
		AccountID sharedkernel.ID `json:"accountid"`
		Name      string          `json:"name"`
		Kind      ConnectionKind  `json:"kind"`
	}

	// CSVMappingSaved is recorded each time a valid CSV mapping is saved,
	// with the mapping's new version.
	CSVMappingSaved struct {
		CSVMapping     CSVMapping `json:"csvmapping"`
		MappingVersion int        `json:"mappingversion"`
	}
)

func (ConnectionCreated) isConnectionEvent() {}
func (CSVMappingSaved) isConnectionEvent()   {}

// Connection is the Connection Decider.
var Connection = decider.Decider[ConnectionState, ConnectionCommand, ConnectionEvent]{
	Decide: decideConnection,
	Evolve: evolveConnection,
}

func decideConnection(s ConnectionState, c ConnectionCommand) ([]ConnectionEvent, error) {
	switch c := c.(type) {
	case CreateConnection:
		return createConnection(s, c)
	case SaveCSVMapping:
		if !s.Created {
			return nil, ErrConnectionNotCreated
		}

		return saveMapping(s, c.CSVMapping)
	default:
		return nil, nil
	}
}

func createConnection(s ConnectionState, c CreateConnection) ([]ConnectionEvent, error) {
	if s.Created {
		return nil, ErrConnectionAlreadyCreated
	}

	if c.AccountID == sharedkernel.NilID {
		return nil, ErrConnectionAccountRequired
	}

	name, err := connectionName(c.Name)
	if err != nil {
		return nil, err
	}

	switch c.Kind {
	case ConnectionKindStatementFileImport:
	case connectionKindBankFeed:
		return nil, ErrBankFeedsNotSupported
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownConnectionKind, c.Kind)
	}

	events := []ConnectionEvent{ConnectionCreated{AccountID: c.AccountID, Name: name, Kind: c.Kind}}

	if c.CSVMapping != nil {
		saved, err := saveMapping(s, *c.CSVMapping)
		if err != nil {
			return nil, err
		}

		events = append(events, saved...)
	}

	return events, nil
}

// ConnectionNameMaxLength is the longest Connection name, in characters.
const ConnectionNameMaxLength = 100

// connectionName is a Connection name trimmed, refused when blank or too
// long. Uniqueness within the Account is the fold store's.
func connectionName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > ConnectionNameMaxLength {
		return "", ErrConnectionNameInvalid
	}

	return name, nil
}

func saveMapping(s ConnectionState, m CSVMapping) ([]ConnectionEvent, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}

	return []ConnectionEvent{CSVMappingSaved{CSVMapping: m, MappingVersion: s.MappingVersion + 1}}, nil
}

func evolveConnection(s ConnectionState, e decider.Envelope[ConnectionEvent]) ConnectionState {
	switch ev := e.Event.(type) {
	case ConnectionCreated:
		s.Created, s.AccountID, s.Name, s.Kind = true, ev.AccountID, ev.Name, ev.Kind
	case CSVMappingSaved:
		m := ev.CSVMapping
		s.CSVMapping, s.MappingVersion = &m, ev.MappingVersion
	}

	return s
}
