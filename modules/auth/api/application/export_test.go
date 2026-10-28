package application

import "time"

// RunNow makes the service do its after-the-answer work before returning, so tests can look at
// the result.
func RunNow(s *Service) { s.later = func(f func()) { f() } }

// SetClock makes the service and its limiter read the time from now, so tests can move it.
func SetClock(s *Service, now func() time.Time) {
	s.now = now
	s.limiter.now = now
}
