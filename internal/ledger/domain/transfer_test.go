package domain_test

import (
	"slices"
	"testing"

	"github.com/mumia/wallet-accountant/internal/ledger/domain"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider/decidertest"
)

var (
	walletAccount   = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-0000000000e1")
	transferCat     = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-0000000000e2")
	bankLeg         = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-0000000000e3")
	cashLeg         = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-0000000000e4")
	aMatch          = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-0000000000e5")
	anotherMatch    = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-0000000000e6")
	legReversal     = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-0000000000e7")
	foodCategoryLeg = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-0000000000e8")
)

// withdrawal is the cash leg of an ATM withdrawal of 50.00 from the main
// Account into the wallet.
func withdrawal() domain.RecordTransaction {
	return domain.RecordTransaction{
		BookingDate:          startDate,
		Description:          "ATM",
		CounterpartAccountID: mainAccount,
		Postings: []domain.Posting{
			{AccountID: walletAccount, Amount: dec("50.00"), Currency: "EUR"},
			{CategoryID: transferCat, Amount: dec("-50.00"), Currency: "EUR"},
		},
	}
}

func recordedWithdrawal() domain.TransactionRecorded {
	return domain.TransactionRecorded{
		BookingDate:          "2026-01-01",
		TransactionDate:      "2026-01-01",
		Description:          "ATM",
		CounterpartAccountID: mainAccount,
		Postings:             withdrawal().Postings,
	}
}

func legRecorded() domain.TransactionEvent { return recordedWithdrawal() }

// confirmed is matchConfirmed as a past event.
var confirmed = domain.MatchEvent(matchConfirmed)

func TestRecordingATransferLeg(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Transaction).When(withdrawal()).Then(recordedWithdrawal())

	automatic := withdrawal()
	automatic.CreatedByTransfer = true
	want := recordedWithdrawal()
	want.CreatedByTransfer = true
	decidertest.Given(t, domain.Transaction).When(automatic).Then(want)

	s := decidertest.Given(t, domain.Transaction, domain.TransactionEvent(want)).State()
	if !s.IsTransferLeg() || !s.InTransit() || !s.CreatedByTransfer || s.CounterpartAccountID != mainAccount {
		t.Errorf("state = %+v, want an automatic leg In transit naming the main Account", s)
	}

	leg := s.Leg(cashLeg)
	if leg.AccountID != walletAccount || leg.CounterpartAccountID != mainAccount || !leg.Amount.Equal(dec("50")) {
		t.Errorf("leg = %+v, want +50.00 on the wallet naming the main Account", leg)
	}
}

func TestRecordingATransferLegRefusals(t *testing.T) {
	t.Parallel()

	itself := withdrawal()
	itself.CounterpartAccountID = walletAccount
	decidertest.Given(t, domain.Transaction).When(itself).ThenError(domain.ErrCounterpartIsItself)

	twoAccounts := withdrawal()
	twoAccounts.Postings = []domain.Posting{
		{AccountID: walletAccount, Amount: dec("50.00"), Currency: "EUR"},
		{AccountID: savingsAccount, Amount: dec("-50.00"), Currency: "EUR"},
	}
	decidertest.Given(t, domain.Transaction).When(twoAccounts).ThenError(domain.ErrInvalidPosting)

	noCounterpart := withdrawal()
	noCounterpart.CounterpartAccountID, noCounterpart.CreatedByTransfer = sharedkernel.NilID, true
	decidertest.Given(t, domain.Transaction).When(noCounterpart).ThenError(domain.ErrNotATransferLeg)
}

func TestSettingTheCategoryOfATransferLegIsRefused(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Transaction, legRecorded()).
		When(domain.SetTransactionCategory{CategoryID: foodCategoryLeg}).
		ThenError(domain.ErrTransferLeg)
}

func TestMatchingAndUnmatchingATransferLeg(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Transaction, legRecorded()).
		When(domain.MatchTransaction{MatchID: aMatch}).
		Then(domain.TransactionMatched{MatchID: aMatch})

	matched := []domain.TransactionEvent{recordedWithdrawal(), domain.TransactionMatched{MatchID: aMatch}}

	if s := decidertest.Given(t, domain.Transaction, matched...).State(); s.MatchID != aMatch || s.InTransit() {
		t.Errorf("state = %+v, want Matched, not In transit", s)
	}

	decidertest.Given(t, domain.Transaction, matched...).
		When(domain.UnmatchTransaction{MatchID: aMatch}).
		Then(domain.TransactionUnmatched{MatchID: aMatch})

	unmatched := append(slices.Clone(matched), domain.TransactionEvent(domain.TransactionUnmatched{MatchID: aMatch}))
	if s := decidertest.Given(t, domain.Transaction, unmatched...).State(); s.MatchID != sharedkernel.NilID || !s.InTransit() {
		t.Errorf("state = %+v, want In transit again", s)
	}

	// Once unmatched, it can be Matched again.
	decidertest.Given(t, domain.Transaction, unmatched...).
		When(domain.MatchTransaction{MatchID: anotherMatch}).
		Then(domain.TransactionMatched{MatchID: anotherMatch})
}

func TestMatchingRefusals(t *testing.T) {
	t.Parallel()

	matched := []domain.TransactionEvent{recordedWithdrawal(), domain.TransactionMatched{MatchID: aMatch}}
	decidertest.Given(t, domain.Transaction, matched...).
		When(domain.MatchTransaction{MatchID: anotherMatch}).ThenError(domain.ErrTransactionAlreadyMatched)

	decidertest.Given(t, domain.Transaction, legRecorded(), domain.TransactionEvent(domain.TransactionReversed{ReversalID: legReversal})).
		When(domain.MatchTransaction{MatchID: aMatch}).ThenError(domain.ErrTransactionReversed)

	decidertest.Given(t, domain.Transaction, domain.TransactionEvent(domain.TransactionRecorded{
		BookingDate: "2026-01-01", TransactionDate: "2026-01-01", Description: "Coffee",
		Postings: []domain.Posting{
			{AccountID: walletAccount, Amount: dec("-2.50"), Currency: "EUR"},
			{CategoryID: foodCategoryLeg, Amount: dec("2.50"), Currency: "EUR"},
		},
	})).When(domain.MatchTransaction{MatchID: aMatch}).ThenError(domain.ErrNotATransferLeg)

	decidertest.Given(t, domain.Transaction).
		When(domain.MatchTransaction{MatchID: aMatch}).ThenError(domain.ErrTransactionNotRecorded)

	decidertest.Given(t, domain.Transaction, legRecorded()).
		When(domain.MatchTransaction{}).ThenError(domain.ErrMatchIDRequired)

	decidertest.Given(t, domain.Transaction, matched...).
		When(domain.UnmatchTransaction{MatchID: anotherMatch}).ThenError(domain.ErrTransactionNotMatched)
}

// legs is a withdrawal of 50.00 from the main Account: -50.00 there naming
// the wallet, +50.00 in the wallet naming the main Account.
func legs() [2]domain.TransferLeg {
	return [2]domain.TransferLeg{
		{TransactionID: bankLeg, AccountID: mainAccount, CounterpartAccountID: walletAccount, Amount: dec("-50.00"), Currency: "EUR"},
		{TransactionID: cashLeg, AccountID: walletAccount, CounterpartAccountID: mainAccount, Amount: dec("50.00"), Currency: "EUR"},
	}
}

var matchConfirmed = domain.MatchConfirmed{Kind: domain.MatchKindAccountTransfer, Legs: [2]domain.MatchLeg{
	{TransactionID: bankLeg, AccountID: mainAccount},
	{TransactionID: cashLeg, AccountID: walletAccount},
}}

func TestConfirmingAndRemovingAMatch(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Match).
		When(domain.ConfirmMatch{Kind: domain.MatchKindAccountTransfer, Legs: legs()}).
		Then(matchConfirmed)

	s := decidertest.Given(t, domain.Match, confirmed).State()
	if !s.Confirmed || s.Removed || s.Legs != matchConfirmed.Legs {
		t.Errorf("state = %+v, want confirmed with both legs", s)
	}

	decidertest.Given(t, domain.Match, confirmed).When(domain.RemoveMatch{}).Then(domain.MatchRemoved{})

	if s := decidertest.Given(t, domain.Match, confirmed, domain.MatchEvent(domain.MatchRemoved{})).State(); !s.Removed {
		t.Errorf("state = %+v, want removed", s)
	}
}

func TestConfirmingAMatchRefusals(t *testing.T) {
	t.Parallel()

	for name, change := range map[string]func(l *[2]domain.TransferLeg){
		"the same sign":            func(l *[2]domain.TransferLeg) { l[1].Amount = dec("-50.00") },
		"different amounts":        func(l *[2]domain.TransferLeg) { l[1].Amount = dec("49.99") },
		"zero":                     func(l *[2]domain.TransferLeg) { l[0].Amount, l[1].Amount = dec("0"), dec("0") },
		"different currencies":     func(l *[2]domain.TransferLeg) { l[1].Currency = "USD" },
		"a mismatched counterpart": func(l *[2]domain.TransferLeg) { l[0].CounterpartAccountID = savingsAccount },
		"one-sided counterparts":   func(l *[2]domain.TransferLeg) { l[1].CounterpartAccountID = savingsAccount },
		"the same Transaction":     func(l *[2]domain.TransferLeg) { l[1].TransactionID = bankLeg },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			l := legs()
			change(&l)
			decidertest.Given(t, domain.Match).
				When(domain.ConfirmMatch{Kind: domain.MatchKindAccountTransfer, Legs: l}).
				ThenError(domain.ErrLegsDontMatch)
		})
	}

	decidertest.Given(t, domain.Match).
		When(domain.ConfirmMatch{Kind: "planned", Legs: legs()}).ThenError(domain.ErrUnknownMatchKind)
	decidertest.Given(t, domain.Match, confirmed).
		When(domain.ConfirmMatch{Kind: domain.MatchKindAccountTransfer, Legs: legs()}).
		ThenError(domain.ErrMatchAlreadyConfirmed)
	decidertest.Given(t, domain.Match).When(domain.RemoveMatch{}).ThenError(domain.ErrMatchNotConfirmed)
	decidertest.Given(t, domain.Match, confirmed, domain.MatchEvent(domain.MatchRemoved{})).
		When(domain.RemoveMatch{}).ThenError(domain.ErrMatchRemoved)
}

// A Matched leg is reversed only once its Match is removed; unmatched, it
// reverses like any Transaction.
func TestReversingAMatchedLegNeedsItUnmatchedFirst(t *testing.T) {
	t.Parallel()

	matched := []domain.TransactionEvent{legRecorded(), domain.TransactionMatched{MatchID: aMatch}}

	decidertest.Given(t, domain.Transaction, matched...).
		When(domain.ReverseTransaction{ReversalID: legReversal}).ThenError(domain.ErrTransactionMatched)

	decidertest.Given(t, domain.Transaction, append(slices.Clone(matched),
		domain.TransactionEvent(domain.TransactionUnmatched{MatchID: aMatch}))...).
		When(domain.ReverseTransaction{ReversalID: legReversal}).
		Then(domain.TransactionReversed{ReversalID: legReversal})
}
