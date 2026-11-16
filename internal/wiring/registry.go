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
}

// NewContexts builds every bounded context's application services on pool,
// which connects as wa_app, wiring each cross-context port.
func NewContexts(pool *pgxpool.Pool) Contexts {
	store := eventstore.NewStore(pool)

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

	accounts := ledgerapp.NewAccounts(households,
		ledgerapp.NewAccountAggregate(ledgerpg.AccountFolds{}), ledgerpg.AccountReader{})

	return Contexts{
		Users:      NewUsers(pool),
		Households: households,
		Categories: categories,
		Accounts:   accounts,
		Connections: ingestionapp.NewConnections(households, accounts,
			ingestionapp.NewConnectionAggregate(ingestionpg.ConnectionFolds{}), ingestionpg.ConnectionReader{}),
	}
}

// handlers lists every deferred event handler the worker runs. Each bounded
// context adds its Watermill handlers here.
func handlers() []forwarding.Handler {
	return nil
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
