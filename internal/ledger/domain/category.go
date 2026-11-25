package domain

import (
	"errors"
	"fmt"
	"slices"
	"strings"

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
	// RoleAccountTransfer is the other side of every Transfer leg: money
	// moving between two of the Household's Accounts is neither spending
	// nor income. It is reached only through a Transfer.
	RoleAccountTransfer InternalRole = "account_transfer"
)

var internalNames = map[InternalRole]string{
	RoleOpeningBalance:    "Opening balance",
	RoleHouseholdTransfer: "Household transfer",
	RoleUncategorised:     "Uncategorised",
	RoleAccountTransfer:   "Account transfer",
}

// InternalRoles lists every Internal Category role, in the order they are
// seeded.
func InternalRoles() []InternalRole {
	return []InternalRole{RoleOpeningBalance, RoleHouseholdTransfer, RoleUncategorised, RoleAccountTransfer}
}

var (
	// ErrCategoryAlreadyCreated is returned for creating a Category twice.
	ErrCategoryAlreadyCreated = errors.New("category already created")
	// ErrCategoryNotFound is returned for changing a Category that was
	// never created.
	ErrCategoryNotFound = errors.New("category not found")
	// ErrUnknownInternalRole is returned for seeding an Internal Category
	// that doesn't exist.
	ErrUnknownInternalRole = errors.New("unknown internal category role")
	// ErrCategoryNameRequired is returned for a blank Category name.
	ErrCategoryNameRequired = errors.New("category name required")
	// ErrCategoryNameTaken is returned when a sibling already has the name,
	// ignoring case and surrounding spaces.
	ErrCategoryNameTaken = errors.New("category name taken by a sibling")
	// ErrCategoryKindRequired is returned for a root Category without the
	// Income or Expense kind.
	ErrCategoryKindRequired = errors.New("category kind must be income or expense")
	// ErrCategoryKindMismatch is returned for placing a Category under a
	// parent of another kind: an Expense tree never holds an Income Category.
	ErrCategoryKindMismatch = errors.New("category kind differs from its parent's")
	// ErrInternalCategory is returned for creating an Internal Category, or
	// nesting under, renaming or moving one: they belong to the system.
	ErrInternalCategory = errors.New("internal categories belong to the system")
	// ErrCategoryCycle is returned for moving a Category under itself or
	// one of its own sub-Categories.
	ErrCategoryCycle = errors.New("category can't be moved under itself")
)

// CategoryState is a Category's fold. ID is the Category's own ID (its
// stream's).
type CategoryState struct {
	Created      bool
	ID           sharedkernel.ID
	Name         string
	Kind         Kind
	ParentID     sharedkernel.ID
	InternalRole InternalRole
}

// Parent is the Category another one is placed under, as the application
// read it: its kind and its Path, the IDs from its root Category down to it,
// itself included.
type Parent struct {
	ID   sharedkernel.ID
	Kind Kind
	Path []sharedkernel.ID
}

type (
	// CategoryCommand is a command to a Category.
	CategoryCommand interface{ isCategoryCommand() }

	// SeedInternalCategory creates one of a new Household's Internal
	// Categories.
	SeedInternalCategory struct {
		Role InternalRole
	}

	// CreateCategory creates a User's Category under Parent, or as a root
	// Category when Parent is nil. A root Category needs the Income or
	// Expense kind; a sub-Category takes its parent's, so Kind may be left
	// empty. Siblings are the names of the Categories already under the
	// same parent (or of the root Categories), read with the Household's
	// Category tree locked.
	CreateCategory struct {
		Name     string
		Kind     Kind
		Parent   *Parent
		Siblings []string
	}

	// ChangeCategory renames a Category when Name is set, and moves it
	// under Parent (to the root when Parent is nil) when Move is set.
	// Siblings are the names of the other Categories under the parent the
	// Category ends up under, read with the Household's Category tree locked.
	ChangeCategory struct {
		Name     *string
		Move     bool
		Parent   *Parent
		Siblings []string
	}
)

func (SeedInternalCategory) isCategoryCommand() {}
func (CreateCategory) isCategoryCommand()       {}
func (ChangeCategory) isCategoryCommand()       {}

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

	// CategoryRenamed is recorded when a Category gets a new name.
	CategoryRenamed struct {
		Name string `json:"name"`
	}

	// CategoryMoved is recorded when a Category is placed under another
	// parent; ParentID is the zero ID for the root. Its kind stays the same.
	CategoryMoved struct {
		ParentID sharedkernel.ID `json:"parentid,omitzero"`
	}
)

func (CategoryCreated) isCategoryEvent() {}
func (CategoryRenamed) isCategoryEvent() {}
func (CategoryMoved) isCategoryEvent()   {}

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
	case CreateCategory:
		return createCategory(s, c)
	case ChangeCategory:
		return changeCategory(s, c)
	default:
		return nil, nil
	}
}

func createCategory(s CategoryState, c CreateCategory) ([]CategoryEvent, error) {
	if s.Created {
		return nil, ErrCategoryAlreadyCreated
	}

	kind, err := createdKind(c.Kind, c.Parent)
	if err != nil {
		return nil, err
	}

	name, err := siblingName(c.Name, c.Siblings)
	if err != nil {
		return nil, err
	}

	created := CategoryCreated{Name: name, Kind: kind}
	if c.Parent != nil {
		created.ParentID = c.Parent.ID
	}

	return []CategoryEvent{created}, nil
}

// createdKind returns the kind a new Category gets: its parent's, or the
// requested one for a root Category.
func createdKind(requested Kind, parent *Parent) (Kind, error) {
	if requested == KindInternal {
		return "", fmt.Errorf("%w: a User can't create one", ErrInternalCategory)
	}

	if parent != nil {
		if err := canHold(parent, parent.Kind); err != nil {
			return "", err
		}

		if requested != "" && requested != parent.Kind {
			return "", fmt.Errorf("%w: the parent is %s, not %s", ErrCategoryKindMismatch, parent.Kind, requested)
		}

		return parent.Kind, nil
	}

	if requested != KindIncome && requested != KindExpense {
		return "", ErrCategoryKindRequired
	}

	return requested, nil
}

func changeCategory(s CategoryState, c ChangeCategory) ([]CategoryEvent, error) {
	if !s.Created {
		return nil, ErrCategoryNotFound
	}

	if s.Kind == KindInternal {
		return nil, fmt.Errorf("%w: it can't be renamed or moved", ErrInternalCategory)
	}

	var events []CategoryEvent

	name := s.Name
	if c.Name != nil {
		name = strings.TrimSpace(*c.Name)
		if name != s.Name {
			events = append(events, CategoryRenamed{Name: name})
		}
	}

	if c.Move {
		parent := sharedkernel.NilID

		if c.Parent != nil {
			if slices.Contains(c.Parent.Path, s.ID) {
				return nil, ErrCategoryCycle
			}

			if err := canHold(c.Parent, s.Kind); err != nil {
				return nil, err
			}

			parent = c.Parent.ID
		}

		if parent != s.ParentID {
			events = append(events, CategoryMoved{ParentID: parent})
		}
	}

	if len(events) == 0 {
		return nil, nil
	}

	if _, err := siblingName(name, c.Siblings); err != nil {
		return nil, err
	}

	return events, nil
}

// canHold refuses placing a Category of the kind under parent.
func canHold(parent *Parent, kind Kind) error {
	if parent.Kind == KindInternal {
		return fmt.Errorf("%w: nothing can be nested under one", ErrInternalCategory)
	}

	if parent.Kind != kind {
		return fmt.Errorf("%w: the parent is %s, not %s", ErrCategoryKindMismatch, parent.Kind, kind)
	}

	return nil
}

// siblingName returns the trimmed name if it is neither blank nor taken by
// one of the siblings, ignoring case.
func siblingName(name string, siblings []string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrCategoryNameRequired
	}

	for _, sibling := range siblings {
		if strings.EqualFold(strings.TrimSpace(sibling), name) {
			return "", fmt.Errorf("%w: %q", ErrCategoryNameTaken, name)
		}
	}

	return name, nil
}

func evolveCategory(s CategoryState, e decider.Envelope[CategoryEvent]) CategoryState {
	s.ID = e.StreamID

	switch ev := e.Event.(type) {
	case CategoryCreated:
		s.Created = true
		s.Name, s.Kind, s.ParentID, s.InternalRole = ev.Name, ev.Kind, ev.ParentID, ev.InternalRole
	case CategoryRenamed:
		s.Name = ev.Name
	case CategoryMoved:
		s.ParentID = ev.ParentID
	}

	return s
}
