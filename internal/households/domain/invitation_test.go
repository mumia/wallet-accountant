package domain_test

import (
	"testing"
	"time"

	"github.com/mumia/wallet-accountant/internal/households/domain"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider/decidertest"
)

var (
	issuedAt  = time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC)
	expiresAt = issuedAt.Add(24 * time.Hour)
	firstHash = domain.HashCode("first-code")
)

func issued() domain.InvitationEvent {
	return domain.InvitationIssued{
		Role:      domain.RoleInstanceAdministrator,
		CodeHash:  firstHash,
		ExpiresAt: expiresAt,
		IssuedBy:  domain.IssuedByOperator,
	}
}

func TestIssuingANewInvitation(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Invitation).
		When(domain.IssueInvitation{
			Role:      domain.RoleInstanceAdministrator,
			CodeHash:  firstHash,
			ExpiresAt: expiresAt,
			IssuedBy:  domain.IssuedByOperator,
		}).
		Then(issued())
}

func TestRedeemingWithTheCurrentCodeBeforeItExpires(t *testing.T) {
	t.Parallel()

	user := sharedkernel.MustParseID("019a8b2c-0000-7000-8000-000000000001")

	decidertest.Given(t, domain.Invitation, issued()).
		When(domain.RedeemInvitation{CodeHash: firstHash, UserID: user, At: expiresAt.Add(-time.Second)}).
		Then(domain.InvitationRedeemed{UserID: user})
}

func TestRedemptionRefusals(t *testing.T) {
	t.Parallel()

	user := sharedkernel.MustParseID("019a8b2c-0000-7000-8000-000000000001")
	other := sharedkernel.MustParseID("019a8b2c-0000-7000-8000-000000000002")
	beforeExpiry := expiresAt.Add(-time.Minute)
	reissued := domain.InvitationReissued{CodeHash: domain.HashCode("second-code"), ExpiresAt: expiresAt}

	for _, tc := range []struct {
		name string
		past []domain.InvitationEvent
		cmd  domain.RedeemInvitation
		want error
	}{
		{
			name: "never issued",
			cmd:  domain.RedeemInvitation{CodeHash: firstHash, UserID: user, At: beforeExpiry},
			want: domain.ErrInvitationNotIssued,
		},
		{
			name: "wrong code",
			past: []domain.InvitationEvent{issued()},
			cmd:  domain.RedeemInvitation{CodeHash: domain.HashCode("guess"), UserID: user, At: beforeExpiry},
			want: domain.ErrWrongInvitationCode,
		},
		{
			name: "code superseded by a reissue",
			past: []domain.InvitationEvent{issued(), reissued},
			cmd:  domain.RedeemInvitation{CodeHash: firstHash, UserID: user, At: beforeExpiry},
			want: domain.ErrWrongInvitationCode,
		},
		{
			name: "at the expiry",
			past: []domain.InvitationEvent{issued()},
			cmd:  domain.RedeemInvitation{CodeHash: firstHash, UserID: user, At: expiresAt},
			want: domain.ErrInvitationExpired,
		},
		{
			name: "after the expiry",
			past: []domain.InvitationEvent{issued()},
			cmd:  domain.RedeemInvitation{CodeHash: firstHash, UserID: user, At: expiresAt.Add(time.Hour)},
			want: domain.ErrInvitationExpired,
		},
		{
			name: "already redeemed",
			past: []domain.InvitationEvent{issued(), domain.InvitationRedeemed{UserID: other}},
			cmd:  domain.RedeemInvitation{CodeHash: firstHash, UserID: user, At: beforeExpiry},
			want: domain.ErrInvitationAlreadyRedeemed,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			decidertest.Given(t, domain.Invitation, tc.past...).When(tc.cmd).ThenError(tc.want)
		})
	}
}

func TestIssuingARedeemedInvitationIsRefused(t *testing.T) {
	t.Parallel()

	past := []domain.InvitationEvent{issued(), domain.InvitationRedeemed{UserID: sharedkernel.MustParseID("019a8b2c-0000-7000-8000-000000000003")}}

	decidertest.Given(t, domain.Invitation, past...).
		When(domain.IssueInvitation{
			Role:      domain.RoleInstanceAdministrator,
			CodeHash:  domain.HashCode("second-code"),
			ExpiresAt: expiresAt,
			IssuedBy:  domain.IssuedByOperator,
		}).
		ThenError(domain.ErrInvitationAlreadyRedeemed)
}

func TestIssuingAnUnredeemedInvitationAgainReissuesItWithANewCodeAndExpiry(t *testing.T) {
	t.Parallel()

	later := expiresAt.Add(time.Hour)

	decidertest.Given(t, domain.Invitation, issued()).
		When(domain.IssueInvitation{
			Role:      domain.RoleInstanceAdministrator,
			CodeHash:  domain.HashCode("second-code"),
			ExpiresAt: later,
			IssuedBy:  domain.IssuedByOperator,
		}).
		Then(domain.InvitationReissued{
			CodeHash:  domain.HashCode("second-code"),
			ExpiresAt: later,
			IssuedBy:  domain.IssuedByOperator,
		})
}

func TestIssuingAnInvitationToAHousehold(t *testing.T) {
	t.Parallel()

	// The role is always user and the code expires CodeLifetime (24 hours)
	// after the command's timestamp. The Household and the issuing User come
	// from the envelope, so the event names neither.
	decidertest.Given(t, domain.Invitation).
		When(domain.IssueHouseholdInvitation{CodeHash: firstHash, At: issuedAt}).
		Then(domain.InvitationIssued{
			Role:      domain.RoleUser,
			CodeHash:  firstHash,
			ExpiresAt: time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC),
		})
}

func TestAnInvitationToAHouseholdIsNeverReissued(t *testing.T) {
	t.Parallel()

	var past domain.InvitationEvent = domain.InvitationIssued{Role: domain.RoleUser, CodeHash: firstHash, ExpiresAt: expiresAt}

	decidertest.Given(t, domain.Invitation, past).
		When(domain.IssueHouseholdInvitation{CodeHash: domain.HashCode("second-code"), At: issuedAt}).
		ThenError(domain.ErrInvitationAlreadyIssued)
}

func TestAnInvitationToAHouseholdExpiresAtExactly24Hours(t *testing.T) {
	t.Parallel()

	user := sharedkernel.MustParseID("019a8b2c-0000-7000-8000-000000000001")
	var past domain.InvitationEvent = domain.InvitationIssued{Role: domain.RoleUser, CodeHash: firstHash, ExpiresAt: expiresAt}

	decidertest.Given(t, domain.Invitation, past).
		When(domain.RedeemInvitation{CodeHash: firstHash, UserID: user, At: issuedAt.Add(24*time.Hour - time.Nanosecond)}).
		Then(domain.InvitationRedeemed{UserID: user})

	decidertest.Given(t, domain.Invitation, past).
		When(domain.RedeemInvitation{CodeHash: firstHash, UserID: user, At: issuedAt.Add(24 * time.Hour)}).
		ThenError(domain.ErrInvitationExpired)
}
