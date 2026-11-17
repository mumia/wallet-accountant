package domain

import (
	"errors"
	"slices"
	"strings"

	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider"
)

var (
	// ErrHouseholdAlreadyCreated is returned for creating a Household twice.
	ErrHouseholdAlreadyCreated = errors.New("household already created")
	// ErrHouseholdNameRequired is returned for a blank Household name.
	ErrHouseholdNameRequired = errors.New("household name required")
)

// HouseholdState is a Household's fold. Membership is part of it.
type HouseholdState struct {
	Created      bool
	Name         string
	BaseCurrency string
	Members      []sharedkernel.ID
}

// IsMember reports whether the User belongs to the Household.
func (s HouseholdState) IsMember(user sharedkernel.ID) bool {
	return slices.Contains(s.Members, user)
}

type (
	// HouseholdCommand is a command to a Household.
	HouseholdCommand interface{ isHouseholdCommand() }

	// CreateHousehold creates a Household with a name and a Base currency;
	// the User creating it becomes its first member.
	CreateHousehold struct {
		Name         string
		BaseCurrency string
		CreatedBy    sharedkernel.ID
	}
)

func (CreateHousehold) isHouseholdCommand() {}

type (
	// HouseholdEvent is a Household's event.
	HouseholdEvent interface{ isHouseholdEvent() }

	// HouseholdCreated is recorded when a Household is created.
	HouseholdCreated struct {
		Name         string `json:"name"`
		BaseCurrency string `json:"basecurrency"`
	}

	// HouseholdMemberAdded is recorded when a User becomes a member.
	HouseholdMemberAdded struct {
		UserID sharedkernel.ID `json:"userid"`
	}
)

func (HouseholdCreated) isHouseholdEvent()     {}
func (HouseholdMemberAdded) isHouseholdEvent() {}

// Household is the Household Decider (ADR-0003).
var Household = decider.Decider[HouseholdState, HouseholdCommand, HouseholdEvent]{
	Decide: decideHousehold,
	Evolve: evolveHousehold,
}

func decideHousehold(s HouseholdState, c HouseholdCommand) ([]HouseholdEvent, error) {
	switch c := c.(type) {
	case CreateHousehold:
		if s.Created {
			return nil, ErrHouseholdAlreadyCreated
		}

		name := strings.TrimSpace(c.Name)
		if name == "" {
			return nil, ErrHouseholdNameRequired
		}

		currency, err := sharedkernel.ParseCurrency(c.BaseCurrency)
		if err != nil {
			return nil, err //nolint:wrapcheck // the shared kernel's own refusal
		}

		return []HouseholdEvent{
			HouseholdCreated{Name: name, BaseCurrency: currency.Code()},
			HouseholdMemberAdded{UserID: c.CreatedBy},
		}, nil
	default:
		return nil, nil
	}
}

func evolveHousehold(s HouseholdState, e decider.Envelope[HouseholdEvent]) HouseholdState {
	switch ev := e.Event.(type) {
	case HouseholdCreated:
		s.Created = true
		s.Name, s.BaseCurrency = ev.Name, ev.BaseCurrency
	case HouseholdMemberAdded:
		if !s.IsMember(ev.UserID) {
			s.Members = append(slices.Clone(s.Members), ev.UserID)
		}
	}

	return s
}
