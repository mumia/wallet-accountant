// Package postgres is the Households PostgreSQL adapter: the fold stores of
// the Invitation and User aggregates, with their lookup rows, and the
// lookups the application reads them by. Every table is in instance scope
// (ADR-0017).
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mumia/wallet-accountant/internal/households/app"
	"github.com/mumia/wallet-accountant/internal/households/domain"
	"github.com/mumia/wallet-accountant/internal/platform/authn"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
)

// InvitationFolds stores Invitation folds and their code lookup rows.
type InvitationFolds struct{}

var _ eventstore.FoldStore[domain.InvitationState] = InvitationFolds{}

// Load implements eventstore.FoldStore.
func (InvitationFolds) Load(ctx context.Context, tx *eventstore.Tx, id uuid.UUID) (eventstore.Fold[domain.InvitationState], bool, error) {
	var (
		f          eventstore.Fold[domain.InvitationState]
		redeemedBy *uuid.UUID
		redeemedAt *time.Time
	)

	err := tx.QueryRow(ctx, `SELECT role, code_hash, expires_at, redeemed_by, redeemed_at, version, schema_version
		FROM invitations WHERE id = $1`, id).
		Scan(&f.State.Role, &f.State.CodeHash, &f.State.ExpiresAt, &redeemedBy, &redeemedAt, &f.Version, &f.SchemaVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return eventstore.Fold[domain.InvitationState]{}, false, nil
	}

	if err != nil {
		return eventstore.Fold[domain.InvitationState]{}, false, fmt.Errorf("load invitation: %w", err)
	}

	f.State.Issued = true

	if redeemedBy != nil {
		f.State.RedeemedBy = *redeemedBy
	}

	if redeemedAt != nil {
		f.State.RedeemedAt = *redeemedAt
	}

	return f, true, nil
}

// Save implements eventstore.FoldStore. The code lookup row is replaced, so
// a reissued Invitation is found only by its new code.
func (InvitationFolds) Save(ctx context.Context, tx *eventstore.Tx, id uuid.UUID, f eventstore.Fold[domain.InvitationState], expected int64) error {
	s := f.State

	var redeemedBy *uuid.UUID
	if s.Redeemed() {
		redeemedBy = &s.RedeemedBy
	}

	var redeemedAt *time.Time
	if !s.RedeemedAt.IsZero() {
		redeemedAt = &s.RedeemedAt
	}

	args := []any{id, s.Role, s.CodeHash, s.ExpiresAt, redeemedBy, redeemedAt, f.Version, f.SchemaVersion}
	q := `INSERT INTO invitations (id, role, code_hash, expires_at, redeemed_by, redeemed_at, version, schema_version)
	      VALUES ($1, $2, $3, $4, $5, $6, $7, $8) ON CONFLICT (id) DO NOTHING`

	if expected != 0 {
		args = append(args, expected)
		q = `UPDATE invitations SET role = $2, code_hash = $3, expires_at = $4, redeemed_by = $5, redeemed_at = $6,
		     version = $7, schema_version = $8 WHERE id = $1 AND version = $9`
	}

	tag, err := tx.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("save invitation: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return eventstore.ErrVersionConflict
	}

	if _, err := tx.Exec(ctx, `DELETE FROM invitation_codes WHERE invitation_id = $1`, id); err != nil {
		return fmt.Errorf("replace invitation code: %w", err)
	}

	if _, err := tx.Exec(ctx, `INSERT INTO invitation_codes (code_hash, invitation_id) VALUES ($1, $2)`, s.CodeHash, id); err != nil {
		return fmt.Errorf("save invitation code: %w", err)
	}

	return nil
}

// UserFolds stores User folds and their identity lookup rows.
type UserFolds struct{}

var _ eventstore.FoldStore[domain.UserState] = UserFolds{}

// Load implements eventstore.FoldStore.
func (UserFolds) Load(ctx context.Context, tx *eventstore.Tx, id uuid.UUID) (eventstore.Fold[domain.UserState], bool, error) {
	var f eventstore.Fold[domain.UserState]

	err := tx.QueryRow(ctx, `SELECT oidc_issuer, oidc_subject, invitation_id, is_instance_administrator, version, schema_version
		FROM users WHERE id = $1`, id).
		Scan(&f.State.Issuer, &f.State.Subject, &f.State.InvitationID, &f.State.IsInstanceAdministrator, &f.Version, &f.SchemaVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return eventstore.Fold[domain.UserState]{}, false, nil
	}

	if err != nil {
		return eventstore.Fold[domain.UserState]{}, false, fmt.Errorf("load user: %w", err)
	}

	f.State.Registered = true

	return f, true, nil
}

// Save implements eventstore.FoldStore. The identity lookup row's primary
// key stops a second User for the same identity: Save then returns
// app.ErrAlreadyRegistered.
func (UserFolds) Save(ctx context.Context, tx *eventstore.Tx, id uuid.UUID, f eventstore.Fold[domain.UserState], expected int64) error {
	s := f.State

	args := []any{id, s.Issuer, s.Subject, s.InvitationID, s.IsInstanceAdministrator, f.Version, f.SchemaVersion}
	q := `INSERT INTO users (id, oidc_issuer, oidc_subject, invitation_id, is_instance_administrator, version, schema_version)
	      VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (id) DO NOTHING`

	if expected != 0 {
		args = append(args, expected)
		q = `UPDATE users SET oidc_issuer = $2, oidc_subject = $3, invitation_id = $4, is_instance_administrator = $5,
		     version = $6, schema_version = $7 WHERE id = $1 AND version = $8`
	}

	tag, err := tx.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("save user: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return eventstore.ErrVersionConflict
	}

	// Saving the same User's row again (a rebuild) changes nothing; another
	// User's row for the identity leaves no row affected.
	tag, err = tx.Exec(ctx, `INSERT INTO user_identities (oidc_issuer, oidc_subject, user_id) VALUES ($1, $2, $3)
		ON CONFLICT (oidc_issuer, oidc_subject) DO UPDATE SET user_id = EXCLUDED.user_id
		WHERE user_identities.user_id = EXCLUDED.user_id`, s.Issuer, s.Subject, id)
	if err != nil {
		return fmt.Errorf("save user identity: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return app.ErrAlreadyRegistered
	}

	return nil
}

// Lookups implements app.Lookups.
type Lookups struct{}

var _ app.Lookups = Lookups{}

// InvitationByCode implements app.Lookups.
func (Lookups) InvitationByCode(ctx context.Context, tx *eventstore.Tx, hash domain.CodeHash) (uuid.UUID, bool, error) {
	var id uuid.UUID

	err := tx.QueryRow(ctx, `SELECT invitation_id FROM invitation_codes WHERE code_hash = $1`, hash).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}

	if err != nil {
		return uuid.Nil, false, fmt.Errorf("find invitation by code: %w", err)
	}

	return id, true, nil
}

// UserByIdentity implements app.Lookups.
func (Lookups) UserByIdentity(ctx context.Context, tx *eventstore.Tx, id authn.Identity) (app.User, bool, error) {
	var u app.User

	err := tx.QueryRow(ctx, `SELECT u.id, u.is_instance_administrator
		FROM user_identities i JOIN users u ON u.id = i.user_id
		WHERE i.oidc_issuer = $1 AND i.oidc_subject = $2`, id.Issuer, id.Subject).
		Scan(&u.ID, &u.IsInstanceAdministrator)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.User{}, false, nil
	}

	if err != nil {
		return app.User{}, false, fmt.Errorf("find user by identity: %w", err)
	}

	return u, true, nil
}
