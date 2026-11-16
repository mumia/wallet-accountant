package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/mumia/wallet-accountant/internal/ledger/domain"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
)

// Stream type and event types of the Account aggregate (ADR-0015).
const (
	AccountStreamType = "ledger.account"
	AccountAddedType  = "io.walletaccountant.ledger.account.added.v1"

	AccountOpeningBalanceRecordedType   = "io.walletaccountant.ledger.account.openingbalancerecorded.v1"
	AccountTransactionBookedType        = "io.walletaccountant.ledger.account.transactionbooked.v1"
	AccountReversalBookedType           = "io.walletaccountant.ledger.account.reversalbooked.v1"
	AccountTransactionRecategorisedType = "io.walletaccountant.ledger.account.transactionrecategorised.v1"
	AccountOpeningBalanceReversedType   = "io.walletaccountant.ledger.account.openingbalancereversed.v1"
	AccountMonthClosedType              = "io.walletaccountant.ledger.account.monthclosed.v1"
)

// ErrAccountNotFound is returned for an Account that is not in the Household.
var ErrAccountNotFound = errors.New("account not found")

// AccountAggregate is the Account Decider wired to the runtime.
type AccountAggregate = eventstore.Aggregate[domain.AccountState, domain.AccountCommand, domain.AccountEvent]

// NewAccountAggregate wires the Account Decider to its fold store.
func NewAccountAggregate(folds eventstore.FoldStore[domain.AccountState]) AccountAggregate {
	codec := eventstore.NewCodec[domain.AccountEvent](Source)
	eventstore.Register[domain.AccountEvent, domain.AccountAdded](codec, AccountAddedType)
	eventstore.Register[domain.AccountEvent, domain.OpeningBalanceRecorded](codec, AccountOpeningBalanceRecordedType)
	eventstore.Register[domain.AccountEvent, domain.TransactionBooked](codec, AccountTransactionBookedType)
	eventstore.Register[domain.AccountEvent, domain.ReversalBooked](codec, AccountReversalBookedType)
	eventstore.Register[domain.AccountEvent, domain.TransactionRecategorised](codec, AccountTransactionRecategorisedType)
	eventstore.Register[domain.AccountEvent, domain.OpeningBalanceReversed](codec, AccountOpeningBalanceReversedType)
	eventstore.Register[domain.AccountEvent, domain.MonthClosed](codec, AccountMonthClosedType)

	return AccountAggregate{
		StreamType: AccountStreamType,
		Decider:    domain.Account,
		Codec:      codec,
		Folds:      folds,
		// 2: the Opening balance and the running balance per Booking month.
		// 3: the Uncategorised count per Booking month.
		// 4: the Month close per Booking month.
		FoldSchemaVersion: 4,
	}
}

// Account is an Account as the API sees it. An Account awaiting its first
// Statement import has no balance (Balance is nil).
type Account struct {
	ID                           uuid.UUID
	Name                         string
	Kind                         domain.AccountKind
	Currency                     string
	StartDate                    time.Time
	AwaitingFirstStatementImport bool
	Balance                      *sharedkernel.Decimal
}

// AccountReader reads the Household's Account folds.
type AccountReader interface {
	// Accounts returns every Account of the Household the transaction is set
	// to, in the order they were added.
	Accounts(ctx context.Context, tx *eventstore.Tx) ([]Account, error)
}

// NewAccount is the request to add an Account.
type NewAccount struct {
	Name      string
	Kind      domain.AccountKind
	Currency  string
	StartDate time.Time
}

// Accounts manages a Household's Accounts.
type Accounts struct {
	scope    HouseholdScope
	accounts AccountAggregate
	reader   AccountReader
}

// NewAccounts returns the Accounts service.
func NewAccounts(scope HouseholdScope, accounts AccountAggregate, reader AccountReader) *Accounts {
	return &Accounts{scope: scope, accounts: accounts, reader: reader}
}

// Add adds an Account to the Household, awaiting its first Statement import
// (consistency map: Add Account).
func (a *Accounts) Add(ctx context.Context, household uuid.UUID, req NewAccount) (Account, error) {
	id, err := sharedkernel.NewID()
	if err != nil {
		return Account{}, err //nolint:wrapcheck // already wrapped by the shared kernel
	}

	var added Account

	err = a.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		res, err := a.accounts.Execute(ctx, tx, id, domain.AddAccount{
			Name: req.Name, Kind: req.Kind, Currency: req.Currency, StartDate: req.StartDate,
		})
		if err != nil {
			return err //nolint:wrapcheck // the Decider's refusal or the runtime's error
		}

		added = accountOf(id, res.State)

		return nil
	})
	if err != nil {
		return Account{}, fmt.Errorf("add account: %w", err)
	}

	return added, nil
}

// List returns every Account of the Household.
func (a *Accounts) List(ctx context.Context, household uuid.UUID) ([]Account, error) {
	var list []Account

	err := a.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		var err error
		list, err = a.reader.Accounts(ctx, tx)

		return err //nolint:wrapcheck // the adapter's error
	})
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}

	return list, nil
}

// Get returns one Account of the Household, read from its fold.
func (a *Accounts) Get(ctx context.Context, household, account uuid.UUID) (Account, error) {
	var got Account

	err := a.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		fold, found, err := a.accounts.Folds.Load(ctx, tx, account)
		if err != nil {
			return err //nolint:wrapcheck // the adapter's error
		}

		if !found {
			return ErrAccountNotFound
		}

		got = accountOf(account, fold.State)

		return nil
	})
	if err != nil {
		return Account{}, fmt.Errorf("get account %s: %w", account, err)
	}

	return got, nil
}

// AccountExists reports whether the Account belongs to the Household the
// transaction is set to. Ingestion calls it inside its request transaction.
func (a *Accounts) AccountExists(ctx context.Context, tx *eventstore.Tx, account uuid.UUID) (bool, error) {
	_, found, err := a.accounts.Folds.Load(ctx, tx, account)
	if err != nil {
		return false, fmt.Errorf("find account %s: %w", account, err)
	}

	return found, nil
}

// AccountForImport returns the Account as a Statement import needs it, read
// from its fold in the caller's transaction; ErrAccountNotFound if it isn't
// one of the Household's.
func (a *Accounts) AccountForImport(ctx context.Context, tx *eventstore.Tx, account uuid.UUID) (Account, error) {
	fold, found, err := a.accounts.Folds.Load(ctx, tx, account)
	if err != nil {
		return Account{}, fmt.Errorf("find account %s: %w", account, err)
	}

	if !found {
		return Account{}, ErrAccountNotFound
	}

	return accountOf(account, fold.State), nil
}

func accountOf(id uuid.UUID, s domain.AccountState) Account {
	a := Account{
		ID: id, Name: s.Name, Kind: s.Kind, Currency: s.Currency, StartDate: s.StartDate,
		AwaitingFirstStatementImport: s.AwaitingFirstStatementImport,
	}

	if s.HasBalance() {
		a.Balance = new(s.Balance())
	}

	return a
}
