package wiring

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	householdspg "github.com/mumia/wallet-accountant/internal/households/adapters/postgres"
	householdsapp "github.com/mumia/wallet-accountant/internal/households/app"
	ingestionpg "github.com/mumia/wallet-accountant/internal/ingestion/adapters/postgres"
	ingestionapp "github.com/mumia/wallet-accountant/internal/ingestion/app"
	ledgerpg "github.com/mumia/wallet-accountant/internal/ledger/adapters/postgres"
	ledgerapp "github.com/mumia/wallet-accountant/internal/ledger/app"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
	"github.com/mumia/wallet-accountant/internal/platform/forwarding"
)

// NewUsers builds the Households Users service on pool, which connects as
// wa_app.
func NewUsers(pool *pgxpool.Pool) *householdsapp.Users {
	return householdsapp.NewUsers(eventstore.NewStore(pool),
		householdsapp.NewInvitationAggregate(householdspg.InvitationFolds{}),
		householdsapp.NewUserAggregate(householdspg.UserFolds{}),
		householdspg.Lookups{},
		time.Now,
	)
}

// Contexts are the bounded contexts' application services, built on one
// wa_app pool.
type Contexts struct {
	Users      *householdsapp.Users
	Households *householdsapp.Households
	Categories *ledgerapp.Categories
	Accounts   *ledgerapp.Accounts
	// Connections checks Accounts through the Ledger.
	Connections *ingestionapp.Connections
	// Transactions lists Accounts' Transactions, records Opening balances,
	// and recategorises and reverses Transactions, returning reversed
	// Imported transactions to pending through Ingestion.
	Transactions *ledgerapp.Transactions
	// StatementImports records the first import's Opening balance through
	// the Ledger.
	StatementImports     *ingestionapp.StatementImports
	ImportedTransactions *ingestionapp.ImportedTransactions
	// Months shows and closes Accounts' months, reading unresolved Imported
	// transactions and Balance checks through Ingestion.
	Months        *ledgerapp.Months
	Notifications *householdsapp.Notifications
	// Invitations issues and lists Invitations to a Household.
	Invitations *householdsapp.Invitations
}

// NewContexts builds every bounded context's application services on pool,
// which connects as wa_app, wiring each cross-context port. now is the
// clock their transactions read (tx.Now), time.Now outside tests.
func NewContexts(pool *pgxpool.Pool, now func() time.Time) Contexts {
	store := eventstore.NewStore(pool, eventstore.WithClock(now))

	// Households seeds Internal Categories through the Ledger, and the Ledger
	// runs its requests through Households' request transaction.
	categories := &ledgerapp.Categories{}
	households := householdsapp.NewHouseholds(store,
		householdsapp.NewHouseholdAggregate(householdspg.HouseholdFolds{}),
		householdspg.Lookups{},
		householdspg.HouseholdLookups{},
		categories,
	)
	*categories = *ledgerapp.NewCategories(households,
		ledgerapp.NewCategoryAggregate(ledgerpg.CategoryFolds{}),
		ledgerpg.CategoryReader{},
	)

	// Accepting books through the Ledger, and reversing returns Imported
	// transactions to pending through Ingestion: imported is filled in once
	// the Ledger port exists.
	imported := &ingestionapp.ImportedTransactions{}
	accountAggregate := ledgerapp.NewAccountAggregate(ledgerpg.AccountFolds{})
	accounts := ledgerapp.NewAccounts(households, accountAggregate, ledgerpg.AccountReader{},
		ledgerIngestion{imported: imported})
	transactions := ledgerapp.NewTransactions(households, accountAggregate,
		ledgerapp.NewTransactionAggregate(ledgerpg.TransactionFolds{}),
		ledgerapp.NewCategoryAggregate(ledgerpg.CategoryFolds{}),
		ledgerpg.TransactionReader{}, ledgerpg.CategoryReader{}, ledgerIngestion{imported: imported})
	months := ledgerapp.NewMonths(households, accountAggregate, ledgerIngestion{imported: imported})
	connections := ingestionapp.NewConnectionAggregate(ingestionpg.ConnectionFolds{})
	ledger := ingestionLedger{accounts: accounts, transactions: transactions, months: months}
	rows := ingestionapp.NewImportedTransactionAggregate(ingestionpg.ImportedTransactionFolds{})
	statementImports := ingestionapp.NewStatementImportAggregate(ingestionpg.StatementImportFolds{})
	*imported = *ingestionapp.NewImportedTransactions(households, accounts, ledger, rows,
		ingestionpg.ImportedTransactionReader{}, statementImports, ingestionpg.StatementImportReader{})

	return Contexts{
		Users:        NewUsers(pool),
		Households:   households,
		Categories:   categories,
		Accounts:     accounts,
		Transactions: transactions,
		Connections:  ingestionapp.NewConnections(households, accounts, ledger, connections, ingestionpg.ConnectionReader{}),
		StatementImports: ingestionapp.NewStatementImports(households, ledger, connections, statementImports, rows,
			ingestionpg.StatementImportReader{}),
		ImportedTransactions: imported,
		Months:               months,
		Notifications: householdsapp.NewNotifications(households,
			householdsapp.NewNotificationAggregate(householdspg.NotificationFolds{}), householdspg.NotificationReader{}),
		Invitations: householdsapp.NewInvitations(households,
			householdsapp.NewInvitationAggregate(householdspg.InvitationFolds{}), householdspg.InvitationReader{}),
	}
}

// handlers lists every deferred event handler the worker runs. Each bounded
// context adds its Watermill handlers here.
func handlers() []forwarding.Handler {
	return []forwarding.Handler{
		suspectedDuplicatesNotifier(),
	}
}

// rebuildTargets lists every aggregate whose folds the rebuild run mode
// rebuilds. Each bounded context adds its aggregates here.
func rebuildTargets() []eventstore.RebuildTarget {
	return []eventstore.RebuildTarget{
		householdsapp.NewInvitationAggregate(householdspg.InvitationFolds{}).RebuildTarget(),
		householdsapp.NewUserAggregate(householdspg.UserFolds{}).RebuildTarget(),
		householdsapp.NewHouseholdAggregate(householdspg.HouseholdFolds{}).RebuildTarget(),
		ledgerapp.NewCategoryAggregate(ledgerpg.CategoryFolds{}).RebuildTarget(),
		ledgerapp.NewAccountAggregate(ledgerpg.AccountFolds{}).RebuildTarget(),
		ingestionapp.NewConnectionAggregate(ingestionpg.ConnectionFolds{}).RebuildTarget(),
		ledgerapp.NewTransactionAggregate(ledgerpg.TransactionFolds{}).RebuildTarget(),
		ingestionapp.NewStatementImportAggregate(ingestionpg.StatementImportFolds{}).RebuildTarget(),
		ingestionapp.NewImportedTransactionAggregate(ingestionpg.ImportedTransactionFolds{}).RebuildTarget(),
		householdsapp.NewNotificationAggregate(householdspg.NotificationFolds{}).RebuildTarget(),
	}
}

// ConsumerGroups names the consumer group of every deferred handler, for
// tests that drain pending events.
func ConsumerGroups() []string {
	hs := handlers()
	groups := make([]string, 0, len(hs))

	for _, h := range hs {
		groups = append(groups, h.Name)
	}

	return groups
}
