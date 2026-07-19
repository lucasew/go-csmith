package csmith

import (
	"strings"
	"testing"
)

func TestAddRemoveQualifiers(t *testing.T) {
	q := NewCVQualifiers([]bool{true}, []bool{false})
	q.AddQualifiers(false, true)
	if len(q.IsConsts) != 2 || !q.IsVolatiles[1] {
		t.Fatal(q)
	}
	q.RemoveQualifiers(1)
	if len(q.IsConsts) != 1 || !q.IsConsts[0] {
		t.Fatal(q)
	}
}

func TestIndirectQualifiersMultiLevelAddrFailClosed(t *testing.T) {
	// CVQualifiers.cpp:510 — assert(level == -1); multi-level & → empty
	q := NewCVQualifiers([]bool{false}, []bool{false})
	got := q.IndirectQualifiers(-2)
	if len(got.IsConsts) != 0 || len(got.IsVolatiles) != 0 {
		t.Fatal("multi-level address-of must fail closed empty")
	}
	// -1 adds one level
	got = q.IndirectQualifiers(-1)
	if len(got.IsConsts) != 2 || got.IsConsts[1] {
		t.Fatal(got)
	}
}

func TestOutputFirstQualsRespectsOptions(t *testing.T) {
	// CVQualifiers.cpp:641–648 — no invent keyword when option off
	prev := ProcessOptions()
	opts := Defaults()
	opts.Consts = false
	opts.Volatiles = false
	SetProcessOptions(opts)
	defer SetProcessOptions(prev)
	q := NewCVQualifiers([]bool{true}, []bool{true})
	if s := q.OutputFirstQuals(); s != "" {
		t.Fatalf("want empty when options off, got %q", s)
	}
	opts.Consts = true
	opts.Volatiles = true
	SetProcessOptions(opts)
	if s := q.OutputFirstQuals(); !strings.Contains(s, "const") || !strings.Contains(s, "volatile") {
		t.Fatal(s)
	}
}

func TestNewCVQualifiersUnequalLenFailClosed(t *testing.T) {
	// CVQualifiers.cpp:96 — sizes must match; truncate to min
	q := NewCVQualifiers([]bool{true, false}, []bool{false})
	if len(q.IsConsts) != 1 || len(q.IsVolatiles) != 1 {
		t.Fatal(q)
	}
}

func TestSanityCheck(t *testing.T) {
	// int: level 0 → need 1 qualifier slot
	q := NewCVQualifiers([]bool{false}, []bool{false})
	if !q.SanityCheck(GetIntType()) {
		t.Fatal("int")
	}
	// int*: level 1 → need 2 slots
	pt := PointerTo(GetIntType())
	if q.SanityCheck(pt) {
		t.Fatal("short qfer for ptr")
	}
	q2 := NewCVQualifiers([]bool{false, false}, []bool{false, false})
	if !q2.SanityCheck(pt) {
		t.Fatal("ptr ok")
	}
}

func TestRandomAddQualifiers(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	base := NewCVQualifiers([]bool{false}, []bool{false})
	got := base.RandomAddQualifiers(NewRng(1), opts, probs, true)
	if len(got.IsConsts) != 2 {
		t.Fatal(len(got.IsConsts))
	}
	// no_volatile → second vol false
	if got.IsVolatiles[1] {
		t.Fatal("vol")
	}
	// match_exact → always false,false
	opts.MatchExactQualifiers = true
	got2 := base.RandomAddQualifiers(NewRng(1), opts, probs, false)
	if got2.IsConsts[1] || got2.IsVolatiles[1] {
		t.Fatal("exact")
	}
}

func TestOutputFirstQuals(t *testing.T) {
	q := NewCVQualifiers([]bool{true}, []bool{true})
	s := q.OutputFirstQuals()
	if s != "const volatile " {
		t.Fatal(s)
	}
}

func TestGetAllQualifiers(t *testing.T) {
	all := GetAllQualifiers(50, 50)
	if len(all) != 4 {
		t.Fatal(len(all))
	}
}

func TestLhsGetLvarsAndQualifiers(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	tgt := CreateVariableScalars("g_1", GetIntType(), true, false)
	facts := []*FactPointTo{MakeFactPointTo(p, tgt)}
	// bare pointer as LHS (want int* → indirect 0)
	lhs0 := &Lhs{Var: p, Type: PointerTo(GetIntType())}
	if lhs0.IndirectLevel() != 0 {
		t.Fatal(lhs0.IndirectLevel())
	}
	// deref *p (want int)
	lhs1 := &Lhs{Var: p, Type: GetIntType()}
	if lhs1.IndirectLevel() != 1 {
		t.Fatal(lhs1.IndirectLevel())
	}
	lvars := lhs1.GetLvars(facts)
	if len(lvars) != 1 || lvars[0] != tgt {
		t.Fatal(lvars)
	}
	ptrs := lhs1.GetReferencedPtrs()
	if len(ptrs) != 1 || ptrs[0] != p {
		t.Fatal(ptrs)
	}
	// qfer with 2 levels for pointer
	p.Qfer = NewCVQualifiers([]bool{false, true}, []bool{false, false})
	// indirect 1 → pop one → remaining [false]
	q := lhs1.GetQualifiers()
	if len(q.IsConsts) != 1 {
		t.Fatal(q)
	}
}

func TestLhsGetQualifiersConstSetsError(t *testing.T) {
	// Lhs.cpp:200 — assert(!qfer.is_const()); sticky error, no invent strip
	ClearError()
	v := CreateVariableScalars("g_c", GetIntType(), true, false)
	lhs := &Lhs{Var: v, Type: GetIntType()}
	_ = lhs.GetQualifiers()
	if !HasError() {
		t.Fatal("const LHS qfer must set sticky error")
	}
	ClearError()
}

func TestLhsVisitIndicesOK(t *testing.T) {
	av := &ArrayVariable{
		Variable: Variable{Name: "g_a", IsArray: true, Type: GetIntType()},
		Indices:  []string{"i"},
	}
	av.AsArray = av
	lhs := &Lhs{Var: &av.Variable, Type: GetIntType()}
	if !lhs.VisitIndices(nil, Defaults()) {
		t.Fatal("indices")
	}
}

func TestLhsIsVolatileAfterDeref(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	// volatile at storage (after one deref of pointer-to-vol?)
	// IsVolatileAfterDeref(0) for bare uses Qfer
	p.Qfer = NewCVQualifiers([]bool{false, true}, []bool{false, true})
	// want int: indirect 1 → check volatile after deref 1
	lhs := &Lhs{Var: p, Type: GetIntType()}
	_ = lhs.IsVolatile() // exercise path
}

func TestIsConstVolatileAfterDerefIncompleteTypeFailClosed(t *testing.T) {
	// soft invent: Type nil / OOB peel → not const/vol → allow write/access
	// fair: incomplete fails closed const/vol true
	// 2-level non-vol qfer so Qfer path does not OOB-fail-closed as full vol first
	v := &Variable{Name: "g_p", Qfer: NewCVQualifiers([]bool{false, false}, []bool{false, false})}
	// Type nil
	if !v.IsConstAfterDeref(1) {
		t.Fatal("nil Type must fail closed as const after deref")
	}
	if !v.IsVolatileAfterDeref(1) {
		t.Fatal("nil Type must fail closed as volatile after deref")
	}
	if !v.IsPartialVolatileAfterDeref(1) {
		t.Fatal("nil Type must fail closed as partial volatile")
	}
	// OOB peel: pointer type peels once then nil at high deref
	v.Type = PointerTo(GetIntType())
	if !v.IsConstAfterDeref(3) {
		t.Fatal("OOB peel must fail closed as const")
	}
	if !v.IsVolatileAfterDeref(3) {
		t.Fatal("OOB peel must fail closed as volatile")
	}
	// GetFieldID incomplete parent FieldVars
	parent := &Variable{Name: "u", Type: &Type{isUnion: true}}
	f0 := &Variable{Name: "u.f0", Type: GetIntType(), FieldVarOf: parent}
	parent.FieldVars = []*Variable{nil, f0}
	if f0.GetFieldID() != -1 {
		t.Fatal("FieldVars hole must fail closed GetFieldID -1")
	}
}
