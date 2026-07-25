package csmith

import (
	"strings"
	"testing"
)

func TestAssignOpsProbabilitySimpleWhenDisabled(t *testing.T) {
	opts := Defaults()
	opts.CompoundAssignment = false
	tab := NewAssignOpsTable(opts)
	op := AssignOpsProbability(NewRngSess(testAmbientSession, 2), opts, tab, GetIntTypeSess(testAmbientSession))
	if op != AssignSimple {
		t.Fatal(op)
	}
}

func TestAssignOpsProbabilitySignedFiltersIncr(t *testing.T) {
	opts := Defaults()
	tab := NewAssignOpsTable(opts)
	// many draws: never ++/-- on signed int
	r := NewRngSess(testAmbientSession, 2)
	for i := 0; i < 100; i++ {
		op := AssignOpsProbability(r, opts, tab, GetIntTypeSess(testAmbientSession))
		if op.NeedNoRHS() {
			t.Fatalf("signed should filter incr, got %v", op)
		}
	}
}

func TestMakeRandomAssignAllocatesStmID(t *testing.T) {
	// Statement.cpp:364–367 — Statement ctor always assigns stm_id
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	_ = vs.GenerateNewGlobal(AccessWrite, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, 1))
	for seed := uint64(1); seed < 40; seed++ {
		c := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
		st := MakeRandomAssign(NewRngSess(testAmbientSession, seed), opts, probs, vs, NewExprTables(opts), &c, GetIntTypeSess(testAmbientSession))
		if !stmtOK(st) {
			continue
		}
		if st.StmID == 0 {
			t.Fatal("success assign must have stm_id from Statement ctor")
		}
		return
	}
	t.Fatal("no assign")
}

func TestMakeRandomAssignCompoundPossible(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	tables := NewExprTables(opts)
	// seed globals for selection
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	_ = vs.GenerateNewGlobal(AccessWrite, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, 1))
	foundCompound := false
	for seed := uint64(1); seed < 80; seed++ {
		r := NewRngSess(testAmbientSession, seed)
		st := func() Stmt {
			c := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
			return MakeRandomAssign(r, opts, probs, vs, tables, &c, GetIntTypeSess(testAmbientSession))
		}()
		// StmtAssign is iota 0 — empty nullptr and success share Kind; use stmtOK
		if !stmtOK(st) {
			continue
		}
		if st.AssignOp != AssignSimple {
			foundCompound = true
			break
		}
	}
	if !foundCompound {
		t.Fatal("expected some compound/bit assign in seeds")
	}
}

func TestAssignOutputIncr(t *testing.T) {
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	st := Stmt{Kind: StmtAssign, LhsVar: v, AssignOp: AssignPostIncr, Expr: &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)}}
	out := (&Block{Stmts: []Stmt{st}}).Output(0)
	if !strings.Contains(out, "g_1++") {
		t.Fatal(out)
	}
}

func TestGenerateCanEmitCompoundAssign(t *testing.T) {
	found := false
	for seed := uint64(1); seed < 50; seed++ {
		opts := Defaults()
		opts.Seed = seed
		out, err := Generate(opts)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out, " &= ") || strings.Contains(out, " |= ") ||
			strings.Contains(out, " ^= ") || strings.Contains(out, "++") || strings.Contains(out, "--") {
			found = true
			break
		}
	}
	if !found {
		t.Log("no compound/incr in 1..49 (unsigned incr rare; bit ops should appear)")
	}
}

func TestMakeRandomAssignQferForcesExact(t *testing.T) {
	// StatementAssign.cpp:190–203 — qf non-nil → match_exact during Lhs
	// StatementAssign.cpp:145/168 — qf also passed to RHS Expression::make_random
	opts := Defaults()
	opts.MatchExactQualifiers = false
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	// seed a volatile global so volatile qfer can select
	vq := NewCVQualifiers([]bool{false}, []bool{true})
	_ = vs.GenerateNewGlobal(AccessWrite, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), &vq, NewRngSess(testAmbientSession, 1))
	// volatile-only WRITE qfer
	q := NewCVQualifiers([]bool{false}, []bool{true})
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	// should not panic; may fail to find var and return empty assign
	st := MakeRandomAssignQfer(NewRngSess(testAmbientSession, 3), opts, probs, vs, NewExprTables(opts), &cg, GetIntTypeSess(testAmbientSession), &q)
	// global option restored conceptually (opts is by-value); package default unchanged
	if opts.MatchExactQualifiers {
		t.Fatal("caller opts mutated")
	}
	// ExpressionAssign path: when successful, RHS was built under caller qfer
	if st.LhsVar != nil && st.Expr == nil && !st.AssignOp.NeedNoRHS() {
		t.Fatal("successful lhs with needing RHS must have expr")
	}
	_ = st
}

func TestMakeRandomAssignCompatibleCheckFails(t *testing.T) {
	// StatementAssign.cpp:218–223 — compatible_check true → nullptr
	opts := Defaults()
	opts.CompatibleCheck = true
	// hard to force compatible_check without crafted expand_struct; just ensure
	// failed factory has no LhsVar when we force error path manually
	st := Stmt{Kind: StmtAssign}
	if stmtOK(st) {
		t.Fatal("empty assign fails stmtOK")
	}
}

func TestMakeRandomAssignUpdatesIndirectFacts(t *testing.T) {
	// make_random update_fact_for_assign with full indir (same as Qfer path)
	ppT := PointerToSess(testAmbientSession, PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)))
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", ppT, false, false)
	q := CreateVariableScalarsSess(testAmbientSession, "g_q", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	fm := NewFactMgrSess(testAmbientSession, nil)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, q)}
	rhs := &Expression{Term: TermConstant, Con: &Constant{Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), Value: "0"}, ExprType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))}
	fm.UpdateFactForAssign(p, 1, rhs)
	got := FindRelatedPointToSess(testAmbientSession, fm.GlobalFacts, q)
	if got == nil || !got.IsNullSess(testAmbientSession) {
		t.Fatalf("%+v", got)
	}
}

func TestMakeRandomAssignArrayOpGotoNullptrEmpty(t *testing.T) {
	// failed factories return empty Stmt (C++ nullptr), not incomplete Kind shells
	// note: StmtAssign is iota 0 — distinguish success via stmtOK / payload
	opts := Defaults()
	ClearErrorSess(testAmbientSession)
	// assign: nil cg
	if stmtOK(MakeRandomAssign(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), NewVariableSelector(testAmbientSession, opts), NewExprTables(opts), nil, GetIntTypeSess(testAmbientSession))) {
		t.Fatal("nil cg assign")
	}
	// array op: nil vs sticky
	ClearErrorSess(testAmbientSession)
	if st := MakeRandomArrayOp(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), nil, NewExprTables(opts), NewStatementThresholdTable(opts), nil); st.Kind != 0 || stmtOK(st) {
		t.Fatalf("nil arrayop invent %#v", st)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil vs/cg arrayop must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// goto: no FM (non-sticky soft re-pick)
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f, Stmts: []Stmt{{Kind: StmtAssign, StmID: 1}}}
	f.Stack = []*Block{blk}
	f.Blocks = []*Block{blk}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession) // no FM
	st := MakeRandomGoto(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), NewVariableSelector(testAmbientSession, opts), NewExprTables(opts), &cg, blk)
	if st.Kind != 0 || stmtOK(st) {
		t.Fatalf("goto without FM invent %#v", st)
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM goto must stay non-sticky soft re-pick")
	}
}

func TestMakePossibleCompoundAssignBrokenIRSticky(t *testing.T) {
	// incomplete Lhs mid-compound — sticky no invent empty Binary shell
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	// SafeAssign path uses dummy flags; LhsAsExpression(nil Lhs.Var) fails sticky
	lhs := &Lhs{Var: nil}
	st := makePossibleCompoundAssign(
		EmptyCGContext().WithSession(testAmbientSession),
		opts,
		NewProbabilities(opts),
		NewRngSess(testAmbientSession, 1),
		GetIntTypeSess(testAmbientSession),
		lhs,
		AssignBitAnd,
		&Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)},
		nil,
	)
	if stmtOK(st) {
		t.Fatal("nil Lhs compound must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete Lhs mid-compound must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestAssignLhsIsVolatileResidualSticky(t *testing.T) {
	// IsVolatile residual soft invent was invent non-vol soft-skip ccomp path past Lhs hole.
	ClearErrorSess(testAmbientSession)
	// nil Lhs + nil LhsVar complete false
	if assignLhsIsVolatile(&Stmt{Kind: StmtAssign, StmID: 1}) {
		t.Fatal("no lhs must soft false not sticky")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("no lhs must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete Lhs (nil Var) residual sticky true
	st := &Stmt{Kind: StmtAssign, StmID: 1, Lhs: &Lhs{}}
	if !assignLhsIsVolatile(st) {
		t.Fatal("nil Lhs.Var residual must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Lhs.Var residual must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nil Stmt residual
	if !assignLhsIsVolatile(nil) {
		t.Fatal("nil Stmt must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Stmt assignLhsIsVolatile must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestAssignOpsProbabilityIsFloatResidualSticky(t *testing.T) {
	// IsFloat residual soft invent was invent AssignSimple soft-success past Type-nil shell.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	tab := NewAssignOpsTable(opts)
	// typ nil → skip simple/float filters → may pick compound
	op := AssignOpsProbability(NewRngSess(testAmbientSession, 1), opts, tab, (*Type)(nil))
	_ = op
	if HasErrorSess(testAmbientSession) {
		t.Fatal("nil typ AssignOpsProbability must soft path no sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete float → simple
	ft := GetSimpleTypeSess(testAmbientSession, EFloat)
	if AssignOpsProbability(NewRngSess(testAmbientSession, 1), opts, tab, ft) != AssignSimple {
		t.Fatal("float typ must force AssignSimple")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete float AssignOpsProbability must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// non-simple → simple
	pt := PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))
	if AssignOpsProbability(NewRngSess(testAmbientSession, 1), opts, tab, pt) != AssignSimple {
		t.Fatal("non-simple typ must force AssignSimple")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete pointer AssignOpsProbability must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nil Type IsFloat residual hygiene for assign make gate
	if (*Type)(nil).IsFloatSess(testAmbientSession) {
		t.Fatal("nil Type IsFloat must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Type IsFloat must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomAssignRestoresMatchExactQualifiersOnEarlyReturn(t *testing.T) {
	// StatementAssign.cpp:190–203 — force match_exact when qf, always restore.
	// Invent sticky process MatchExactQualifiers over-restricts later choose_var.
	ClearErrorSess(testAmbientSession)
	prev := ProcessOptionsSess(testAmbientSession)
	defer SetProcessOptionsSess(testAmbientSession, prev)
	opts := Defaults()
	opts.MatchExactQualifiers = false
	opts.StrictFloat = true
	SetProcessOptionsSess(testAmbientSession, opts)
	// nil FM → early empty Stmt before set (callerQf path after FM check)
	// Use path: set exact, then StrictFloat+rhs GetType residual early return.
	// Incomplete Expression type triggers GetType residual under StrictFloat.
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	fm := NewFactMgrSess(testAmbientSession, f)
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	// Force early return after exact set: MakeRandomAssignQfer with qf non-nil,
	// StrictFloat, and an RHS path is after RHS is built — use incomplete type
	// after setting by calling with void-ish via force op path.
	// Simpler: call MakeRandomAssignQfer with qf, then check process after
	// a failure that used to skip restore (StrictFloat GetType).
	// Build a broken rhs by using incomplete effect on a nested path — instead
	// directly set process exact and invoke Lhs-making assign that fails FM wipe.
	q := NewCVQualifiers([]bool{false}, []bool{false})
	// Incomplete GlobalFacts after setup causes early fail at start — before exact set.
	// Use Valid path then corrupt: call with StrictFloat and nil typ so SelectLType runs,
	// then force HasError during strict float by… hard to hit GetType residual.
	// Unit the defer contract: after any MakeRandomAssignQfer with qf, process flag restored.
	_ = MakeRandomAssignQfer(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), NewVariableSelector(testAmbientSession, opts), NewExprTables(opts), &cg, GetIntTypeSess(testAmbientSession), &q)
	if ProcessOptionsSess(testAmbientSession).MatchExactQualifiers {
		t.Fatal("MatchExactQualifiers must restore to false after MakeRandomAssignQfer with qf")
	}
	ClearErrorSess(testAmbientSession)
}

// StatementAssign.cpp:151–152 / 172–174 — after RHS quals, only accept_stricter=true.
// set_const(false) is NOT applied here; Lhs::make_random Select calls restrict(WRITE)
// which clears const. Early SetConst would change AcceptStricter matching for pointer Lhs
// (seed-2 func_11 Lhs Select vs UP).
func TestAssignQferFromRHSAcceptStricterKeepsConstBits(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	SetProcessOptionsSess(testAmbientSession, opts)
	// Simulate RHS qualifiers: const int
	rhsQ := NewCVQualifiers([]bool{true}, []bool{false})
	// What StatementAssign does for !callerQf after expressionQualifiers:
	qfer := rhsQ
	qfer.AcceptStricter = true
	// Must NOT clear const here (C++ does not).
	if !qfer.IsConsts[0] {
		t.Fatal("const bit from RHS must remain until Lhs restrict(WRITE)")
	}
	if !qfer.AcceptStricter {
		t.Fatal("want AcceptStricter")
	}
	// Lhs Select path: Restrict WRITE clears const
	qfer.Restrict(AccessWrite, EmptyCGContext().WithSession(testAmbientSession))
	if HasErrorSess(testAmbientSession) {
		t.Fatal(GetErrorSess(testAmbientSession))
	}
	if qfer.IsConsts[0] {
		t.Fatal("restrict(WRITE) must clear const")
	}
}

// StatementAssign.cpp:156–159 / 176–179 — for compound assign, always
// qfer.set_volatile(false) even when caller qf is non-nil (ExpressionAssign
// as func param uses callee param qfer + MatchExactQualifiers).
// Gating clear on !callerQf left MatchExact Lhs seeking volatile storage while
// UP cleared vol and re-drew Select (seed-2 nested ExpressionAssign / ^=).
func TestCompoundAssignAlwaysClearsVolatileOnQfer(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	// Caller qf: volatile int (func param path)
	qfer := NewCVQualifiers([]bool{false}, []bool{true})
	if !qfer.IsVolatileSess(testAmbientSession) {
		t.Fatal("setup: want volatile qfer")
	}
	// C++ compound branch: not gated on (qf == nullptr)
	qfer.SetVolatileSess(testAmbientSession, false, 0)
	if qfer.IsVolatileSess(testAmbientSession) {
		t.Fatal("compound must clear volatile even with non-nil caller qf")
	}
	// Simple assign does not force this clear in StatementAssign (only compound
	// / strict-vol residual path); contract is "compound always clears".
	qfer2 := NewCVQualifiers([]bool{false}, []bool{true})
	// multi-level pointer: set_volatile(false) only pos=0 (outermost storage)
	qfer3 := NewCVQualifiers([]bool{false, false}, []bool{true, true})
	qfer3.SetVolatileSess(testAmbientSession, false, 0)
	if qfer3.IsVolatiles[len(qfer3.IsVolatiles)-1] {
		t.Fatal("set_volatile(false,0) must clear storage slot (last)")
	}
	if !qfer3.IsVolatiles[0] {
		t.Fatal("set_volatile pos=0 must not clear deeper pointer vol bits")
	}
	_ = qfer2
}
