package wiring

import (
	"context"

	"github.com/jackc/pgx/v5"

	householdspg "github.com/mumia/wallet-accountant/internal/households/adapters/postgres"
	householdsapp "github.com/mumia/wallet-accountant/internal/households/app"
	ingestionapp "github.com/mumia/wallet-accountant/internal/ingestion/app"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
	"github.com/mumia/wallet-accountant/internal/platform/forwarding"
)

// SuspectedDuplicatesNotifier is the consumer group of the handler that
// tells a Household about Suspected duplicates. Renaming it replays every
// event.
const SuspectedDuplicatesNotifier = "households.suspected-duplicates-notifier"

// MembershipNotifier is the consumer group of the handler that tells a
// Household about members joining, leaving and being removed. Renaming it
// replays every event.
const MembershipNotifier = "households.membership-notifier"

// membershipNotifier raises a Households Notification for a
// household.memberadded or household.memberremoved event (consistency map:
// Join Household and Remove member / leave, deferred). The processed-events
// table makes each event raise at most one.
func membershipNotifier() forwarding.Handler {
	raiser := householdsapp.NewNotificationRaiser(householdsapp.NewNotificationAggregate(householdspg.NotificationFolds{}))

	return forwarding.Handler{
		Name: MembershipNotifier,
		Handle: func(ctx context.Context, tx pgx.Tx, ev eventstore.CloudEvent) error {
			return raiser.RaiseMembershipNotification(ctx, eventstore.HandlerTx(tx, ev), ev)
		},
	}
}

// suspectedDuplicatesNotifier raises a Households Notification when an
// Ingestion Statement import brings Suspected duplicates (consistency map:
// Upload Statement import, deferred). It translates between the two
// contexts' terms, so neither imports the other.
func suspectedDuplicatesNotifier() forwarding.Handler {
	raiser := householdsapp.NewNotificationRaiser(householdsapp.NewNotificationAggregate(householdspg.NotificationFolds{}))

	return forwarding.Handler{
		Name: SuspectedDuplicatesNotifier,
		Handle: func(ctx context.Context, tx pgx.Tx, ev eventstore.CloudEvent) error {
			found, ok, err := ingestionapp.SuspectedDuplicatesFoundIn(ev)
			if err != nil || !ok {
				return err //nolint:wrapcheck // already wrapped by Ingestion
			}

			return raiser.RaiseSuspectedDuplicatesFound(ctx, eventstore.HandlerTx(tx, ev),
				householdsapp.SuspectedDuplicatesFound{
					StatementImportID:   found.StatementImportID,
					ConnectionID:        found.ConnectionID,
					AccountID:           found.AccountID,
					SuspectedDuplicates: found.SuspectedDuplicates,
				})
		},
	}
}
