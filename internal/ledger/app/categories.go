package app

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	householdsapp "github.com/mumia/wallet-accountant/internal/households/app"
	"github.com/mumia/wallet-accountant/internal/ledger/domain"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
)

// Stream types and event types of the Ledger aggregates (ADR-0015).
const (
	Source = "/ledger"

	CategoryStreamType  = "ledger.category"
	CategoryCreatedType = "io.walletaccountant.ledger.category.created.v1"
)

// CategoryAggregate is the Category Decider wired to the runtime.
type CategoryAggregate = eventstore.Aggregate[domain.CategoryState, domain.CategoryCommand, domain.CategoryEvent]

// NewCategoryAggregate wires the Category Decider to its fold store.
func NewCategoryAggregate(folds eventstore.FoldStore[domain.CategoryState]) CategoryAggregate {
	codec := eventstore.NewCodec[domain.CategoryEvent](Source)
	eventstore.Register[domain.CategoryEvent, domain.CategoryCreated](codec, CategoryCreatedType)

	return CategoryAggregate{
		StreamType:        CategoryStreamType,
		Decider:           domain.Category,
		Codec:             codec,
		Folds:             folds,
		FoldSchemaVersion: 1,
	}
}

// ErrHouseholdNotFound is returned for a Household the caller may not know
// exists (Households' request transaction decides it).
var ErrHouseholdNotFound = householdsapp.ErrHouseholdNotFound

// HouseholdScope is the Households port every Household-scoped request runs
// through: one transaction, the caller's User resolved and its membership
// checked, app.household_id set (ADR-0014).
type HouseholdScope interface {
	InHousehold(ctx context.Context, household uuid.UUID,
		fn func(ctx context.Context, tx *eventstore.Tx, user uuid.UUID) error) error
}

// Category is a Category as the API sees it. ParentID is uuid.Nil for a
// root Category.
type Category struct {
	ID       uuid.UUID
	Name     string
	Kind     domain.Kind
	ParentID uuid.UUID
}

// CategoryReader reads the Household's Category folds.
type CategoryReader interface {
	// Categories returns every Category of the Household the transaction is
	// set to.
	Categories(ctx context.Context, tx *eventstore.Tx) ([]Category, error)
}

// Categories manages a Household's Categories.
type Categories struct {
	scope      HouseholdScope
	categories CategoryAggregate
	reader     CategoryReader
}

// NewCategories returns the Categories service.
func NewCategories(scope HouseholdScope, categories CategoryAggregate, reader CategoryReader) *Categories {
	return &Categories{scope: scope, categories: categories, reader: reader}
}

// SeedInternalCategories creates the Internal Categories of the new
// Household the transaction is set to. Households calls it in the
// transaction that creates the Household.
func (c *Categories) SeedInternalCategories(ctx context.Context, tx *eventstore.Tx) error {
	for _, role := range domain.InternalRoles() {
		id, err := sharedkernel.NewID()
		if err != nil {
			return err //nolint:wrapcheck // already wrapped by the shared kernel
		}

		if _, err := c.categories.Execute(ctx, tx, id, domain.SeedInternalCategory{Role: role}); err != nil {
			return fmt.Errorf("seed %s: %w", role, err)
		}
	}

	return nil
}

// List returns every Category of the Household, Internal Categories included.
func (c *Categories) List(ctx context.Context, household uuid.UUID) ([]Category, error) {
	var list []Category

	err := c.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		var err error
		list, err = c.reader.Categories(ctx, tx)

		return err //nolint:wrapcheck // the adapter's error
	})
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}

	return list, nil
}
