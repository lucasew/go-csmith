// Session holds mutable generation state for one run (or the unit-test default).
//
// Model (SPEC / library contract):
//   - Read-only package data: opcode tables, probability name maps, simpleTypes
//     templates, builtin string lists — never mutated per run.
//   - Session-specific: Options, Rng, Probabilities, stmt/scope/expr tables,
//     and everything DoFinalization clears (caches, labels, gensym, Error, …).
//
// Concurrent Generate in one process is not supported (upstream csmith is one
// process / one generation). Fuzz workers are separate OS processes and need
// no shared-session lock. generateMu is therefore unnecessary.
//
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// Session is the CGOptions / Probabilities / RNG / table bag for one generation
// (or the long-lived default bag used by unit tests via SetProcess*).
type Session struct {
	Opts         Options
	Probs        *Probabilities
	Rng          *Rng
	StmtTab      *ThresholdTable
	ScopeTab     *ThresholdTable
	AssignOpsTab *DistributionTable
	ExprTables   *ExprTables
	ProgramGen   *ProgramGenerator
	// RandomNumber is RandomNumber::instance_ for this session.
	RandomNumber *RandomNumber
}

// defaultSession is active when no Generate is running (unit tests, library
// helpers that call SetProcessOptions without NewProgramGenerator).
var defaultSession = &Session{Opts: Defaults()}

// activeSession is the Generate-scoped session; nil means use defaultSession.
var activeSession *Session

func currentSession() *Session {
	if activeSession != nil {
		return activeSession
	}
	return defaultSession
}

// activateSession makes s the Process* target until the returned restore runs.
// Nested activate is supported for tests (restore previous).
func activateSession(s *Session) (restore func()) {
	prev := activeSession
	activeSession = s
	return func() { activeSession = prev }
}

// BeginGenerateSession installs a fresh session for one full-program Generate
// and returns restore that deactivates it. Caller should DoFinalization at the
// end of generation (Initialize already runs a mid-setup finalization).
func BeginGenerateSession() (restore func()) {
	s := &Session{Opts: Defaults()}
	return activateSession(s)
}
