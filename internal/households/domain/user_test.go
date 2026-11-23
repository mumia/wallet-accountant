package domain_test

import (
	"testing"

	"github.com/mumia/wallet-accountant/internal/households/domain"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider/decidertest"
)

var invitationID = sharedkernel.MustParseID("019a8b2c-0000-7000-8000-0000000000aa")

func register() domain.RegisterUser {
	return domain.RegisterUser{
		Issuer:       "https://issuer.example",
		Subject:      "subject-1",
		InvitationID: invitationID,
		Role:         domain.RoleInstanceAdministrator,
		DisplayName:  "Ana",
	}
}

func TestRegisteringAUser(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.User).
		When(register()).
		Then(domain.UserRegistered{
			Issuer:       "https://issuer.example",
			Subject:      "subject-1",
			InvitationID: invitationID,
			Role:         domain.RoleInstanceAdministrator,
			DisplayName:  "Ana",
		})
}

func TestTheDisplayNameIsTrimmed(t *testing.T) {
	t.Parallel()

	c := register()
	c.DisplayName = "  Ana \t"

	want := domain.UserRegistered(register())

	decidertest.Given(t, domain.User).When(c).Then(want)
}

func TestABlankDisplayNameIsRefused(t *testing.T) {
	t.Parallel()

	c := register()
	c.DisplayName = " \t "

	decidertest.Given(t, domain.User).When(c).ThenError(domain.ErrDisplayNameRequired)
}

func TestAUserRegisteredBeforeDisplayNamesHasNone(t *testing.T) {
	t.Parallel()

	legacy := domain.UserRegistered(register())
	legacy.DisplayName = ""

	if name := decidertest.Given(t, domain.User, domain.UserEvent(legacy)).State().DisplayName; name != "" {
		t.Errorf("display name = %q, want none", name)
	}
}

func TestARegisteredUserCannotRegisterAgain(t *testing.T) {
	t.Parallel()

	past := []domain.UserEvent{domain.UserRegistered(register())}

	decidertest.Given(t, domain.User, past...).When(register()).ThenError(domain.ErrUserAlreadyRegistered)
}

func TestTheInstanceAdministratorRoleMakesTheUserInstanceAdministrator(t *testing.T) {
	t.Parallel()

	admin := domain.UserRegistered(register())
	plain := domain.UserRegistered(register())
	plain.Role = domain.RoleUser

	if !decidertest.Given(t, domain.User, domain.UserEvent(admin)).State().IsInstanceAdministrator {
		t.Error("a User registered with the Instance administrator role is not Instance administrator")
	}

	if decidertest.Given(t, domain.User, domain.UserEvent(plain)).State().IsInstanceAdministrator {
		t.Error("a User registered with the user role is Instance administrator")
	}
}
