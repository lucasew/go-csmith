package csmith

import (
	"strings"
	"testing"
)

func TestAddRemoveQualifiers(t *testing.T) {
	q := NewCVQualifiers([]bool{true}, []bool{false})
	q.AddQualifiersSess(testAmbientSession, false, true)
	if len(q.IsConsts) != 2 || !q.IsVolatiles[1] {
		t.Fatal(q)
	}
	q.RemoveQualifiersSess(testAmbientSession, 1)
	if len(q.IsConsts) != 1 || !q.IsConsts[0] {
		t.Fatal(q)
	}
	// over-pop sticky (no invent soft-break partial pop as complete)
	ClearErrorSess(testAmbientSession)
	q.RemoveQualifiersSess(testAmbientSession, 5)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("over-pop RemoveQualifiers must SetError sticky")
	}
	// vector emptied up to fail; residual ERROR sticky
	if len(q.IsConsts) != 0 {
		t.Fatalf("over-pop must stop at empty, got %v", q.IsConsts)
	}
	ClearErrorSess(testAmbientSession)
	// CVQualifiers always live; sticky no invent soft-skip mutators past hole
	(*CVQualifiers)(nil).AddQualifiersSess(testAmbientSession, false, false)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil AddQualifiers must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	(*CVQualifiers)(nil).RemoveQualifiersSess(testAmbientSession, 1)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RemoveQualifiers must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	(*CVQualifiers)(nil).SetConstSess(testAmbientSession, true, 0)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil SetConst must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	(*CVQualifiers)(nil).SetVolatileSess(testAmbientSession, true, 0)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil SetVolatile must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	(*CVQualifiers)(nil).Restrict(AccessWrite, EmptyCGContext().WithSession(testAmbientSession))
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Restrict must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIndirectQualifiersMultiLevelAddrFailClosed(t *testing.T) {
	// CVQualifiers.cpp:510 — assert(level == -1); multi-level & sticky → empty
	ClearErrorSess(testAmbientSession)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	got := q.IndirectQualifiersSess(testAmbientSession, -2)
	if len(got.IsConsts) != 0 || len(got.IsVolatiles) != 0 {
		t.Fatal("multi-level address-of must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("multi-level address-of must SetError sticky")
	}
	// -1 adds one level
	ClearErrorSess(testAmbientSession)
	got = q.IndirectQualifiersSess(testAmbientSession, -1)
	if len(got.IsConsts) != 2 || got.IsConsts[1] {
		t.Fatal(got)
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputFirstQualsRespectsOptions(t *testing.T) {
	// CVQualifiers.cpp:641–648 — assert sticky when bit set but option off
	ClearErrorSess(testAmbientSession)
	prev := ProcessOptionsSess(testAmbientSession)
	opts := Defaults()
	opts.Consts = false
	opts.Volatiles = false
	SetProcessOptionsSess(testAmbientSession, opts)
	defer SetProcessOptionsSess(testAmbientSession, prev)
	q := NewCVQualifiers([]bool{true}, []bool{true})
	if s := q.OutputFirstQualsSess(testAmbientSession); s != "" {
		t.Fatalf("want empty when options off, got %q", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("const bit without Consts option must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	opts.Consts = true
	opts.Volatiles = true
	SetProcessOptionsSess(testAmbientSession, opts)
	if s := q.OutputFirstQualsSess(testAmbientSession); !strings.Contains(s, "const") || !strings.Contains(s, "volatile") {
		t.Fatal(s)
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("valid OutputFirstQuals must not leave sticky")
	}
}

func TestNewCVQualifiersUnequalLenFailClosed(t *testing.T) {
	// CVQualifiers.cpp:96 — sizes must match; sticky empty (no invent truncate)
	ClearErrorSess(testAmbientSession)
	q := NewCVQualifiers([]bool{true, false}, []bool{false})
	if len(q.IsConsts) != 0 || len(q.IsVolatiles) != 0 {
		t.Fatal(q)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("unequal len NewCVQualifiers must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSanityCheck(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	// int: level 0 → need 1 qualifier slot
	q := NewCVQualifiers([]bool{false}, []bool{false})
	if !q.SanityCheckSess(testAmbientSession, GetIntType()) {
		t.Fatal("int")
	}
	// int*: level 1 → need 2 slots
	pt := PointerTo(GetIntType())
	if q.SanityCheckSess(testAmbientSession, pt) {
		t.Fatal("short qfer for ptr")
	}
	q2 := NewCVQualifiers([]bool{false, false}, []bool{false, false})
	if !q2.SanityCheckSess(testAmbientSession, pt) {
		t.Fatal("ptr ok")
	}
	// CVQualifiers.cpp:527 assert(t) sticky
	if q.SanityCheckSess(testAmbientSession, nil) {
		t.Fatal("nil type SanityCheck invent")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil type SanityCheck must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRandomAddQualifiers(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	base := NewCVQualifiers([]bool{false}, []bool{false})
	got := base.RandomAddQualifiersSess(testAmbientSession, NewRng(1), opts, probs, true)
	if len(got.IsConsts) != 2 {
		t.Fatal(len(got.IsConsts))
	}
	// no_volatile → second vol false
	if got.IsVolatiles[1] {
		t.Fatal("vol")
	}
	// match_exact → always false,false
	opts.MatchExactQualifiers = true
	got2 := base.RandomAddQualifiersSess(testAmbientSession, NewRng(1), opts, probs, false)
	if got2.IsConsts[1] || got2.IsVolatiles[1] {
		t.Fatal("exact")
	}
}

func TestOutputFirstQuals(t *testing.T) {
	q := NewCVQualifiers([]bool{true}, []bool{true})
	s := q.OutputFirstQualsSess(testAmbientSession)
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
	// Lhs.cpp:200 — assert(!qfer.is_const()); sticky error, no invent strip / invent quals shell
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalars("g_c", GetIntType(), true, false)
	lhs := &Lhs{Var: v, Type: GetIntType()}
	q := lhs.GetQualifiers()
	if len(q.IsConsts) != 0 || len(q.IsVolatiles) != 0 {
		t.Fatal("const LHS residual must fail closed empty quals, not invent shell", q)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("const LHS qfer must set sticky error")
	}
	ClearErrorSess(testAmbientSession)
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
	_ = lhs.IsVolatileSess(testAmbientSession) // exercise path
}

func TestIsConstVolatileAfterDerefIncompleteTypeFailClosed(t *testing.T) {
	// soft invent: Type nil / OOB peel → not const/vol → allow write/access
	// fair: incomplete sticky fails closed const/vol true
	// 2-level non-vol qfer so Qfer path does not OOB-fail-closed as full vol first
	ClearErrorSess(testAmbientSession)
	v := &Variable{Name: "g_p", Qfer: NewCVQualifiers([]bool{false, false}, []bool{false, false})}
	// Type nil
	if !v.IsConstAfterDeref(1) {
		t.Fatal("nil Type must fail closed as const after deref")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Type IsConstAfterDeref must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !v.IsVolatileAfterDeref(1) {
		t.Fatal("nil Type must fail closed as volatile after deref")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Type IsVolatileAfterDeref must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !v.IsPartialVolatileAfterDeref(1) {
		t.Fatal("nil Type must fail closed as partial volatile")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Type IsPartialVolatileAfterDeref must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// OOB peel: pointer type peels once then nil at high deref.
	// Qfer OOB (len 2, deref 3) fail-closed const/vol non-sticky first; Type peel
	// sticky only when Qfer does not already short-circuit (empty qfer + nil type).
	v.Type = PointerTo(GetIntType())
	if !v.IsConstAfterDeref(3) {
		t.Fatal("OOB peel must fail closed as const")
	}
	if !v.IsVolatileAfterDeref(3) {
		t.Fatal("OOB peel must fail closed as volatile")
	}
	// Type peel sticky path: empty qfer + incomplete type
	ClearErrorSess(testAmbientSession)
	v2 := &Variable{Name: "g_q", Type: nil}
	if !v2.IsConstAfterDeref(0) {
		t.Fatal("nil Type empty qfer must fail closed const")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Type IsConstAfterDeref must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// GetFieldID incomplete parent FieldVars sticky -1
	parent := &Variable{Name: "u", Type: &Type{isUnion: true}}
	f0 := &Variable{Name: "u.f0", Type: GetIntType(), FieldVarOf: parent}
	parent.FieldVars = []*Variable{nil, f0}
	if f0.GetFieldID() != -1 {
		t.Fatal("FieldVars hole must fail closed GetFieldID -1")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("FieldVars hole GetFieldID must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Variable always live; sticky -1 (no invent not-field soft-skip past hole)
	if (*Variable)(nil).GetFieldID() != -1 {
		t.Fatal("nil GetFieldID must fail closed -1")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil GetFieldID must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}
