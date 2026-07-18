package csmith

import (
	"os"
	"testing"
)

// TestMain installs process singletons so CreateVariableScalars / pick paths match
// C++ DefaultRndNumGenerator + Probabilities during unit tests — no invent private
// nextCreateVarRng stream. Tests that need a clean slate save/restore Process*.
func TestMain(m *testing.M) {
	opts := Defaults()
	SetProcessOptions(opts)
	SetProcessRng(NewRng(1))
	SetProcessProbabilities(NewProbabilities(opts))
	InitScopeTable(opts)
	InitSessionProbabilityTables(opts)
	InitAttrGenerators(opts, ProcessProbabilities())
	code := m.Run()
	os.Exit(code)
}
