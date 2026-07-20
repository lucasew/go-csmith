package csmith

import (
	"os"
	"testing"
)

// TestMain installs process singletons so CreateVariableScalars / pick paths match
// C++ DefaultRndNumGenerator + Probabilities during unit tests — no invent private
// nextCreateVarRng stream. Tests that need a clean slate save/restore Process*.
// ReinstallTestProcessSingletons restores TestMain process handles after
// DoFinalization / RandomNumberDoFinalization wiped them (library multi-run).
func ReinstallTestProcessSingletons() {
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
