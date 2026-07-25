package csmith

import (
	"strings"
	"testing"
)

func TestStr2Int(t *testing.T) {
	if Str2Int("42") != 42 {
		t.Fatal(Str2Int("42"))
	}
	if Str2Int("(7)") != 7 {
		t.Fatal(Str2Int("(7)"))
	}
	if Str2Int("0x10") != 16 {
		t.Fatal(Str2Int("0x10"))
	}
	// StringUtils.cpp:153 — assert close paren; no invent parse
	if Str2Int("(7") != -1 {
		t.Fatal("unbalanced paren must fail closed")
	}
	// stringstream >> int stops at L/UL (Constant small-path suffixes)
	// StringUtils.cpp:161–162 — seed-2 e15477 div/mod re-pick needs equals(0) on "0L"
	if Str2Int("0L") != 0 {
		t.Fatalf("0L want 0 got %d", Str2Int("0L"))
	}
	if Str2Int("0UL") != 0 {
		t.Fatalf("0UL want 0 got %d", Str2Int("0UL"))
	}
	if Str2Int("-1L") != -1 {
		t.Fatalf("-1L want -1 got %d", Str2Int("-1L"))
	}
	if Str2Int("1L") != 1 {
		t.Fatalf("1L want 1 got %d", Str2Int("1L"))
	}
	if Str2Int("0xAL") != 10 {
		t.Fatalf("0xAL want 10 got %d", Str2Int("0xAL"))
	}
	// stringstream >> int on overflow → numeric_limits::max (not -1).
	// 18446744073709551607UL is common as ~0ULL-8; Atoi-error→-1 made
	// FunctionInvocationBinary::equals(0) treat `x % BIGUL` as 0 (equals(-1) on RHS).
	big := "18446744073709551607UL"
	got := Str2Int(big)
	if got == -1 {
		t.Fatalf("overflow %s must not invent -1 (C++ clamps to MaxInt), got %d", big, got)
	}
	if got != int(^uint(0)>>1) { // math.MaxInt
		// On 64-bit host, ParseInt bitSize 0 still fits? 18446744073709551607 > MaxInt64
		// so ErrRange → MaxInt64 for signed parse... bitSize 0 uses int width.
		// int is 64-bit here: MaxInt = 2^63-1
		want := int(^uint(0) >> 1)
		if got != want {
			t.Fatalf("overflow %s: got %d want MaxInt %d", big, got, want)
		}
	}
	// FunctionInvocationBinary.cpp:171–173 — equals(-1) must not fire on BIGUL
	c := &Constant{Type: GetSimpleTypeSess(testAmbientSession, EULongLong), Value: big}
	if c.EqualsSess(testAmbientSession, -1) {
		t.Fatal("BIGUL must not Equals(-1) after overflow clamp")
	}
}

// TestModHugeConstNotEqualsZero — FunctionInvocationBinary.cpp:154–177 + 246–253.
// div/mod re-pick uses rhs.equals(0)||is_0_or_1(); mod with RHS=BIGUL must not
// fold to 0 via Str2Int overflow→-1 (seed 105 safe_div vs safe_add).
func TestModHugeConstNotEqualsZero(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	left := &Expression{Term: TermConstant, Con: &Constant{Type: GetIntTypeSess(testAmbientSession), Value: "1L"}, ExprType: GetIntTypeSess(testAmbientSession)}
	right := &Expression{Term: TermConstant, Con: &Constant{Type: GetSimpleTypeSess(testAmbientSession, EULongLong), Value: "18446744073709551607UL"}, ExprType: GetSimpleTypeSess(testAmbientSession, EULongLong)}
	fi := &Invocation{IsStd: true, Binary: "%", Args: []*Expression{left, right}}
	if fi.EqualsInt(0) {
		t.Fatal("1 % BIGUL must not EqualsInt(0) (would re-pick div/mod)")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("EqualsInt sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestChopEmptyEndWith(t *testing.T) {
	if Chop("  ab\t") != "ab" {
		t.Fatal(Chop("  ab\t"))
	}
	if !EmptyLine("  \t") || EmptyLine("x") {
		t.Fatal("empty")
	}
	if !EndWith("hello", "lo") {
		t.Fatal("end")
	}
}

func TestSplitIntString(t *testing.T) {
	got := SplitIntString("1, 2, 0xA", ",")
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 10 {
		t.Fatal(got)
	}
}

func TestBreakupAssigns(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	vs, vals := BreakupAssigns("a = 1; b=2;")
	if len(vs) != 2 || vs[0] != "a" || vals[0] != "1" || vs[1] != "b" || vals[1] != "2" {
		t.Fatal(vs, vals)
	}
	// StringUtils.cpp:222 assert(pair.size()==2); sticky no soft invent skip
	ClearErrorSess(testAmbientSession)
	vs, vals = BreakupAssigns("a=1; broken; c=3")
	if vs != nil || vals != nil {
		t.Fatal("malformed assign must fail whole parse")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("malformed BreakupAssigns must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestStr2Int64(t *testing.T) {
	if Str2Int64("0xFF") != 255 {
		t.Fatal(Str2Int64("0xFF"))
	}
	if Int642Str(99) != "99" {
		t.Fatal(Int642Str(99))
	}
}

func TestCollectAndOutputSkippedInits(t *testing.T) {
	outer := &Block{}
	loc := CreateVariableScalarsSess(testAmbientSession, "l_1", GetIntTypeSess(testAmbientSession), false, false)
	loc.Name = "l_1"
	loc.Init = MakeIntSess(testAmbientSession, 3)
	inner := &Block{Parent: outer, LocalVars: []*Variable{loc}}
	skipped := CollectInitSkippedVars(outer, inner)
	if skipped == nil || len(skipped) != 1 || skipped[0] != loc {
		t.Fatal(skipped)
	}
	st := &Stmt{Kind: StmtGoto, InitSkippedVars: skipped}
	out := OutputSkippedVarInits(st, "    ")
	if !strings.Contains(out, "l_1 = 3;") {
		t.Fatal(out)
	}
	// nil LocalVars hole fails closed sticky incomplete (not bare nil invent empty complete)
	ClearErrorSess(testAmbientSession)
	hole := &Block{Parent: outer, LocalVars: []*Variable{nil}}
	if VariablesComplete(CollectInitSkippedVars(outer, hole)) {
		t.Fatal("nil local hole must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil local hole CollectInitSkippedVars must SetError sticky")
	}
	if !HasInitSkippedVars(outer, hole) {
		t.Fatal("incomplete must fail closed as has-skipped")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSkippedInitsAtLabelNotEmitted(t *testing.T) {
	// Statement.cpp:911–913 — output_skipped_var_inits after label is commented out
	loc := CreateVariableScalarsSess(testAmbientSession, "l_2", GetIntTypeSess(testAmbientSession), false, false)
	loc.Name = "l_2"
	loc.Init = MakeIntSess(testAmbientSession, 9)
	b := &Block{Stmts: []Stmt{
		{Kind: StmtGoto, Label: "lbl_x", InitSkippedVars: []*Variable{loc},
			Expr: &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)}},
		{Kind: StmtAssign, SourceLabel: "lbl_x",
			LhsVar:   CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), true, false),
			AssignOp: AssignSimple,
			Expr:     &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0)}},
	}}
	out := b.Output(0)
	if !strings.Contains(out, "lbl_x:") {
		t.Fatal(out)
	}
	if strings.Contains(out, "l_2 = 9;") {
		t.Fatal("skipped inits must not emit after label (C++ pre_output commented)", out)
	}
}

func TestOutputSkippedVarInitsUsesInitExpr(t *testing.T) {
	// StatementGoto.cpp:271 — v->init->Output (InitExpr preferred)
	loc := CreateVariableScalarsSess(testAmbientSession, "l_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	loc.Name = "l_p"
	tgt := CreateVariableScalarsSess(testAmbientSession, "g_t", GetIntTypeSess(testAmbientSession), false, false)
	loc.InitExpr = &Expression{Term: TermVariable, Var: tgt, ExprType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))}
	// force address-like output path: variable expr of pointed type often emits name
	// set InitExpr to constant pointer-ish "0" via constant for stable assert
	loc.InitExpr = &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0), ExprType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))}
	st := &Stmt{Kind: StmtGoto, InitSkippedVars: []*Variable{loc}}
	out := OutputSkippedVarInits(st, "")
	if !strings.Contains(out, "l_p = 0;") {
		t.Fatal(out)
	}
	// InitExpr wins over Init
	loc.Init = MakeIntSess(testAmbientSession, 99)
	out2 := OutputSkippedVarInits(st, "")
	if strings.Contains(out2, "99") {
		t.Fatal("InitExpr should win", out2)
	}
}

func TestOutputSkippedVarInitsNoInventEmptyRHS(t *testing.T) {
	// StatementGoto.cpp:271 — assert(v->init); vars[i] always live
	// incomplete entry fails whole emit sticky (no invent skip holes / partial list)
	ClearErrorSess(testAmbientSession)
	v := CreateVariableWithInitSess(testAmbientSession, "l_miss", GetIntTypeSess(testAmbientSession), nil, NewCVQualifiers([]bool{false}, []bool{false}))
	v.Name = "l_miss"
	good := CreateVariableScalarsSess(testAmbientSession, "l_ok", GetIntTypeSess(testAmbientSession), false, false)
	good.Name = "l_ok"
	good.Init = MakeIntSess(testAmbientSession, 4)
	st := &Stmt{Kind: StmtGoto, InitSkippedVars: []*Variable{v, good}}
	out := OutputSkippedVarInits(st, "")
	if out != "" {
		t.Fatal("incomplete re-init list must fail closed whole emit", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("missing init re-init list must SetError sticky")
	}
	// nil hole fails closed sticky
	ClearErrorSess(testAmbientSession)
	stHole := &Stmt{Kind: StmtGoto, InitSkippedVars: []*Variable{good, nil}}
	if out := OutputSkippedVarInits(stHole, ""); out != "" {
		t.Fatal("nil InitSkippedVars hole must fail closed", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil InitSkippedVars hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// sticky no invent " = 5;" without identifier / partial good siblings
	anon := CreateVariableScalarsSess(testAmbientSession, "l_x", GetIntTypeSess(testAmbientSession), false, false)
	anon.Name = ""
	anon.Init = MakeIntSess(testAmbientSession, 5)
	st2 := &Stmt{Kind: StmtGoto, InitSkippedVars: []*Variable{anon, good}}
	out2 := OutputSkippedVarInits(st2, "")
	if out2 != "" {
		t.Fatal("empty name must fail closed whole emit", out2)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty name re-init must SetError sticky")
	}
	// complete list still emits
	ClearErrorSess(testAmbientSession)
	st3 := &Stmt{Kind: StmtGoto, InitSkippedVars: []*Variable{good}}
	if out3 := OutputSkippedVarInits(st3, ""); !strings.Contains(out3, "l_ok = 4;") {
		t.Fatal(out3)
	}
	ClearErrorSess(testAmbientSession)
}

func TestVariableInitOutput(t *testing.T) {
	// StatementGoto.cpp:271 — assert(v->init); sticky no soft invent "0" when missing
	ClearErrorSess(testAmbientSession)
	v := CreateVariableWithInitSess(testAmbientSession, "l_1", GetIntTypeSess(testAmbientSession), nil, NewCVQualifiers([]bool{false}, []bool{false}))
	if variableInitOutput(v) != "" {
		t.Fatal("nil init must not invent 0", variableInitOutput(v))
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil init variableInitOutput must SetError sticky")
	}
	// Variable.cpp:395 — CreateVariableScalars always Constant::make_random
	ClearErrorSess(testAmbientSession)
	v2 := CreateVariableScalarsSess(testAmbientSession, "l_2", GetIntTypeSess(testAmbientSession), false, false)
	if variableInitOutput(v2) == "" {
		t.Fatal("scalars path always has init")
	}
	v.Init = MakeIntSess(testAmbientSession, 5)
	if variableInitOutput(v) != "5" {
		t.Fatal(variableInitOutput(v))
	}
	v.InitExpr = &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 7)}
	if variableInitOutput(v) != "7" {
		t.Fatal(variableInitOutput(v))
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomGotoInitSkippedIncompleteFailClosed(t *testing.T) {
	// CollectInitSkippedVars nil (LocalVars hole) must not invent goto with empty skip list
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	// outer with hole in LocalVars; nested dest so Collect climbs past hole
	outer := &Block{Func: f, StmID: 1, LocalVars: []*Variable{nil}, Stmts: []Stmt{
		{Kind: StmtAssign, StmID: 10},
	}}
	inner := &Block{Func: f, Parent: outer, StmID: 2, Stmts: []Stmt{
		{Kind: StmtAssign, StmID: 20},
	}}
	f.Blocks = []*Block{outer, inner}
	f.Stack = []*Block{outer, inner}
	fm := NewFactMgrSess(testAmbientSession, f)
	// need read vars for cond
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	fm.GlobalFacts = []*FactPointTo{}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	eff := EmptyEffect().ReadVarSess(testAmbientSession, g)
	cg.EffectAccum = &eff
	// force map accum effect for forward path cond
	fm.MapAccumEffect = map[int]Effect{10: EmptyEffect().ReadVarSess(testAmbientSession, g)}
	// plant globals for ChooseVisibleReadVar
	vs := NewVariableSelector(testAmbientSession, opts)
	vs.GlobalList = []*Variable{g}
	// many seeds: if any succeeds with Kind Goto, fail
	for seed := uint64(1); seed < 40; seed++ {
		// reset blocks each time (forward path mutates)
		outer.Stmts = []Stmt{{Kind: StmtAssign, StmID: 10}}
		inner.Stmts = []Stmt{{Kind: StmtAssign, StmID: 20}}
		st := MakeRandomGoto(NewRngSess(testAmbientSession, seed), opts, NewProbabilities(opts), vs, NewExprTablesSess(testAmbientSession, opts), &cg, inner)
		if st.Kind == StmtGoto {
			// if Collect was incomplete, must have failed closed
			// hole on outer LocalVars when dest is outer and src is inner:
			// CollectInitSkippedVars(inner, outer) — dest outer, climb outer, LocalVars hole → nil
			// path depends on back vs forward
			if !VariablesComplete(st.InitSkippedVars) {
				// incomplete skip list must not be attached to a live goto
				t.Fatal("must not invent goto with incomplete InitSkippedVars", st.InitSkippedVars)
			}
		}
	}
	// Direct: Collect incomplete when hole
	if VariablesComplete(CollectInitSkippedVars(inner, outer)) {
		t.Fatal("outer LocalVars hole must yield incomplete Collect")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputSkippedVarInitsNilStmtSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if OutputSkippedVarInits(nil, "  ") != "" {
		t.Fatal("nil Stmt OutputSkippedVarInits must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Stmt OutputSkippedVarInits must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIgnoreSpacesFindAnyGetSubstringBefore(t *testing.T) {
	// StringUtils.cpp ignore_spaces / find_any_char / get_substring_before
	if IgnoreSpaces("  \tabc", 0) != 3 {
		t.Fatalf("ignore_spaces: %d", IgnoreSpaces("  \tabc", 0))
	}
	if FindAnyChar("hello,world", 0, ",;") != 5 {
		t.Fatalf("find_any: %d", FindAnyChar("hello,world", 0, ",;"))
	}
	if FindAnyChar("hello", 0, ",;") != -1 {
		t.Fatal("npos")
	}
	if GetSubstringBefore("UInt,UChar)", 0, ')') != "UInt,UChar" {
		t.Fatal(GetSubstringBefore("UInt,UChar)", 0, ')'))
	}
	if GetSubstringBefore("abc", 0, ')') != "" {
		t.Fatal("missing close")
	}
}
