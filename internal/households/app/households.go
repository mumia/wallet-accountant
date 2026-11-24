package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/mumia/wallet-accountant/internal/households/domain"
	"github.com/mumia/wallet-accountant/internal/platform/authn"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
)

var (
	// ErrHouseholdNotFound is returned for a Household the caller may not
	// know exists: one that doesn't exist, one they are not a member of, and
	// every Household when they are unregistered. They are not told apart,
	// so Household IDs can't be probed (spec "Authorisation").
	ErrHouseholdNotFound = errors.New("household not found")
	// ErrInvalidHousehold is returned for a Household the domain refuses to
	// create, for example with a blank name or an unknown Base currency.
	ErrInvalidHousehold = errors.New("invalid household")
	// errNoIdentity means the request was not authenticated; the HTTP
	// adapter's security handler makes it impossible.
	errNoIdentity = errors.New("request has no authenticated identity")
)

// Household is a Household as the API sees it.
type Household struct {
	ID           uuid.UUID
	Name         string
	BaseCurrency string
}

// HouseholdLookups reads the Household fold's lookup rows.
type HouseholdLookups interface {
	// HouseholdsOf returns the Households the User is a member of, oldest
	// first. It runs in instance scope.
	HouseholdsOf(ctx context.Context, tx *eventstore.Tx, user uuid.UUID) ([]uuid.UUID, error)
	// IsMember reports whether the User is a member of the Household the
	// transaction is set to.
	IsMember(ctx context.Context, tx *eventstore.Tx, user uuid.UUID) (bool, error)
	// Members returns the members of the Household the transaction is set
	// to, oldest first, with their display names, which it reads in
	// instance scope before setting the transaction back to the Household.
	Members(ctx context.Context, tx *eventstore.Tx) ([]Member, error)
}

// Member is a User who belongs to a Household, as the members list shows
// them.
type Member struct {
	UserID uuid.UUID
	// DisplayName is empty for a User who registered before display names
	// existed and hasn't set one.
	DisplayName string
	// JoinedAt is when they became a member.
	JoinedAt time.Time
}

// InternalCategories is the Ledger's port that seeds a new Household's
// Internal Categories, in the transaction that creates the Household, which
// is set to it (consistency map: Create Household).
type InternalCategories interface {
	SeedInternalCategories(ctx context.Context, tx *eventstore.Tx) error
}

// Households creates Households and runs every Household-scoped request.
type Households struct {
	store      *eventstore.Store
	households HouseholdAggregate
	users      Lookups
	lookups    HouseholdLookups
	categories InternalCategories
}

// NewHouseholds returns the Households service.
func NewHouseholds(store *eventstore.Store, households HouseholdAggregate, users Lookups, lookups HouseholdLookups,
	categories InternalCategories,
) *Households {
	return &Households{store: store, households: households, users: users, lookups: lookups, categories: categories}
}

// InHousehold runs fn as the caller of ctx in one database transaction as
// wa_app, scoped to the Household (ADR-0014): it resolves the caller's User
// in instance scope, then sets app.household_id to household before any
// Household-scoped query, and checks that the User is a member. An
// unregistered caller or a non-member gets ErrHouseholdNotFound. fn gets the
// User's ID, which the transaction's events are recorded for.
func (h *Households) InHousehold(ctx context.Context, household uuid.UUID,
	fn func(ctx context.Context, tx *eventstore.Tx, user uuid.UUID) error,
) error {
	id, ok := authn.IdentityFrom(ctx)
	if !ok {
		return errNoIdentity
	}

	err := h.store.InTx(ctx, eventstore.Scope{}, func(ctx context.Context, tx *eventstore.Tx) error {
		user, ok, err := h.users.UserByIdentity(ctx, tx, id)
		if err != nil {
			return err //nolint:wrapcheck // the adapter's error
		}

		if !ok {
			return ErrHouseholdNotFound
		}

		tx.SetActor(user.ID)

		if err := tx.SetHousehold(ctx, household); err != nil {
			return err //nolint:wrapcheck // already wrapped by the runtime
		}

		member, err := h.lookups.IsMember(ctx, tx, user.ID)
		if err != nil {
			return err //nolint:wrapcheck // the adapter's error
		}

		if !member {
			return ErrHouseholdNotFound
		}

		return fn(ctx, tx, user.ID)
	})
	if err != nil {
		return fmt.Errorf("in household %s: %w", household, err)
	}

	return nil
}

// Create creates a Household with the caller as its first member, and seeds
// its Internal Categories in the same transaction.
func (h *Households) Create(ctx context.Context, id authn.Identity, name, baseCurrency string) (Household, error) {
	householdID, err := sharedkernel.NewID()
	if err != nil {
		return Household{}, err //nolint:wrapcheck // already wrapped by the shared kernel
	}

	var created Household

	err = h.store.InTx(ctx, eventstore.Scope{}, func(ctx context.Context, tx *eventstore.Tx) error {
		user, err := h.resolve(ctx, tx, id)
		if err != nil {
			return err
		}

		tx.SetActor(user.ID)

		if err := tx.SetHousehold(ctx, householdID); err != nil {
			return err //nolint:wrapcheck // already wrapped by the runtime
		}

		res, err := h.households.Execute(ctx, tx, householdID,
			domain.CreateHousehold{Name: name, BaseCurrency: baseCurrency, CreatedBy: user.ID})
		if err != nil {
			return asInvalidHousehold(err)
		}

		if err := h.categories.SeedInternalCategories(ctx, tx); err != nil {
			return fmt.Errorf("seed internal categories: %w", err)
		}

		created = Household{ID: householdID, Name: res.State.Name, BaseCurrency: res.State.BaseCurrency}

		return nil
	})
	if err != nil {
		return Household{}, fmt.Errorf("create household: %w", err)
	}

	return created, nil
}

// List returns the Households the caller is a member of, oldest first.
func (h *Households) List(ctx context.Context, id authn.Identity) ([]Household, error) {
	var list []Household

	err := h.store.InTx(ctx, eventstore.Scope{}, func(ctx context.Context, tx *eventstore.Tx) error {
		list = nil

		user, err := h.resolve(ctx, tx, id)
		if err != nil {
			return err
		}

		ids, err := h.lookups.HouseholdsOf(ctx, tx, user.ID)
		if err != nil {
			return err //nolint:wrapcheck // the adapter's error
		}

		for _, householdID := range ids {
			if err := tx.SetHousehold(ctx, householdID); err != nil {
				return err //nolint:wrapcheck // already wrapped by the runtime
			}

			hh, err := h.load(ctx, tx, householdID)
			if err != nil {
				return err
			}

			list = append(list, hh)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list households: %w", err)
	}

	return list, nil
}

// Get returns a Household the caller of ctx is a member of.
func (h *Households) Get(ctx context.Context, household uuid.UUID) (Household, error) {
	var got Household

	err := h.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		var err error
		got, err = h.load(ctx, tx, household)

		return err
	})

	return got, err
}

// Members returns the members of a Household the caller of ctx is a member
// of, oldest first.
func (h *Households) Members(ctx context.Context, household uuid.UUID) ([]Member, error) {
	var members []Member

	err := h.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		var err error
		members, err = h.lookups.Members(ctx, tx)

		return err //nolint:wrapcheck // the adapter's error
	})

	return members, err
}

func (h *Households) resolve(ctx context.Context, tx *eventstore.Tx, id authn.Identity) (User, error) {
	user, ok, err := h.users.UserByIdentity(ctx, tx, id)
	if err != nil {
		return User{}, err //nolint:wrapcheck // the adapter's error
	}

	if !ok {
		return User{}, ErrRegistrationRequired
	}

	return user, nil
}

func (h *Households) load(ctx context.Context, tx *eventstore.Tx, household uuid.UUID) (Household, error) {
	fold, found, err := h.households.Folds.Load(ctx, tx, household)
	if err != nil {
		return Household{}, err //nolint:wrapcheck // the adapter's error
	}

	if !found {
		return Household{}, ErrHouseholdNotFound
	}

	return Household{ID: household, Name: fold.State.Name, BaseCurrency: fold.State.BaseCurrency}, nil
}

func asInvalidHousehold(err error) error {
	if errors.Is(err, domain.ErrHouseholdNameRequired) || errors.Is(err, sharedkernel.ErrUnknownCurrency) {
		return fmt.Errorf("%w: %w", ErrInvalidHousehold, err)
	}

	return err
}
