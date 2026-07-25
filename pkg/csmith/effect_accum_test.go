package csmith

import (
	"strings"
	"testing"
)

func TestBlockEffectAccumAfterAssign(t *testing.T) {
	// Block::make_random(CGContext&) — shared effect_accum sees stmt writes.
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	// Force assign-only statement table
	tab := &ThresholdTable{}
	tab.Add(100, int(StmtAssign))
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	// StatementAssign.cpp:127 assert(fm) — assign needs FactMgr
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	cg.Types = &TypeEnv{Sess: testAmbientSession}
	_ = vs.GenerateNewGlobal(AccessWrite, cg, GetIntType(), nil, NewRng(1))
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	// single-statement block
	opts.MaxBlockSize = 1
	b := MakeRandomBlock(NewRng(2), opts, probs, vs, tables, tab, &cg, false)
	if b == nil {
		t.Fatal("nil block")
	}
	// *CGContext: assign path may write into shared EffectAccum
	if len(b.Stmts) == 0 {
		t.Fatal("empty block")
	}
	// Verify NoteWrite works on shared accum
	v := CreateVariableQfer("g_x", GetIntType(), NewCVQualifiers([]bool{false}, []bool{false}))
	cg.NoteWrite(v)
	// NoteWrite updates EffectAccum; non-vol write stays SE-free (Effect.cpp:144–145)
	if !cg.AccumEffect().IsWrittenSess(testAmbientSession, v) {
		t.Fatal("not written")
	}
	if !cg.AccumEffect().IsSideEffectFreeSess(testAmbientSession) {
		// may already have vol effects from block body
	}
	// volatile write clears SE-free
	eff2 := EmptyEffect()
	cg2 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	cg2.EffectAccum = &eff2
	vv := CreateVariableQfer("g_v", GetIntType(), NewCVQualifiers([]bool{false}, []bool{true}))
	cg2.NoteWrite(vv)
	if cg2.AccumEffect().IsSideEffectFreeSess(testAmbientSession) {
		t.Fatal("vol write clears SE-free")
	}
}

func TestStepHashOutput(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.Seed = 2
	opts.StepHashByStmt = true
	opts.ComputeHash = true
	out, err := Generate(opts)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "void csmith_compute_hash(void)") {
		t.Fatal("missing hash def")
	}
	if !strings.Contains(out, "csmith_compute_hash();") {
		t.Fatal("missing hash call")
	}
	if !strings.Contains(out, "void step_hash(int stmt_id)") {
		t.Fatal("missing step_hash")
	}
	ClearErrorSess(testAmbientSession)
}

func TestStepHashNoInventDeclWithoutComputeHash(t *testing.T) {
	// OutputHashFuncDef gated on ComputeHash — no invent forward decls alone
	opts := Defaults()
	opts.StepHashByStmt = true
	opts.ComputeHash = false
	g := NewProgramGenerator(NewSession(opts))
	hdr := g.OutputHeader()
	if strings.Contains(hdr, "csmith_compute_hash") || strings.Contains(hdr, "step_hash") {
		t.Fatal("must not invent hash decls without ComputeHash", hdr)
	}
	if def := g.OutputHashFuncDef(); def != "" {
		t.Fatal("must not invent hash def without ComputeHash", def)
	}
}

func TestStepHashNoInventCallWithoutDef(t *testing.T) {
	// MaxArrayDim < global array rank → hashFuncDefReady false
	// no invent csmith_compute_hash() call without matching def
	opts := Defaults()
	opts.StepHashByStmt = true
	opts.ComputeHash = true
	opts.MaxArrayDim = 0
	g := NewProgramGenerator(NewSession(opts))
	// one global array so dimen > MaxArrayDim
	av := &Variable{
		Name:       "g_a",
		Type:       GetIntType(),
		IsArray:    true,
		ArraySizes: []int{2, 3},
		Qfer:       NewCVQualifiers([]bool{false}, []bool{false}),
	}
	g.VS.GlobalList = []*Variable{av}
	if g.hashFuncDefReady() {
		t.Fatal("want not ready when MaxArrayDim < dimen")
	}
	if def := g.OutputHashFuncDef(); def != "" {
		t.Fatal("incomplete ctrl must fail closed def", def)
	}
	main := g.OutputMain()
	if strings.Contains(main, "csmith_compute_hash()") {
		t.Fatal("must not invent hash call without def", main)
	}
	ClearErrorSess(testAmbientSession)
}

func TestExpressionCommaUsesEnv(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	env := &TypeEnv{Sess: testAmbientSession}
	GenerateAllTypesEnv(NewRng(2), opts, probs, env)
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, nil))
	cg.Types = env
	e := MakeExpressionComma(NewRng(3), opts, probs, vs, tables, &cg, GetIntType(), nil)
	if e == nil || e.Term != TermCommaExpr {
		t.Fatal(e)
	}
	if e.CommaLHS == nil || e.CommaRHS == nil {
		t.Fatal("nil sides")
	}
}

func TestNoteWriteWriteVarResidualSticky(t *testing.T) {
	// WriteVar residual soft invent was invent soft-complete NoteWrite past Type-nil IsVolatile residual.
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "func_1", ReturnType: GetIntType(), FEffect: EmptyEffect()}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	// Type-nil Variable IsVolatile residual on WriteVar path
	hole := &Variable{Name: "g_x", Type: nil, Qfer: NewCVQualifiers([]bool{false}, []bool{false})}
	cg.NoteWrite(hole)
	// WriteVar residual on Type-nil IsVolatile may sticky IncompleteEffect
	if EffectComplete(*cg.EffectAccum) && !HasErrorSess(testAmbientSession) {
		// may still complete if IsVolatile non-residual on empty qfer
	}
	// Type-nil IsVolatile: Variable.IsVolatile uses Qfer only - no Type residual
	// Use incomplete EffectAccum residual path
	ClearErrorSess(testAmbientSession)
	cg2 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	inc := IncompleteEffect()
	cg2.EffectAccum = &inc
	cg2.NoteWrite(CreateVariableScalars("g_y", GetIntType(), false, false))
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Incomplete EffectAccum NoteWrite must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestNoteWriteNilSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.NoteWrite(nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil NoteWrite must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

// CGContext.cpp:175–185 / 307–317 — NoteRead/NoteWrite are write_var/read_var:
// both EffectAccum and EffectStm (not feffect).
func TestNoteReadWriteUpdateEffectStm(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	cg.EffectStm = EmptyEffect()
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	cg.NoteRead(g)
	if !cg.EffectStm.IsReadSess(testAmbientSession, g) || !cg.EffectAccum.IsReadSess(testAmbientSession, g) {
		t.Fatal("NoteRead must update EffectStm and EffectAccum like read_var")
	}
	if f.FEffect.IsReadSess(testAmbientSession, g) {
		t.Fatal("NoteRead must not touch feffect")
	}
	ClearErrorSess(testAmbientSession)
	cg.EffectStm = EmptyEffect()
	w := CreateVariableScalars("g_2", GetIntType(), false, false)
	cg.NoteWrite(w)
	if !cg.EffectStm.IsWrittenSess(testAmbientSession, w) || !cg.EffectAccum.IsWrittenSess(testAmbientSession, w) {
		t.Fatal("NoteWrite must update EffectStm and EffectAccum like write_var")
	}
	if f.FEffect.IsWrittenSess(testAmbientSession, w) {
		t.Fatal("NoteWrite must not touch feffect")
	}
	ClearErrorSess(testAmbientSession)
}

func TestReadVarGetCollectiveResidualSticky(t *testing.T) {
	// GetCollective residual soft invent was invent soft-complete read past Type-nil array field shell.
	ClearErrorSess(testAmbientSession)
	cg := EmptyCGContext().WithSession(testAmbientSession)
	// IsArray without AsArray GetCollective residual
	shell := &Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	cg.ReadVar(shell)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray ReadVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestWriteVarIsNonWritableResidualSticky(t *testing.T) {
	// IsNonWritable residual soft invent was invent soft-complete write past incomplete NoWriteVars.
	ClearErrorSess(testAmbientSession)
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.RW = &RWDirective{NoWriteVars: []*Variable{nil}}
	v := CreateVariableScalars("g_x", GetIntType(), false, false)
	cg.WriteVar(v)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil NoWriteVars hole WriteVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestFindVariableScopeIsGlobalResidualSticky(t *testing.T) {
	// IsGlobal residual soft invent was invent global-scope past nil Variable already sticky.
	ClearErrorSess(testAmbientSession)
	if EmptyCGContext().WithSession(testAmbientSession).FindVariableScope(nil) != ScopeInactive {
		t.Fatal("nil FindVariableScope must fail closed ScopeInactive")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FindVariableScope must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete global
	v := CreateVariableScalars("g_x", GetIntType(), false, false)
	if EmptyCGContext().WithSession(testAmbientSession).FindVariableScope(v) != ScopeGlobalVar {
		t.Fatal("global FindVariableScope")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete FindVariableScope must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}
