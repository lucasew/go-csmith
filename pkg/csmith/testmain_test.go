package csmith

import (
	"os"
	"testing"
)

// TestMain installs a clean Session bag so CreateVariableScalars / pick paths match
// C++ DefaultRndNumGenerator + Probabilities during unit tests — no invent private
// nextCreateVarRng stream. Tests that need a clean slate call ReinstallTestProcessSingletons.
//
// ReinstallTestProcessSingletons replaces the quarantined testAmbientSession with a
// fresh bag then re-seeds Options/Rng/Probabilities/tables. Generate never uses this bag.
func ReinstallTestProcessSingletons() {
	// Replace unit-test ambient bag entirely (Generate is bag-local; no dual-install).
	testAmbientSession = newSession()
	opts := Defaults()
	SetProcessOptionsSess(testAmbientSession, opts)
	SetProcessRngSess(testAmbientSession, NewRng(1))
	SetProcessProbabilitiesSess(testAmbientSession, NewProbabilities(opts))
	InitScopeTableSess(testAmbientSession, opts)
	InitSessionProbabilityTablesSess(testAmbientSession, opts)
	InitAttrGeneratorsSess(testAmbientSession, opts, ProcessProbabilitiesSess(testAmbientSession))
	ClearErrorSess(testAmbientSession)
}

func TestMain(m *testing.M) {
	ReinstallTestProcessSingletons()
	code := m.Run()
	os.Exit(code)
}
