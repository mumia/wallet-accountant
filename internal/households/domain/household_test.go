package domain_test

import (
	"testing"

	"github.com/mumia/wallet-accountant/internal/households/domain"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider/decidertest"
)

var creator = sharedkernel.MustParseID("019a8b2c-0000-7000-8000-0000000000c1")

func TestCreatingAHouseholdMakesItsCreatorAMember(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Household).
		When(domain.CreateHousehold{Name: "Home", BaseCurrency: "EUR", CreatedBy: creator}).
		Then(
			domain.HouseholdCreated{Name: "Home", BaseCurrency: "EUR"},
			domain.HouseholdMemberAdded{UserID: creator},
		)
}

func TestTheNameIsTrimmed(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Household).
		When(domain.CreateHousehold{Name: "  Home \n", BaseCurrency: "EUR", CreatedBy: creator}).
		Then(
			domain.HouseholdCreated{Name: "Home", BaseCurrency: "EUR"},
			domain.HouseholdMemberAdded{UserID: creator},
		)
}

func TestTheFoldHoldsTheHouseholdAndItsMembers(t *testing.T) {
	t.Parallel()

	s := decidertest.Given(t, domain.Household, []domain.HouseholdEvent{
		domain.HouseholdCreated{Name: "Home", BaseCurrency: "EUR"},
		domain.HouseholdMemberAdded{UserID: creator},
	}...).State()

	if !s.Created || s.Name != "Home" || s.BaseCurrency != "EUR" {
		t.Errorf("state = %+v, want the created Household Home in EUR", s)
	}

	if !s.IsMember(creator) {
		t.Errorf("members = %v, want %v among them", s.Members, creator)
	}

	if s.IsMember(sharedkernel.MustParseID("019a8b2c-0000-7000-8000-0000000000c2")) {
		t.Error("someone who never joined is a member")
	}
}

func TestCreateHouseholdRefusals(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		past []domain.HouseholdEvent
		cmd  domain.CreateHousehold
		want error
	}{
		{
			name: "already created",
			past: []domain.HouseholdEvent{domain.HouseholdCreated{Name: "Home", BaseCurrency: "EUR"}},
			cmd:  domain.CreateHousehold{Name: "Home", BaseCurrency: "EUR", CreatedBy: creator},
			want: domain.ErrHouseholdAlreadyCreated,
		},
		{
			name: "blank name",
			cmd:  domain.CreateHousehold{Name: " \t", BaseCurrency: "EUR", CreatedBy: creator},
			want: domain.ErrHouseholdNameRequired,
		},
		{
			name: "unknown currency",
			cmd:  domain.CreateHousehold{Name: "Home", BaseCurrency: "XYZ", CreatedBy: creator},
			want: sharedkernel.ErrUnknownCurrency,
		},
		{
			name: "lower-case currency",
			cmd:  domain.CreateHousehold{Name: "Home", BaseCurrency: "eur", CreatedBy: creator},
			want: sharedkernel.ErrUnknownCurrency,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			decidertest.Given(t, domain.Household, tc.past...).When(tc.cmd).ThenError(tc.want)
		})
	}
}

var (
	joiner     = sharedkernel.MustParseID("019a8b2c-0000-7000-8000-0000000000c2")
	invitation = sharedkernel.MustParseID("019a8b2c-0000-7000-8000-0000000000e1")
)

func created() []domain.HouseholdEvent {
	return []domain.HouseholdEvent{
		domain.HouseholdCreated{Name: "Home", BaseCurrency: "EUR"},
		domain.HouseholdMemberAdded{UserID: creator},
	}
}

func TestAddingAMemberByInvitation(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Household, created()...).
		When(domain.AddMember{UserID: joiner, InvitationID: invitation}).
		Then(domain.HouseholdMemberAdded{UserID: joiner, InvitationID: invitation})

	s := decidertest.Given(t, domain.Household,
		append(created(), domain.HouseholdMemberAdded{UserID: joiner, InvitationID: invitation})...).State()

	if !s.IsMember(joiner) || !s.IsMember(creator) {
		t.Errorf("members = %v, want %v and %v", s.Members, creator, joiner)
	}
}

func TestAddMemberRefusals(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		past []domain.HouseholdEvent
		cmd  domain.AddMember
		want error
	}{
		{
			name: "an existing member",
			past: created(),
			cmd:  domain.AddMember{UserID: creator, InvitationID: invitation},
			want: domain.ErrAlreadyAMember,
		},
		{
			name: "a Household never created",
			cmd:  domain.AddMember{UserID: joiner, InvitationID: invitation},
			want: domain.ErrHouseholdNotCreated,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			decidertest.Given(t, domain.Household, tc.past...).When(tc.cmd).ThenError(tc.want)
		})
	}
}
