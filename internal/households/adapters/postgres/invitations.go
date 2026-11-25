package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mumia/wallet-accountant/internal/households/app"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
)

// InvitationReader implements app.InvitationReader.
type InvitationReader struct{}

var _ app.InvitationReader = InvitationReader{}

// OpenInvitations implements app.InvitationReader. It reads the Invitations
// in the Household's scope, then switches to instance scope to read their
// issuers' display names, so no display name is copied into the Household.
// Invitation IDs are UUIDv7, so they break a tie in issued_at by issue order.
func (InvitationReader) OpenInvitations(ctx context.Context, tx *eventstore.Tx, at time.Time) ([]app.OpenInvitation, error) {
	rows, err := tx.Query(ctx, `SELECT id, issued_by, issued_at, expires_at FROM household_invitations
		WHERE household_id = $1 AND redeemed_by IS NULL AND revoked_at IS NULL AND expires_at > $2
		ORDER BY issued_at, id`, tx.HouseholdID(), at)
	if err != nil {
		return nil, fmt.Errorf("list open invitations: %w", err)
	}

	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (app.OpenInvitation, error) {
		var inv app.OpenInvitation

		err := r.Scan(&inv.ID, &inv.IssuedBy.ID, &inv.IssuedAt, &inv.ExpiresAt)
		inv.IssuedAt, inv.ExpiresAt = inv.IssuedAt.UTC(), inv.ExpiresAt.UTC()

		return inv, err //nolint:wrapcheck // wrapped below
	})
	if err != nil {
		return nil, fmt.Errorf("list open invitations: %w", err)
	}

	if len(list) == 0 {
		return list, nil
	}

	issuers := make([]uuid.UUID, 0, len(list))
	for _, inv := range list {
		issuers = append(issuers, inv.IssuedBy.ID)
	}

	var names map[uuid.UUID]string

	err = inInstanceScope(ctx, tx, func() error {
		names, err = displayNames(ctx, tx, issuers)

		return err
	})
	if err != nil {
		return nil, err
	}

	for i := range list {
		list[i].IssuedBy.DisplayName = names[list[i].IssuedBy.ID]
	}

	return list, nil
}

// IsOpen implements app.InvitationReader.
func (InvitationReader) IsOpen(ctx context.Context, tx *eventstore.Tx, id uuid.UUID, at time.Time) (bool, error) {
	var open bool

	err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM household_invitations
		WHERE household_id = $1 AND id = $2 AND redeemed_by IS NULL AND revoked_at IS NULL AND expires_at > $3)`,
		tx.HouseholdID(), id, at).Scan(&open)
	if err != nil {
		return false, fmt.Errorf("check open invitation: %w", err)
	}

	return open, nil
}

// displayNames returns the display names of the Users who have one. The
// transaction must be in instance scope.
func displayNames(ctx context.Context, tx *eventstore.Tx, users []uuid.UUID) (map[uuid.UUID]string, error) {
	rows, err := tx.Query(ctx, `SELECT id, display_name FROM users WHERE id = ANY($1) AND display_name IS NOT NULL`, users)
	if err != nil {
		return nil, fmt.Errorf("read display names: %w", err)
	}

	names := map[uuid.UUID]string{}

	var (
		id   uuid.UUID
		name string
	)

	_, err = pgx.ForEachRow(rows, []any{&id, &name}, func() error {
		names[id] = name

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read display names: %w", err)
	}

	return names, nil
}
