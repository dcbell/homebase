// Package clock is the API's source of "now". Production code uses Real; tests
// inject a Fake so time-based behavior (Reminders, retention) is deterministic.
package clock

import (
	"sync"
	"time"
)

// Clock reports the current time.
type Clock interface {
	Now() time.Time
}

// Real returns the system clock.
func Real() Clock {
	return realClock{}
}

type realClock struct{}

func (realClock) Now() time.Time {
	return time.Now()
}

// Fake is a Clock that only moves when told to. It is safe for concurrent use.
type Fake struct {
	mu  sync.Mutex
	now time.Time
}

// NewFake returns a Fake clock stopped at now.
func NewFake(now time.Time) *Fake {
	return &Fake{now: now}
}

func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Set moves the clock to now.
func (f *Fake) Set(now time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = now
}

// Advance moves the clock forward by d.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}
