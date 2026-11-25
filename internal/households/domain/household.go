package domain

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider"
)

var (
	// ErrHouseholdAlreadyCreated is returned for creating a Household twice.
	ErrHouseholdAlreadyCreated = errors.New("household already created")
	// ErrHouseholdNameRequired is returned for a blank Household name.
	ErrHouseholdNameRequired = errors.New("household name required")
	// ErrHouseholdNotCreated is returned for adding a member to a Household
	// that was never created.
	ErrHouseholdNotCreated = errors.New("household not created")
	// ErrAlreadyAMember is returned for adding a User who already belongs to
	// the Household.
	ErrAlreadyAMember = errors.New("already a member")
	// ErrNotAMember is returned for removing a User who doesn't belong to
	// the Household.
	ErrNotAMember = errors.New("not a member")
	// ErrLastMember is returned for removing the Household's only member,
	// so a Household always has someone who can reach it.
	ErrLastMember = errors.New("last member")
)

// HouseholdState is a Household's fold. Membership is part of it.
type HouseholdState struct {
	Created      bool
	Name         string
	BaseCurrency string
	Members      []sharedkernel.ID
	// JoinedAt is when each member joined: the time of their memberadded
	// event.
	JoinedAt map[sharedkernel.ID]time.Time
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

// AddMember makes a User a member of the Household by redeeming an
// Invitation to it.
type AddMember struct {
	UserID       sharedkernel.ID
	InvitationID sharedkernel.ID
}

// RemoveMember removes a member from the Household. Whoever removes them is
// the event's actor; when that is the member themselves, they left.
type RemoveMember struct {
	UserID sharedkernel.ID
}

func (CreateHousehold) isHouseholdCommand() {}
func (AddMember) isHouseholdCommand()       {}
func (RemoveMember) isHouseholdCommand()    {}

type (
	// HouseholdEvent is a Household's event.
	HouseholdEvent interface{ isHouseholdEvent() }

	// HouseholdCreated is recorded when a Household is created.
	HouseholdCreated struct {
		Name         string `json:"name"`
		BaseCurrency string `json:"basecurrency"`
	}

	// HouseholdMemberAdded is recorded when a User becomes a member.
	// InvitationID is the Invitation they redeemed, absent for the
	// Household's creator.
	HouseholdMemberAdded struct {
		UserID       sharedkernel.ID `json:"userid"`
		InvitationID sharedkernel.ID `json:"invitationid,omitzero"`
	}

	// HouseholdMemberRemoved is recorded when a member leaves or is
	// removed. The event's actor is whoever removed them: the member
	// themselves when they left.
	HouseholdMemberRemoved struct {
		UserID sharedkernel.ID `json:"userid"`
	}
)

func (HouseholdCreated) isHouseholdEvent()       {}
func (HouseholdMemberAdded) isHouseholdEvent()   {}
func (HouseholdMemberRemoved) isHouseholdEvent() {}

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
	case AddMember:
		switch {
		case !s.Created:
			return nil, ErrHouseholdNotCreated
		case s.IsMember(c.UserID):
			return nil, ErrAlreadyAMember
		}

		return []HouseholdEvent{HouseholdMemberAdded(c)}, nil
	case RemoveMember:
		switch {
		case !s.IsMember(c.UserID):
			return nil, ErrNotAMember
		case len(s.Members) == 1:
			return nil, ErrLastMember
		}

		return []HouseholdEvent{HouseholdMemberRemoved(c)}, nil
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
			s.JoinedAt = maps.Clone(s.JoinedAt)

			if s.JoinedAt == nil {
				s.JoinedAt = map[sharedkernel.ID]time.Time{}
			}

			s.JoinedAt[ev.UserID] = e.Time
		}
	case HouseholdMemberRemoved:
		// JoinedAt drops them too, so a rejoin records its own time.
		s.Members = slices.DeleteFunc(slices.Clone(s.Members), func(m sharedkernel.ID) bool { return m == ev.UserID })
		s.JoinedAt = maps.Clone(s.JoinedAt)
		delete(s.JoinedAt, ev.UserID)
	}

	return s
}
