// Package postgres is the Households PostgreSQL adapter: the fold stores of
// the Invitation, User, Household and Notification aggregates, with their
// lookup rows, and the lookups the application reads them by. Users, the
// bootstrap Invitation and every code lookup row are in instance scope
// (ADR-0017); an Invitation to a Household is in that Household's scope.
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

// InvitationFolds stores Invitation folds and their code lookup rows. The
// transaction's scope says which: in instance scope, the bootstrap (or an
// operator's) Invitation in invitations; set to a Household, an Invitation
// to that Household in household_invitations. Either way its code lookup row
// is in instance scope.
type InvitationFolds struct{}

var _ eventstore.FoldStore[domain.InvitationState] = InvitationFolds{}

// Load implements eventstore.FoldStore.
func (InvitationFolds) Load(ctx context.Context, tx *eventstore.Tx, id uuid.UUID) (eventstore.Fold[domain.InvitationState], bool, error) {
	if tx.HouseholdID() != uuid.Nil {
		return loadHouseholdInvitation(ctx, tx, id)
	}

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
	if tx.HouseholdID() != uuid.Nil {
		return saveHouseholdInvitation(ctx, tx, id, f, expected)
	}

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

	return saveInvitationCode(ctx, tx, id, s.CodeHash, nil)
}

// saveInvitationCode replaces the Invitation's code lookup row. The
// transaction must be in instance scope. household is the Invitation's
// Household, nil for an instance-level Invitation.
func saveInvitationCode(ctx context.Context, tx *eventstore.Tx, id uuid.UUID, hash domain.CodeHash, household *uuid.UUID) error {
	if _, err := tx.Exec(ctx, `DELETE FROM invitation_codes WHERE invitation_id = $1`, id); err != nil {
		return fmt.Errorf("replace invitation code: %w", err)
	}

	if _, err := tx.Exec(ctx, `INSERT INTO invitation_codes (code_hash, invitation_id, household_id) VALUES ($1, $2, $3)`,
		hash, id, household); err != nil {
		return fmt.Errorf("save invitation code: %w", err)
	}

	return nil
}

func loadHouseholdInvitation(ctx context.Context, tx *eventstore.Tx, id uuid.UUID) (eventstore.Fold[domain.InvitationState], bool, error) {
	var (
		f          eventstore.Fold[domain.InvitationState]
		redeemedBy *uuid.UUID
		redeemedAt *time.Time
		revokedAt  *time.Time
	)

	err := tx.QueryRow(ctx, `SELECT code_hash, issued_by, issued_at, expires_at, redeemed_by, redeemed_at, revoked_at,
		version, schema_version
		FROM household_invitations WHERE household_id = $1 AND id = $2`, tx.HouseholdID(), id).
		Scan(&f.State.CodeHash, &f.State.IssuedBy, &f.State.IssuedAt, &f.State.ExpiresAt, &redeemedBy, &redeemedAt,
			&revokedAt, &f.Version, &f.SchemaVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return eventstore.Fold[domain.InvitationState]{}, false, nil
	}

	if err != nil {
		return eventstore.Fold[domain.InvitationState]{}, false, fmt.Errorf("load household invitation: %w", err)
	}

	f.State.Issued, f.State.Role, f.State.HouseholdID = true, domain.RoleUser, tx.HouseholdID()

	if redeemedBy != nil {
		f.State.RedeemedBy = *redeemedBy
	}

	if redeemedAt != nil {
		f.State.RedeemedAt = *redeemedAt
	}

	if revokedAt != nil {
		f.State.RevokedAt = *revokedAt
	}

	return f, true, nil
}

// saveHouseholdInvitation writes the Household's row, then switches to
// instance scope for the code lookup row, and back.
func saveHouseholdInvitation(ctx context.Context, tx *eventstore.Tx, id uuid.UUID, f eventstore.Fold[domain.InvitationState],
	expected int64,
) error {
	household := tx.HouseholdID()
	s := f.State

	if s.HouseholdID != household {
		return fmt.Errorf("save invitation %s: %w", id, errWrongHousehold)
	}

	var redeemedBy *uuid.UUID
	if s.Redeemed() {
		redeemedBy = &s.RedeemedBy
	}

	var redeemedAt *time.Time
	if !s.RedeemedAt.IsZero() {
		redeemedAt = &s.RedeemedAt
	}

	var revokedAt *time.Time
	if s.Revoked() {
		revokedAt = &s.RevokedAt
	}

	args := []any{
		household, id, s.CodeHash, s.IssuedBy, s.IssuedAt, s.ExpiresAt, redeemedBy, redeemedAt,
		f.Version, f.SchemaVersion, revokedAt,
	}
	q := `INSERT INTO household_invitations (household_id, id, code_hash, issued_by, issued_at, expires_at,
	      redeemed_by, redeemed_at, version, schema_version, revoked_at)
	      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) ON CONFLICT (id) DO NOTHING`

	if expected != 0 {
		args = append(args, expected)
		q = `UPDATE household_invitations SET code_hash = $3, issued_by = $4, issued_at = $5, expires_at = $6,
		     redeemed_by = $7, redeemed_at = $8, version = $9, schema_version = $10, revoked_at = $11
		     WHERE household_id = $1 AND id = $2 AND version = $12`
	}

	tag, err := tx.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("save household invitation: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return eventstore.ErrVersionConflict
	}

	return inInstanceScope(ctx, tx, func() error {
		return saveInvitationCode(ctx, tx, id, s.CodeHash, &household)
	})
}

// UserFolds stores User folds and their identity lookup rows.
type UserFolds struct{}

var _ eventstore.FoldStore[domain.UserState] = UserFolds{}

// Load implements eventstore.FoldStore.
func (UserFolds) Load(ctx context.Context, tx *eventstore.Tx, id uuid.UUID) (eventstore.Fold[domain.UserState], bool, error) {
	var f eventstore.Fold[domain.UserState]

	err := tx.QueryRow(ctx, `SELECT oidc_issuer, oidc_subject, invitation_id, is_instance_administrator,
		COALESCE(display_name, ''), version, schema_version
		FROM users WHERE id = $1`, id).
		Scan(&f.State.Issuer, &f.State.Subject, &f.State.InvitationID, &f.State.IsInstanceAdministrator,
			&f.State.DisplayName, &f.Version, &f.SchemaVersion)
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

	args := []any{
		id, s.Issuer, s.Subject, s.InvitationID, s.IsInstanceAdministrator, f.Version, f.SchemaVersion,
		displayName(s.DisplayName),
	}
	q := `INSERT INTO users (id, oidc_issuer, oidc_subject, invitation_id, is_instance_administrator, version, schema_version,
	      display_name)
	      VALUES ($1, $2, $3, $4, $5, $6, $7, $8) ON CONFLICT (id) DO NOTHING`

	if expected != 0 {
		args = append(args, expected)
		q = `UPDATE users SET oidc_issuer = $2, oidc_subject = $3, invitation_id = $4, is_instance_administrator = $5,
		     version = $6, schema_version = $7, display_name = $8 WHERE id = $1 AND version = $9`
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

// displayName stores a User without a display name as NULL.
func displayName(name string) *string {
	if name == "" {
		return nil
	}

	return &name
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

	err := tx.QueryRow(ctx, `SELECT u.id, u.is_instance_administrator, COALESCE(u.display_name, '')
		FROM user_identities i JOIN users u ON u.id = i.user_id
		WHERE i.oidc_issuer = $1 AND i.oidc_subject = $2`, id.Issuer, id.Subject).
		Scan(&u.ID, &u.IsInstanceAdministrator, &u.DisplayName)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.User{}, false, nil
	}

	if err != nil {
		return app.User{}, false, fmt.Errorf("find user by identity: %w", err)
	}

	return u, true, nil
}
