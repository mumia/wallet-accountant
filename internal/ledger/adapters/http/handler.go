// Package http is the Ledger HTTP adapter: the ogen operations for
// Categories, Accounts and Transactions.
package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/mumia/wallet-accountant/internal/ledger/app"
	"github.com/mumia/wallet-accountant/internal/ledger/domain"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi/oas"
)

// Problem types of the Category operations.
const (
	// InvalidCategoryType is for a Category the domain refuses to create or
	// change, other than for the reasons below.
	InvalidCategoryType = "/problems/invalid-category"
	// InternalCategoryType is for creating an Internal Category, or nesting
	// under, renaming or moving one.
	InternalCategoryType = "/problems/internal-category"
	// CategoryNameTakenType is for a name a sibling already has.
	CategoryNameTakenType = "/problems/category-name-taken"
)

// Handler implements the Ledger operations of oas.Handler.
type Handler struct {
	categories *app.Categories
}

// NewHandler returns the Ledger HTTP adapter.
func NewHandler(categories *app.Categories) *Handler {
	return &Handler{categories: categories}
}

// CreateCategory implements createCategory.
func (h *Handler) CreateCategory(ctx context.Context, req *oas.CategoryCreation, params oas.CreateCategoryParams,
) (oas.CreateCategoryRes, error) {
	created, err := h.categories.Create(ctx, params.HouseholdID, app.NewCategory{
		Name:     req.Name,
		Kind:     domain.Kind(strings.ToLower(req.Kind.Or(""))),
		ParentID: req.ParentID.Or(uuid.Nil),
	})
	if errors.Is(err, app.ErrHouseholdNotFound) {
		return new(oas.CreateCategoryNotFound(httpapi.NotFoundProblem())), nil
	}

	if p, ok := categoryProblem(err); ok {
		if p.Status == http.StatusConflict {
			return new(oas.CreateCategoryConflict(p)), nil
		}

		return new(oas.CreateCategoryUnprocessableEntity(p)), nil
	}

	if err != nil {
		return nil, fmt.Errorf("create category: %w", err)
	}

	return categoryResponse(created), nil
}

// ChangeCategory implements changeCategory.
func (h *Handler) ChangeCategory(ctx context.Context, req *oas.CategoryChange, params oas.ChangeCategoryParams,
) (oas.ChangeCategoryRes, error) {
	change := app.CategoryChange{Move: req.ParentID.Set, ParentID: req.ParentID.Or(uuid.Nil)}
	if name, ok := req.Name.Get(); ok {
		change.Name = &name
	}

	changed, err := h.categories.Change(ctx, params.HouseholdID, params.CategoryID, change)
	if errors.Is(err, app.ErrHouseholdNotFound) || errors.Is(err, app.ErrCategoryNotFound) {
		return new(oas.ChangeCategoryNotFound(httpapi.NotFoundProblem())), nil
	}

	if p, ok := categoryProblem(err); ok {
		if p.Status == http.StatusConflict {
			return new(oas.ChangeCategoryConflict(p)), nil
		}

		return new(oas.ChangeCategoryUnprocessableEntity(p)), nil
	}

	if err != nil {
		return nil, fmt.Errorf("change category: %w", err)
	}

	return categoryResponse(changed), nil
}

// categoryProblem maps a refused create or change to its problem.
func categoryProblem(err error) (oas.Problem, bool) {
	invalid := func(detail string) (oas.Problem, bool) {
		return httpapi.Problem(http.StatusUnprocessableEntity, InvalidCategoryType, "Invalid Category", detail), true
	}

	switch {
	case err == nil:
		return oas.Problem{}, false
	case errors.Is(err, domain.ErrCategoryNameTaken):
		return httpapi.Problem(http.StatusConflict, CategoryNameTakenType, "Category name taken",
			"Another Category under the same parent already has this name."), true
	case errors.Is(err, domain.ErrInternalCategory):
		return httpapi.Problem(http.StatusUnprocessableEntity, InternalCategoryType, "Internal Category",
			"Internal Categories belong to the system: they can't be created, renamed or moved, "+
				"and nothing can be placed under them."), true
	case errors.Is(err, domain.ErrCategoryNameRequired):
		return invalid("The name must not be blank.")
	case errors.Is(err, domain.ErrCategoryKindRequired):
		return invalid("A root Category needs the kind INCOME or EXPENSE.")
	case errors.Is(err, domain.ErrCategoryKindMismatch):
		return invalid("A Category has the same kind as its parent.")
	case errors.Is(err, domain.ErrCategoryCycle):
		return invalid("A Category can't be moved under itself or one of its sub-Categories.")
	case errors.Is(err, app.ErrParentNotFound):
		return invalid("The parent Category doesn't exist in this Household.")
	default:
		return oas.Problem{}, false
	}
}

// ListCategories implements listCategories.
func (h *Handler) ListCategories(ctx context.Context, params oas.ListCategoriesParams) (oas.ListCategoriesRes, error) {
	list, err := h.categories.List(ctx, params.HouseholdID)

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound):
		return new(oas.ListCategoriesNotFound(httpapi.NotFoundProblem())), nil
	case err != nil:
		return nil, fmt.Errorf("list categories: %w", err)
	}

	roots := nest(list, uuid.Nil)
	if roots == nil {
		roots = []oas.CategoryNode{}
	}

	return &oas.CategoryTree{Items: roots}, nil
}

// nest returns the Categories under parent with their sub-Categories, in
// the order list has them; nil when there are none, so a leaf has no
// children property.
func nest(list []app.Category, parent uuid.UUID) []oas.CategoryNode {
	var nodes []oas.CategoryNode

	for _, c := range list {
		if c.ParentID != parent {
			continue
		}

		r := categoryResponse(c)
		nodes = append(nodes, oas.CategoryNode{
			CategoryID: r.CategoryID, Name: r.Name, Kind: r.Kind, ParentID: r.ParentID,
			Children: nest(list, c.ID),
		})
	}

	return nodes
}

func categoryResponse(c app.Category) *oas.Category {
	r := &oas.Category{CategoryID: c.ID, Name: c.Name, Kind: strings.ToUpper(string(c.Kind))}
	if c.ParentID != uuid.Nil {
		r.ParentID = oas.NewOptUUID(c.ParentID)
	}

	return r
}
