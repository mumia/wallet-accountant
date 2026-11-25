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
	AccountMonthReopenedType            = "io.walletaccountant.ledger.account.monthreopened.v1"
	AccountRenamedType                  = "io.walletaccountant.ledger.account.renamed.v1"
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
	eventstore.Register[domain.AccountEvent, domain.MonthReopened](codec, AccountMonthReopenedType)
	eventstore.Register[domain.AccountEvent, domain.AccountRenamed](codec, AccountRenamedType)

	return AccountAggregate{
		StreamType: AccountStreamType,
		Decider:    domain.Account,
		Codec:      codec,
		Folds:      folds,
		// 2: the Opening balance and the running balance per Booking month.
		// 3: the Uncategorised count per Booking month.
		// 4: the Month close per Booking month.
		// 5: a closed month flagged by every change that touches it.
		// 6: the Open month and the unreversed Transactions per Booking
		// month; flags gone (ADR-0024).
		// 7: a cash month's Cash count with its Month close.
		FoldSchemaVersion: 7,
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
	// OpenMonth is the Account's one open Booking month (MonthLayout).
	OpenMonth string
	// Pending and SuspectedDuplicates count the Account's Imported
	// transactions in those states, in every month, read through Ingestion
	// in the request's transaction.
	Pending             int
	SuspectedDuplicates int
}

// ImportedRows counts an Account's pending and Suspected duplicate Imported
// transactions.
type ImportedRows struct {
	Pending             int
	SuspectedDuplicates int
}

// AccountReader reads the Household's Account folds.
type AccountReader interface {
	// Accounts returns every Account of the Household the transaction is set
	// to, in the order they were added.
	Accounts(ctx context.Context, tx *eventstore.Tx) ([]Account, error)
	// MonthsForShare returns the Account's start date and Open month,
	// locking its fold row FOR SHARE until the transaction ends; false if
	// it isn't one of the Household's.
	MonthsForShare(ctx context.Context, tx *eventstore.Tx, account uuid.UUID) (AccountMonths, bool, error)
}

// AccountMonths is where an Account's months stand: every month from the
// start date's month up to the Open month is closed, and every month after
// it is not open yet (ADR-0024).
type AccountMonths struct {
	StartDate time.Time
	OpenMonth string
}

// NewAccount is the request to add an Account. A cash Account sends its
// Opening count twice; any other kind sends neither.
type NewAccount struct {
	Name                     string
	Kind                     domain.AccountKind
	Currency                 string
	StartDate                time.Time
	OpeningCount             *Count
	OpeningCountConfirmation *Count
}

// Accounts manages a Household's Accounts.
type Accounts struct {
	scope     HouseholdScope
	accounts  AccountAggregate
	reader    AccountReader
	ingestion Ingestion
	openings  OpeningBalances
}

// NewAccounts returns the Accounts service.
func NewAccounts(scope HouseholdScope, accounts AccountAggregate, reader AccountReader, ingestion Ingestion,
	openings OpeningBalances,
) *Accounts {
	return &Accounts{scope: scope, accounts: accounts, reader: reader, ingestion: ingestion, openings: openings}
}

// Add adds an Account to the Household. A statement Account awaits its
// first Statement import; a cash Account's Opening balance is recorded from
// its Opening count in the same transaction, after the two counts are
// checked equal (consistency map: Add Account, Add Account (cash)).
func (a *Accounts) Add(ctx context.Context, household uuid.UUID, req NewAccount) (Account, error) {
	id, err := sharedkernel.NewID()
	if err != nil {
		return Account{}, err //nolint:wrapcheck // already wrapped by the shared kernel
	}

	var added Account

	err = a.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		cash := req.Kind == domain.AccountKindCash

		count, err := openingCount(cash, req.Currency, req.OpeningCount, req.OpeningCountConfirmation)
		if err != nil {
			return err
		}

		res, err := a.accounts.Execute(ctx, tx, id, domain.AddAccount{
			Name: req.Name, Kind: req.Kind, Currency: req.Currency, StartDate: req.StartDate, OpeningCount: count,
		})
		if err != nil {
			return err //nolint:wrapcheck // the Decider's refusal or the runtime's error
		}

		if !cash {
			added = accountOf(id, res.State)

			return nil
		}

		opening, err := sharedkernel.NewID()
		if err != nil {
			return err //nolint:wrapcheck // already wrapped by the shared kernel
		}

		if err := a.openings.RecordOpeningBalance(ctx, tx, id, opening, *count); err != nil {
			return err //nolint:wrapcheck // the port's error
		}

		added, err = a.AccountForImport(ctx, tx, id)

		return err
	})
	if err != nil {
		return Account{}, fmt.Errorf("add account: %w", err)
	}

	return added, nil
}

// Rename gives the Account a new name; the same name again records nothing.
// Its kind, Account currency and start date never change.
func (a *Accounts) Rename(ctx context.Context, household, account uuid.UUID, name string) (Account, error) {
	var renamed Account

	err := a.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		found, err := a.AccountExists(ctx, tx, account)
		if err != nil {
			return err
		}

		if !found {
			return ErrAccountNotFound
		}

		res, err := a.accounts.Execute(ctx, tx, account, domain.RenameAccount{Name: name})
		if err != nil {
			return err //nolint:wrapcheck // the Decider's refusal or the runtime's error
		}

		renamed = accountOf(account, res.State)

		counts, err := a.ingestion.UnresolvedRows(ctx, tx, &account)
		if err != nil {
			return err //nolint:wrapcheck // the port's error
		}

		renamed.withImportedRows(counts[account])

		return nil
	})
	if err != nil {
		return Account{}, fmt.Errorf("rename account %s: %w", account, err)
	}

	return renamed, nil
}

// List returns every Account of the Household, with their pending and
// Suspected duplicate counts read in one grouped query.
func (a *Accounts) List(ctx context.Context, household uuid.UUID) ([]Account, error) {
	var list []Account

	err := a.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		var err error
		if list, err = a.reader.Accounts(ctx, tx); err != nil {
			return err //nolint:wrapcheck // the adapter's error
		}

		counts, err := a.ingestion.UnresolvedRows(ctx, tx, nil)
		if err != nil {
			return err //nolint:wrapcheck // the port's error
		}

		for i := range list {
			list[i].withImportedRows(counts[list[i].ID])
		}

		return nil
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

		counts, err := a.ingestion.UnresolvedRows(ctx, tx, &account)
		if err != nil {
			return err //nolint:wrapcheck // the port's error
		}

		got.withImportedRows(counts[account])

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

// MonthsForShare returns the Account's start date and Open month in the
// caller's transaction, locking its fold row FOR SHARE: a Close or Reopen
// of the Account waits until the transaction ends, so a check against the
// Open month holds for the whole transaction (consistency map: Keep or
// Discard Imported transaction). ErrAccountNotFound if it isn't one of the
// Household's.
func (a *Accounts) MonthsForShare(ctx context.Context, tx *eventstore.Tx, account uuid.UUID) (AccountMonths, error) {
	m, found, err := a.reader.MonthsForShare(ctx, tx, account)
	if err != nil {
		return AccountMonths{}, err //nolint:wrapcheck // already wrapped by the adapter
	}

	if !found {
		return AccountMonths{}, ErrAccountNotFound
	}

	return m, nil
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

func (a *Account) withImportedRows(r ImportedRows) {
	a.Pending, a.SuspectedDuplicates = r.Pending, r.SuspectedDuplicates
}

func accountOf(id uuid.UUID, s domain.AccountState) Account {
	a := Account{
		ID: id, Name: s.Name, Kind: s.Kind, Currency: s.Currency, StartDate: s.StartDate,
		AwaitingFirstStatementImport: s.AwaitingFirstStatementImport,
		OpenMonth:                    s.OpenMonth,
	}

	if s.HasBalance() {
		a.Balance = new(s.Balance())
	}

	return a
}
