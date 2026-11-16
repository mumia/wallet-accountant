// Package http is the Ledger HTTP adapter: the ogen operations for
// Categories, Accounts and Transactions.
package http

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/mumia/wallet-accountant/internal/ledger/app"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi/oas"
)

// Handler implements the Ledger operations of oas.Handler.
type Handler struct {
	categories *app.Categories
}

// NewHandler returns the Ledger HTTP adapter.
func NewHandler(categories *app.Categories) *Handler {
	return &Handler{categories: categories}
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

	items := make([]oas.Category, 0, len(list))

	for _, c := range list {
		item := oas.Category{CategoryID: c.ID, Name: c.Name, Kind: strings.ToUpper(string(c.Kind))}
		if c.ParentID != uuid.Nil {
			item.ParentID = oas.NewOptUUID(c.ParentID)
		}

		items = append(items, item)
	}

	return &oas.CategoryList{Items: items}, nil
}
