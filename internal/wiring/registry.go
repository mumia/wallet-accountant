package wiring

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	householdspg "github.com/mumia/wallet-accountant/internal/households/adapters/postgres"
	householdsapp "github.com/mumia/wallet-accountant/internal/households/app"
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
