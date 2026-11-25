package domain_test

import (
	"testing"
	"time"

	"github.com/mumia/wallet-accountant/internal/households/domain"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider"
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

func TestAMemberJoinedWhenTheirMemberAddedEventOccurred(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, time.March, 10, 9, 0, 0, 0, time.UTC)
	s := domain.Household.Evolve(domain.HouseholdState{},
		decider.Envelope[domain.HouseholdEvent]{Time: created, Event: domain.HouseholdCreated{Name: "Home", BaseCurrency: "EUR"}})
	s = domain.Household.Evolve(s,
		decider.Envelope[domain.HouseholdEvent]{Time: created, Event: domain.HouseholdMemberAdded{UserID: creator}})

	if got := s.JoinedAt[creator]; !got.Equal(created) {
		t.Errorf("creator joined at %v, want %v", got, created)
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

func withJoiner() []domain.HouseholdEvent {
	return append(created(), domain.HouseholdMemberAdded{UserID: joiner, InvitationID: invitation})
}

func TestRemovingAnotherMember(t *testing.T) {
	t.Parallel()

	// The creator removes the joiner; who removed them is the event's actor.
	decidertest.Given(t, domain.Household, withJoiner()...).
		When(domain.RemoveMember{UserID: joiner}).
		Then(domain.HouseholdMemberRemoved{UserID: joiner})

	s := decidertest.Given(t, domain.Household,
		append(withJoiner(), domain.HouseholdMemberRemoved{UserID: joiner})...).State()

	if s.IsMember(joiner) || !s.IsMember(creator) {
		t.Errorf("members = %v, want only %v", s.Members, creator)
	}

	if _, ok := s.JoinedAt[joiner]; ok {
		t.Errorf("joined at still holds the removed member: %v", s.JoinedAt)
	}
}

func TestLeaving(t *testing.T) {
	t.Parallel()

	// The creator leaves, so the joiner is left as the only member.
	decidertest.Given(t, domain.Household, withJoiner()...).
		When(domain.RemoveMember{UserID: creator}).
		Then(domain.HouseholdMemberRemoved{UserID: creator})

	s := decidertest.Given(t, domain.Household,
		append(withJoiner(), domain.HouseholdMemberRemoved{UserID: creator})...).State()

	if s.IsMember(creator) || !s.IsMember(joiner) {
		t.Errorf("members = %v, want only %v", s.Members, joiner)
	}
}

func TestRemoveMemberRefusals(t *testing.T) {
	t.Parallel()

	outsider := sharedkernel.MustParseID("019a8b2c-0000-7000-8000-0000000000c3")

	for _, tc := range []struct {
		name string
		past []domain.HouseholdEvent
		cmd  domain.RemoveMember
		want error
	}{
		{
			name: "the last member",
			past: created(),
			cmd:  domain.RemoveMember{UserID: creator},
			want: domain.ErrLastMember,
		},
		{
			name: "the last member after the other left",
			past: append(withJoiner(), domain.HouseholdMemberRemoved{UserID: joiner}),
			cmd:  domain.RemoveMember{UserID: creator},
			want: domain.ErrLastMember,
		},
		{
			name: "a User who isn't a member",
			past: withJoiner(),
			cmd:  domain.RemoveMember{UserID: outsider},
			want: domain.ErrNotAMember,
		},
		{
			name: "a member already removed",
			past: append(withJoiner(), domain.HouseholdMemberRemoved{UserID: joiner}),
			cmd:  domain.RemoveMember{UserID: joiner},
			want: domain.ErrNotAMember,
		},
		{
			name: "a Household never created",
			cmd:  domain.RemoveMember{UserID: creator},
			want: domain.ErrNotAMember,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			decidertest.Given(t, domain.Household, tc.past...).When(tc.cmd).ThenError(tc.want)
		})
	}
}

func TestRejoiningAfterRemoval(t *testing.T) {
	t.Parallel()

	second := sharedkernel.MustParseID("019a8b2c-0000-7000-8000-0000000000e2")
	removed := append(withJoiner(), domain.HouseholdMemberRemoved{UserID: joiner})

	decidertest.Given(t, domain.Household, removed...).
		When(domain.AddMember{UserID: joiner, InvitationID: second}).
		Then(domain.HouseholdMemberAdded{UserID: joiner, InvitationID: second})
}

func TestARejoinRecordsTheNewJoiningTime(t *testing.T) {
	t.Parallel()

	first := time.Date(2026, time.March, 10, 9, 0, 0, 0, time.UTC)
	again := first.Add(48 * time.Hour)

	var s domain.HouseholdState
	for _, e := range []decider.Envelope[domain.HouseholdEvent]{
		{Time: first, Event: domain.HouseholdCreated{Name: "Home", BaseCurrency: "EUR"}},
		{Time: first, Event: domain.HouseholdMemberAdded{UserID: creator}},
		{Time: first, Event: domain.HouseholdMemberAdded{UserID: joiner, InvitationID: invitation}},
		{Time: first.Add(time.Hour), Event: domain.HouseholdMemberRemoved{UserID: joiner}},
		{Time: again, Event: domain.HouseholdMemberAdded{UserID: joiner, InvitationID: invitation}},
	} {
		s = domain.Household.Evolve(s, e)
	}

	if !s.IsMember(joiner) {
		t.Fatalf("members = %v, want %v among them", s.Members, joiner)
	}

	if got := s.JoinedAt[joiner]; !got.Equal(again) {
		t.Errorf("joiner joined at %v, want %v", got, again)
	}
}
