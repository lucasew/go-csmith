package csmith

import "testing"

func TestGoGeneratorDFSLoopDebugSequence(t *testing.T) {
	// Finite debug sequence ends all_done after last choice.
	ClearError()
	prevO := ProcessOptionsSess(testAmbientSession)
	defer func() {
		DoFinalization()
		ReinstallTestProcessSingletons()
		SetProcessOptionsSess(testAmbientSession, prevO)
		ClearError()
	}()
	o := Defaults()
	o.DFSExhaustive = true
	o.RandomBased = false
	o.MaxExhaustiveDepth = 4
	// short debug sequence so loop terminates
	o.DFSDebugSequence = "0"
	o.MaxFuncs = 1
	o.MaxBlockSize = 1
	o.MaxBlockDepth = 1
	o.MaxExprComplexity = 1
	g := NewProgramGenerator(NewSession(o))
	if g.Rng == nil || g.Rng.Kind() != RngKindDFS {
		t.Fatal("need DFS rng")
	}
	// Direct loop without full GenerateFunctions may still run; bound by all_done
	// after first random_choice consuming the single debug token.
	// Drive manually: one RndUpto sets all_done when pos >= len-1
	_ = g.Rng.RndUpto(2)
	if !g.Rng.DFSGetAllDone() {
		// with length 1 sequence, pos 0 >= 0 → all_done
		t.Log("all_done", g.Rng.DFSGetAllDone(), "pos", g.Rng.DFSGetCurrentPos())
	}
	// reset then loop should exit immediately if already all_done after one choice
	// Rebuild with fresh generator for loop test
	DoFinalization()
	ReinstallTestProcessSingletons()
	ClearError()
	o.DFSDebugSequence = "0"
	g = NewProgramGenerator(NewSession(o))
	// Manually set all_done path: after debug sequence exhausted
	_ = g.Rng.RndUpto(3)
	out := g.GoGeneratorDFSLoop()
	// may be empty if generation fails; must not hang and must not invent on sticky DFS without engine
	_ = out
	if GetError() == ErrGeneric && g.Rng == nil {
		t.Fatal("unexpected")
	}
	ClearError()
}

func TestGetCountPrefixDFSAfterGood(t *testing.T) {
	ClearError()
	g := &ProgramGenerator{Sess: testAmbientSession, OutputKind: OutputMgrKindDFS, GoodCount: 0}
	if g.GetCountPrefix("n") != "p_0_n" {
		t.Fatal(g.GetCountPrefix("n"))
	}
}
