package domain

import (
	"errors"
	"fmt"

	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider"
)

// Kind is what a Category is for (ADR-0004).
type Kind string

// The Category kinds.
const (
	KindIncome   Kind = "income"
	KindExpense  Kind = "expense"
	KindInternal Kind = "internal"
)

// InternalRole names an Internal Category: the bookings it is for. Every
// Household has exactly one Internal Category per role, seeded when the
// Household is created, so later commands can find them by role.
type InternalRole string

// The Internal Categories.
const (
	RoleOpeningBalance    InternalRole = "opening_balance"
	RoleHouseholdTransfer InternalRole = "household_transfer"
	RoleUncategorised     InternalRole = "uncategorised"
)

var internalNames = map[InternalRole]string{
	RoleOpeningBalance:    "Opening balance",
	RoleHouseholdTransfer: "Household transfer",
	RoleUncategorised:     "Uncategorised",
}

// InternalRoles lists every Internal Category role, in the order they are
// seeded.
func InternalRoles() []InternalRole {
	return []InternalRole{RoleOpeningBalance, RoleHouseholdTransfer, RoleUncategorised}
}

var (
	// ErrCategoryAlreadyCreated is returned for creating a Category twice.
	ErrCategoryAlreadyCreated = errors.New("category already created")
	// ErrUnknownInternalRole is returned for seeding an Internal Category
	// that doesn't exist.
	ErrUnknownInternalRole = errors.New("unknown internal category role")
)

// CategoryState is a Category's fold.
type CategoryState struct {
	Created      bool
	Name         string
	Kind         Kind
	ParentID     sharedkernel.ID
	InternalRole InternalRole
}

type (
	// CategoryCommand is a command to a Category.
	CategoryCommand interface{ isCategoryCommand() }

	// SeedInternalCategory creates one of a new Household's Internal
	// Categories.
	SeedInternalCategory struct {
		Role InternalRole
	}
)

func (SeedInternalCategory) isCategoryCommand() {}

type (
	// CategoryEvent is a Category's event.
	CategoryEvent interface{ isCategoryEvent() }

	// CategoryCreated is recorded when a Category is created. ParentID is
	// the zero ID for a root Category; InternalRole is set only for an
	// Internal Category.
	CategoryCreated struct {
		Name         string          `json:"name"`
		Kind         Kind            `json:"kind"`
		ParentID     sharedkernel.ID `json:"parentid,omitzero"`
		InternalRole InternalRole    `json:"internalrole,omitempty"`
	}
)

func (CategoryCreated) isCategoryEvent() {}

// Category is the Category Decider.
var Category = decider.Decider[CategoryState, CategoryCommand, CategoryEvent]{
	Decide: decideCategory,
	Evolve: evolveCategory,
}

func decideCategory(s CategoryState, c CategoryCommand) ([]CategoryEvent, error) {
	switch c := c.(type) {
	case SeedInternalCategory:
		if s.Created {
			return nil, ErrCategoryAlreadyCreated
		}

		name, ok := internalNames[c.Role]
		if !ok {
			return nil, fmt.Errorf("%w: %q", ErrUnknownInternalRole, c.Role)
		}

		return []CategoryEvent{CategoryCreated{Name: name, Kind: KindInternal, InternalRole: c.Role}}, nil
	default:
		return nil, nil
	}
}

func evolveCategory(s CategoryState, e decider.Envelope[CategoryEvent]) CategoryState {
	if ev, ok := e.Event.(CategoryCreated); ok {
		s.Created = true
		s.Name, s.Kind, s.ParentID, s.InternalRole = ev.Name, ev.Kind, ev.ParentID, ev.InternalRole
	}

	return s
}
