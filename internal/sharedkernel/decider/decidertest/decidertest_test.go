package decidertest_test

import (
	"errors"
	"testing"

	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider/decidertest"
)

// A counter that may not go above its limit.
type (
	counterState struct{ value, lastVersion int64 }
	add          struct{ n int64 }
	added        struct{ N int64 }
)

var errOverLimit = errors.New("over the limit")

var counter = decider.Decider[counterState, add, added]{
	Decide: func(s counterState, c add) ([]added, error) {
		if s.value+c.n > 10 {
			return nil, errOverLimit
		}

		if c.n == 0 {
			return nil, nil
		}

		return []added{{N: c.n}}, nil
	},
	Evolve: func(s counterState, e decider.Envelope[added]) counterState {
		return counterState{value: s.value + e.Event.N, lastVersion: e.StreamVersion}
	},
}

func TestGivenPastEventsWhenACommandThenNewEvents(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, counter, added{N: 4}, added{N: 5}).
		When(add{n: 1}).
		Then(added{N: 1})
}

func TestGivenPastEventsWhenACommandThenAnError(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, counter, added{N: 4}, added{N: 5}).
		When(add{n: 2}).
		ThenError(errOverLimit)
}

func TestWhenACommandDecidesNothingThenNoEvents(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, counter).
		When(add{n: 0}).
		Then()
}

func TestGivenNumbersPastEventsFromVersionOne(t *testing.T) {
	t.Parallel()

	state := decidertest.Given(t, counter, added{N: 1}, added{N: 1}, added{N: 1}).State()

	if state.lastVersion != 3 {
		t.Errorf("last stream version evolved = %d, want 3", state.lastVersion)
	}
}

func TestThenReportsUnexpectedEvents(t *testing.T) {
	t.Parallel()

	rec := &recorder{TB: t}
	decidertest.Given(rec, counter).When(add{n: 1}).Then(added{N: 2})

	if !rec.failed {
		t.Error("Then accepted events that differ from the expected ones")
	}

	rec = &recorder{TB: t}
	decidertest.Given(rec, counter).When(add{n: 11}).Then(added{N: 11})

	if !rec.failed {
		t.Error("Then accepted an error")
	}
}

func TestThenErrorReportsAnotherOutcome(t *testing.T) {
	t.Parallel()

	rec := &recorder{TB: t}
	decidertest.Given(rec, counter).When(add{n: 1}).ThenError(errOverLimit)

	if !rec.failed {
		t.Error("ThenError accepted events")
	}
}

// recorder notes a failure instead of failing the test.
type recorder struct {
	testing.TB

	failed bool
}

func (r *recorder) Errorf(string, ...any) { r.failed = true }
func (r *recorder) Fatalf(string, ...any) { r.failed = true }
func (*recorder) Helper()                 {}
