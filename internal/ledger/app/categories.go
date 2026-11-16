package app

import (
	"context"
	"errors"
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
	CategoryRenamedType = "io.walletaccountant.ledger.category.renamed.v1"
	CategoryMovedType   = "io.walletaccountant.ledger.category.moved.v1"
)

// CategoryAggregate is the Category Decider wired to the runtime.
type CategoryAggregate = eventstore.Aggregate[domain.CategoryState, domain.CategoryCommand, domain.CategoryEvent]

// NewCategoryAggregate wires the Category Decider to its fold store.
func NewCategoryAggregate(folds eventstore.FoldStore[domain.CategoryState]) CategoryAggregate {
	codec := eventstore.NewCodec[domain.CategoryEvent](Source)
	eventstore.Register[domain.CategoryEvent, domain.CategoryCreated](codec, CategoryCreatedType)
	eventstore.Register[domain.CategoryEvent, domain.CategoryRenamed](codec, CategoryRenamedType)
	eventstore.Register[domain.CategoryEvent, domain.CategoryMoved](codec, CategoryMovedType)

	return CategoryAggregate{
		StreamType:        CategoryStreamType,
		Decider:           domain.Category,
		Codec:             codec,
		Folds:             folds,
		FoldSchemaVersion: 1,
	}
}

var (
	// ErrHouseholdNotFound is returned for a Household the caller may not
	// know exists (Households' request transaction decides it).
	ErrHouseholdNotFound = householdsapp.ErrHouseholdNotFound
	// ErrCategoryNotFound is returned for changing a Category the Household
	// doesn't have.
	ErrCategoryNotFound = domain.ErrCategoryNotFound
	// ErrParentNotFound is returned for placing a Category under a parent
	// the Household doesn't have.
	ErrParentNotFound = errors.New("parent category not found")
)

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
	// set to, siblings sorted by name.
	Categories(ctx context.Context, tx *eventstore.Tx) ([]Category, error)
	// LockTree holds the Household's Category tree until the transaction
	// ends, so a Category's siblings and ancestors read after it stay true
	// until the command is recorded. Concurrent creates of one name, or two
	// moves that together would make a cycle, wait for each other instead.
	LockTree(ctx context.Context, tx *eventstore.Tx) error
}

// NewCategory is a Category to create. Kind may be empty for a
// sub-Category; ParentID is uuid.Nil for a root Category.
type NewCategory struct {
	Name     string
	Kind     domain.Kind
	ParentID uuid.UUID
}

// CategoryChange renames a Category when Name is set, and moves it under
// ParentID (the root when it is uuid.Nil) when Move is set.
type CategoryChange struct {
	Name     *string
	Move     bool
	ParentID uuid.UUID
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

// Create creates a User's Category in the Household (consistency map:
// Create Category).
func (c *Categories) Create(ctx context.Context, household uuid.UUID, nc NewCategory) (Category, error) {
	id, err := sharedkernel.NewID()
	if err != nil {
		return Category{}, err //nolint:wrapcheck // already wrapped by the shared kernel
	}

	var created Category

	err = c.inTree(ctx, household, func(ctx context.Context, tx *eventstore.Tx, t tree) error {
		parent, err := t.parent(nc.ParentID)
		if err != nil {
			return err
		}

		res, err := c.categories.Execute(ctx, tx, id, domain.CreateCategory{
			Name: nc.Name, Kind: nc.Kind, Parent: parent, Siblings: t.siblingNames(nc.ParentID, uuid.Nil),
		})
		if err != nil {
			return err //nolint:wrapcheck // the domain's or runtime's error
		}

		created = categoryOf(res.State)

		return nil
	})
	if err != nil {
		return Category{}, fmt.Errorf("create category: %w", err)
	}

	return created, nil
}

// Change renames or moves a Category of the Household (consistency map:
// Rename or move Category).
func (c *Categories) Change(ctx context.Context, household, id uuid.UUID, change CategoryChange) (Category, error) {
	var changed Category

	err := c.inTree(ctx, household, func(ctx context.Context, tx *eventstore.Tx, t tree) error {
		current, ok := t[id]
		if !ok {
			return ErrCategoryNotFound
		}

		cmd := domain.ChangeCategory{Name: change.Name, Move: change.Move}

		destination := current.ParentID
		if change.Move {
			parent, err := t.parent(change.ParentID)
			if err != nil {
				return err
			}

			cmd.Parent, destination = parent, change.ParentID
		}

		cmd.Siblings = t.siblingNames(destination, id)

		res, err := c.categories.Execute(ctx, tx, id, cmd)
		if err != nil {
			return err //nolint:wrapcheck // the domain's or runtime's error
		}

		changed = categoryOf(res.State)

		return nil
	})
	if err != nil {
		return Category{}, fmt.Errorf("change category %s: %w", id, err)
	}

	return changed, nil
}

// List returns every Category of the Household, Internal Categories
// included, siblings sorted by name.
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

// inTree runs fn in the Household's request transaction with its Category
// tree locked and read.
func (c *Categories) inTree(ctx context.Context, household uuid.UUID,
	fn func(ctx context.Context, tx *eventstore.Tx, t tree) error,
) error {
	return c.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error { //nolint:wrapcheck // callers wrap
		if err := c.reader.LockTree(ctx, tx); err != nil {
			return err //nolint:wrapcheck // the adapter's error
		}

		list, err := c.reader.Categories(ctx, tx)
		if err != nil {
			return err //nolint:wrapcheck // the adapter's error
		}

		t := make(tree, len(list))
		for _, cat := range list {
			t[cat.ID] = cat
		}

		return fn(ctx, tx, t)
	})
}

// tree is a Household's Categories by ID.
type tree map[uuid.UUID]Category

// parent returns the Category id as a domain Parent with its path from the
// root, or nil for uuid.Nil (the root).
func (t tree) parent(id uuid.UUID) (*domain.Parent, error) {
	if id == uuid.Nil {
		return nil, nil //nolint:nilnil // the root has no parent
	}

	p, ok := t[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrParentNotFound, id)
	}

	var path []sharedkernel.ID
	// A tree has no cycles, so the walk ends at a root; the bound only
	// guards against a corrupted fold.
	for at := id; at != uuid.Nil && len(path) <= len(t); at = t[at].ParentID {
		path = append([]sharedkernel.ID{at}, path...)
	}

	return &domain.Parent{ID: id, Kind: p.Kind, Path: path}, nil
}

// siblingNames returns the names of the Categories under parent (the root
// Categories for uuid.Nil), except the Category except.
func (t tree) siblingNames(parent, except uuid.UUID) []string {
	var names []string

	for _, cat := range t {
		if cat.ParentID == parent && cat.ID != except {
			names = append(names, cat.Name)
		}
	}

	return names
}

func categoryOf(s domain.CategoryState) Category {
	return Category{ID: s.ID, Name: s.Name, Kind: s.Kind, ParentID: s.ParentID}
}
