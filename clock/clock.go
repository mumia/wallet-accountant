package clock

import (
	"fmt"
	"log"
	"time"
)

// Clock is meant be included as a pointer field on a struct. Leaving the
// instance as a nil reference will cause any calls on the *Clock to forward
// to the corresponding functions in the standard time package. This is meant
// to be the behavior in production. In testing, set the field to a non-nil
// instance of a *Clock to provide a frozen time instant whenever UTCNow()
// is called.
// Also records any calls made to Sleep with the durations for later inspection.
// Based on concepts found in https://smartystreets.com/blog/2015/09/go-testing-part-5-testing-with-time
type Clock struct {
	instants         []Instant
	instantsIndex    int
	napsRecorder     []time.Duration
	TimeoutsRecorder []time.Duration
	TimeoutChannel   *chan time.Time
}

func NewClock() Clock {
	return Clock{}
}

type Instant struct {
	Label   string
	Instant time.Time
}

// Freeze creates a new *Clock instance with an internal time instant.
// This function is meant to be called from test code.
// If passed, the timeout channel will be used for all 'After' calls
func Freeze(instants []Instant, timeoutChannel *chan time.Time) *Clock {
	if timeoutChannel == nil {
		timeoutChannelValue := make(chan time.Time)
		timeoutChannel = &timeoutChannelValue
	}

	return &Clock{instants: instants, TimeoutChannel: timeoutChannel}
}

// Now -> time.Now() //unless frozen
func (clock *Clock) Now() time.Time {
	if clock == nil || len(clock.instants) == 0 {
		return time.Now()
	}
	defer clock.next()
	instant := clock.instants[clock.instantsIndex]

	clock.logDebug(fmt.Sprintf("Now: %s(%d) - %s", instant.Label, clock.instantsIndex, instant.Instant.String()))

	return instant.Instant
}

func (clock *Clock) next() {
	clock.instantsIndex++
	if clock.instantsIndex == len(clock.instants) {
		clock.instantsIndex = 0
	}
}

// Since -> time.Since(instant)  //unless frozen
func (clock *Clock) Since(instant time.Time) time.Duration {
	now := clock.Now()
	since := now.Sub(instant)

	clock.logDebug(fmt.Sprintf("Since: %s (Now: %s)", since.String(), now.String()))

	return since
}

// Sleep -> time.Sleep  //unless frozen
func (clock *Clock) Sleep(duration time.Duration) {
	if clock == nil {
		time.Sleep(duration)

		return
	}

	clock.logDebug(fmt.Sprintf("Sleep: %s", duration.String()))

	clock.napsRecorder = append(clock.napsRecorder, duration)
}

func (clock *Clock) GetRecordedNaps() []time.Duration {
	return clock.napsRecorder
}

// After -> time.After(duration)  //unless frozen
func (clock *Clock) After(duration time.Duration) <-chan time.Time {
	if clock == nil {
		return time.After(duration)
	}

	clock.logDebug(fmt.Sprintf("After: %s", duration.String()))

	return *clock.TimeoutChannel
}

func (clock *Clock) logDebug(message string) {
	log.Printf("Clock debug: %s", message)
}
