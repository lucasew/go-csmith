package csmith

import (
	"os"
	"testing"
)

// TestMain installs a clean Session bag so CreateVariableScalars / pick paths match
// C++ DefaultRndNumGenerator + Probabilities during unit tests — no invent private
// nextCreateVarRng stream. Tests that need a clean slate call ReinstallTestProcessSingletons.
//
// ReinstallTestProcessSingletons replaces defaultSession with a fresh bag (all
// mutable state session-local) then re-seeds Options/Rng/Probabilities/tables.
func ReinstallTestProcessSingletons() {
	// Drop any Generate-scoped session; replace unit-test bag entirely.
	activeSession = nil
	defaultSession = newSession()
	opts := Defaults()
	SetProcessOptions(opts)
	SetProcessRng(NewRng(1))
	SetProcessProbabilities(NewProbabilities(opts))
	InitScopeTable(opts)
	InitSessionProbabilityTables(opts)
	InitAttrGenerators(opts, ProcessProbabilities())
	ClearError()
}

func TestMain(m *testing.M) {
	ReinstallTestProcessSingletons()
	code := m.Run()
	os.Exit(code)
}
