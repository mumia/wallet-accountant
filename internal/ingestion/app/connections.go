package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	householdsapp "github.com/mumia/wallet-accountant/internal/households/app"
	"github.com/mumia/wallet-accountant/internal/ingestion/domain"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
)

// Stream type and event types of the Connection aggregate (ADR-0015).
const (
	Source = "/ingestion"

	ConnectionStreamType       = "ingestion.connection"
	ConnectionCreatedType      = "io.walletaccountant.ingestion.connection.created.v1"
	ConnectionMappingSavedType = "io.walletaccountant.ingestion.connection.csvmappingsaved.v1"
	ConnectionRenamedType      = "io.walletaccountant.ingestion.connection.renamed.v1"
)

var (
	// ErrHouseholdNotFound is returned for a Household the caller may not
	// know exists (Households' request transaction decides it).
	ErrHouseholdNotFound = householdsapp.ErrHouseholdNotFound
	// ErrAccountNotFound is returned for an Account that is not in the
	// Household.
	ErrAccountNotFound = errors.New("account not found")
	// ErrCashAccountConnection is returned for a Connection on a cash
	// Account, which is kept entirely by hand.
	ErrCashAccountConnection = errors.New("a cash account takes no connection")
	// ErrConnectionNotFound is returned for a Connection that is not one of
	// the Account's.
	ErrConnectionNotFound = errors.New("connection not found")
	// ErrConnectionNameTaken is returned for a Connection name another of
	// the Account's Connections has, ignoring case. The fold store's unique
	// index decides it.
	ErrConnectionNameTaken = errors.New("connection name taken")
)

// HouseholdScope is the Households port every Household-scoped request runs
// through: one transaction, the caller's User resolved and its membership
// checked, app.household_id set (ADR-0014).
type HouseholdScope interface {
	InHousehold(ctx context.Context, household uuid.UUID,
		fn func(ctx context.Context, tx *eventstore.Tx, user uuid.UUID) error) error
}

// Accounts is the Ledger port Ingestion checks Accounts through.
type Accounts interface {
	// AccountExists reports whether the Account belongs to the Household the
	// transaction is set to.
	AccountExists(ctx context.Context, tx *eventstore.Tx, account uuid.UUID) (bool, error)
}

// AccountCurrencies is the part of the Ledger port a mapping test reads its
// Account through.
type AccountCurrencies interface {
	// ImportAccount returns the Account, or ErrAccountNotFound.
	ImportAccount(ctx context.Context, tx *eventstore.Tx, account uuid.UUID) (ImportAccount, error)
}

// ConnectionAggregate is the Connection Decider wired to the runtime.
type ConnectionAggregate = eventstore.Aggregate[domain.ConnectionState, domain.ConnectionCommand, domain.ConnectionEvent]

// NewConnectionAggregate wires the Connection Decider to its fold store.
func NewConnectionAggregate(folds eventstore.FoldStore[domain.ConnectionState]) ConnectionAggregate {
	codec := eventstore.NewCodec[domain.ConnectionEvent](Source)
	eventstore.Register[domain.ConnectionEvent, domain.ConnectionCreated](codec, ConnectionCreatedType)
	eventstore.Register[domain.ConnectionEvent, domain.CSVMappingSaved](codec, ConnectionMappingSavedType)
	eventstore.Register[domain.ConnectionEvent, domain.ConnectionRenamed](codec, ConnectionRenamedType)

	return ConnectionAggregate{
		StreamType:        ConnectionStreamType,
		Decider:           domain.Connection,
		Codec:             codec,
		Folds:             folds,
		FoldSchemaVersion: 2,
	}
}

// Connection is a Connection as the API sees it. CSVMapping is nil and
// MappingVersion 0 while it has no mapping.
type Connection struct {
	ID             uuid.UUID
	AccountID      uuid.UUID
	Name           string
	Kind           domain.ConnectionKind
	CSVMapping     *domain.CSVMapping
	MappingVersion int
}

// ConnectionReader reads the Household's Connection folds.
type ConnectionReader interface {
	// Connections returns every Connection of the Account, in the Household
	// the transaction is set to, in the order they were created.
	Connections(ctx context.Context, tx *eventstore.Tx, account uuid.UUID) ([]Connection, error)
}

// NewConnection is the request to create a Connection.
type NewConnection struct {
	Name       string
	Kind       domain.ConnectionKind
	CSVMapping *domain.CSVMapping
}

// Connections manages the Connections of a Household's Accounts.
type Connections struct {
	scope       HouseholdScope
	accounts    Accounts
	currencies  AccountCurrencies
	connections ConnectionAggregate
	reader      ConnectionReader
}

// NewConnections returns the Connections service.
func NewConnections(scope HouseholdScope, accounts Accounts, currencies AccountCurrencies,
	connections ConnectionAggregate, reader ConnectionReader,
) *Connections {
	return &Connections{scope: scope, accounts: accounts, currencies: currencies, connections: connections, reader: reader}
}

// MappingTest is a sample statement file as an unsaved CSV mapping reads
// it, with the Account currency its amounts are in.
type MappingTest struct {
	domain.Sample

	Currency string
}

// TestMapping reads a sample statement file with an unsaved CSV mapping,
// validated exactly as a save validates it, in the Account currency and
// with no start-date filter (spec: Mapping test). It records nothing and
// keeps nothing.
func (c *Connections) TestMapping(ctx context.Context, household, account uuid.UUID, m domain.CSVMapping, file []byte,
) (MappingTest, error) {
	var currency sharedkernel.Currency

	err := c.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		a, err := c.currencies.ImportAccount(ctx, tx, account)
		currency = a.Currency

		return err //nolint:wrapcheck // the port's error
	})
	if err != nil {
		return MappingTest{}, fmt.Errorf("test CSV mapping: %w", err)
	}

	sample, err := domain.ReadSample(m, file, currency)
	if err != nil {
		return MappingTest{}, fmt.Errorf("test CSV mapping: %w", err)
	}

	return MappingTest{Sample: sample, Currency: currency.Code()}, nil
}

// Create creates a Connection for the Account, with its CSV mapping if one
// is given.
func (c *Connections) Create(ctx context.Context, household, account uuid.UUID, req NewConnection) (Connection, error) {
	id, err := sharedkernel.NewID()
	if err != nil {
		return Connection{}, err //nolint:wrapcheck // already wrapped by the shared kernel
	}

	var created Connection

	err = c.inAccount(ctx, household, account, func(ctx context.Context, tx *eventstore.Tx) error {
		target, err := c.currencies.ImportAccount(ctx, tx, account)
		if err != nil {
			return err //nolint:wrapcheck // the port's error
		}

		if target.Cash {
			return ErrCashAccountConnection
		}

		res, err := c.connections.Execute(ctx, tx, id, domain.CreateConnection{
			AccountID: account, Name: req.Name, Kind: req.Kind, CSVMapping: req.CSVMapping,
		})
		if err != nil {
			return err //nolint:wrapcheck // the Decider's refusal or the runtime's error
		}

		created = connectionOf(id, res.State)

		return nil
	})
	if err != nil {
		return Connection{}, fmt.Errorf("create connection: %w", err)
	}

	return created, nil
}

// List returns every Connection of the Account.
func (c *Connections) List(ctx context.Context, household, account uuid.UUID) ([]Connection, error) {
	var list []Connection

	err := c.inAccount(ctx, household, account, func(ctx context.Context, tx *eventstore.Tx) error {
		var err error
		list, err = c.reader.Connections(ctx, tx, account)

		return err //nolint:wrapcheck // the adapter's error
	})
	if err != nil {
		return nil, fmt.Errorf("list connections: %w", err)
	}

	return list, nil
}

// Rename gives the Connection a new name; the same name again records
// nothing.
func (c *Connections) Rename(ctx context.Context, household, account, connection uuid.UUID, name string) (Connection, error) {
	var renamed Connection

	err := c.inConnection(ctx, household, account, connection, func(ctx context.Context, tx *eventstore.Tx) error {
		res, err := c.connections.Execute(ctx, tx, connection, domain.RenameConnection{Name: name})
		if err != nil {
			return err //nolint:wrapcheck // the Decider's refusal or the runtime's error
		}

		renamed = connectionOf(connection, res.State)

		return nil
	})
	if err != nil {
		return Connection{}, fmt.Errorf("rename connection %s: %w", connection, err)
	}

	return renamed, nil
}

// SaveCSVMapping validates the mapping as a whole and saves it as the
// Connection's new mapping version (consistency map: Save CSV mapping).
func (c *Connections) SaveCSVMapping(ctx context.Context, household, account, connection uuid.UUID, m domain.CSVMapping) (Connection, error) {
	var saved Connection

	err := c.inConnection(ctx, household, account, connection, func(ctx context.Context, tx *eventstore.Tx) error {
		res, err := c.connections.Execute(ctx, tx, connection, domain.SaveCSVMapping{CSVMapping: m})
		if err != nil {
			return err //nolint:wrapcheck // the Decider's refusal or the runtime's error
		}

		saved = connectionOf(connection, res.State)

		return nil
	})
	if err != nil {
		return Connection{}, fmt.Errorf("save CSV mapping of connection %s: %w", connection, err)
	}

	return saved, nil
}

// inConnection runs fn in the Household's request transaction once the
// Connection is known to be one of the Account's.
func (c *Connections) inConnection(ctx context.Context, household, account, connection uuid.UUID,
	fn func(ctx context.Context, tx *eventstore.Tx) error,
) error {
	//nolint:wrapcheck // the callers wrap it
	return c.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		fold, found, err := c.connections.Folds.Load(ctx, tx, connection)
		if err != nil {
			return err //nolint:wrapcheck // the adapter's error
		}

		if !found || fold.State.AccountID != account {
			return ErrConnectionNotFound
		}

		return fn(ctx, tx)
	})
}

// inAccount runs fn in the Household's request transaction once the
// Account is known to be one of the Household's.
func (c *Connections) inAccount(ctx context.Context, household, account uuid.UUID,
	fn func(ctx context.Context, tx *eventstore.Tx) error,
) error {
	//nolint:wrapcheck // the callers wrap it
	return c.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		exists, err := c.accounts.AccountExists(ctx, tx, account)
		if err != nil {
			return err //nolint:wrapcheck // the port's error
		}

		if !exists {
			return ErrAccountNotFound
		}

		return fn(ctx, tx)
	})
}

func connectionOf(id uuid.UUID, s domain.ConnectionState) Connection {
	return Connection{
		ID: id, AccountID: s.AccountID, Name: s.Name, Kind: s.Kind, CSVMapping: s.CSVMapping, MappingVersion: s.MappingVersion,
	}
}
