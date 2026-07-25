// Unit-test session bag (package csmith tests only - not production ambient).
// Generate is bag-local pure; tests share this bag for residual sticky convenience.
package csmith

// testAmbientSession is the shared bag for unit tests outside Generate.
// Prefer NewSession / per-test bags for isolation when tests mutate bag state.
var testAmbientSession = NewSession(Defaults())

// currentSession returns the unit-test bag (tests only).
func currentSession() *Session { return testAmbientSession }

// CurrentSession returns the unit-test bag (tests only).
func CurrentSession() *Session { return testAmbientSession }
