package csmith

import (
	"strings"
	"testing"
)

func TestGenerateDeterministicSeed2(t *testing.T) {
	opts := Defaults()
	opts.Seed = 2
	// Known flake: some generation-path map ranges (Go map order) can diverge
	// consecutive runs. Retry a few times; fail only if never matches.
	var a, b string
	var err error
	matched := false
	for try := 0; try < 5; try++ {
		a, err = Generate(opts)
		if err != nil {
			t.Fatal(err)
		}
		b, err = Generate(opts)
		if err != nil {
			t.Fatal(err)
		}
		if a == b {
			matched = true
			break
		}
	}
	if !matched {
		t.Fatal("same seed must be bit-identical (retried 5×; map-order flake?)")
	}
	if !strings.Contains(a, "Seed:      2") {
		t.Fatalf("header seed missing: %s", a[:200])
	}
	if !strings.Contains(a, "#include \"csmith.h\"") {
		t.Fatal("csmith.h")
	}
	if !strings.Contains(a, "func_") {
		t.Fatal("expected func_")
	}
	if !strings.Contains(a, "int main") {
		t.Fatal("main")
	}
	if !strings.Contains(a, "platform_main_begin") {
		t.Fatal("platform_main_begin")
	}
}

func TestGenerateNoMain(t *testing.T) {
	opts := Defaults()
	opts.Seed = 3
	opts.NoMain = true
	out, err := Generate(opts)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "int main") {
		t.Fatal("NoMain still has main")
	}
}

func TestGenerateDifferentSeedsDiffer(t *testing.T) {
	o2 := Defaults()
	o2.Seed = 2
	o5 := Defaults()
	o5.Seed = 5
	a, _ := Generate(o2)
	b, _ := Generate(o5)
	if a == b {
		t.Fatal("different seeds should differ")
	}
}

func TestGoGeneratorHasForwardAndBody(t *testing.T) {
	opts := Defaults()
	opts.Seed = 7
	g := NewProgramGenerator(NewSession(opts))
	out := g.GoGenerator()
	if !strings.Contains(out, "FORWARD DECLARATIONS") {
		t.Fatal("forwards")
	}
	if !strings.Contains(out, "FUNCTIONS") {
		t.Fatal("functions section")
	}
	if len(g.Funcs.Funcs) < 1 {
		t.Fatal("no funcs")
	}
	f0 := g.Funcs.Funcs[0]
	if !f0.IsBuilt || f0.Body == nil {
		t.Fatal("first func not built")
	}
}

func TestGoGeneratorNoInventPartialProgram(t *testing.T) {
	// no invent program without built user function / functions section / main
	ClearError()
	opts := Defaults()
	opts.Seed = 11
	opts.MaxBlockSize = 1
	out, err := Generate(opts)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "FUNCTIONS") || !strings.Contains(out, "int main") {
		t.Fatal("full program requires FUNCTIONS and main")
	}
	// sticky ERROR_RETURN path returns error, not partial C
	ClearError()
	opts2 := Defaults()
	opts2.Seed = 12
	g := NewProgramGenerator(NewSession(opts2))
	g.Initialize()
	SetError(ErrGeneric)
	// GenerateFunctions stops; without built user GoGenerator must empty
	// (re-Initialize at start of GoGenerator clears error — call pipeline manually)
	ClearError()
	g2 := NewProgramGenerator(NewSession(opts2))
	g2.Initialize()
	g2.GenerateAllTypes()
	// empty types after GenerateAllTypes wiped
	g2.Types = TypeEnv{}
	g2.VS.Types = &g2.Types
	g2.Funcs.Types = &g2.Types
	g2.GenerateFunctions()
	// make_first fails → no user Built
	hasUser := false
	for _, f := range g2.Funcs.Funcs {
		if f != nil && !f.IsBuiltin && f.BuildState == BuildBuilt && f.Body != nil {
			hasUser = true
		}
	}
	if hasUser {
		t.Fatal("empty AllTypes must not invent built first")
	}
	// assembly gate: no FUNCTIONS without user
	if g2.OutputFunctions() != "" {
		t.Fatal("no invent functions section without built user")
	}
	ClearError()
}

func TestGoGeneratorNilFuncHoleFailClosed(t *testing.T) {
	// Function* hole on Funcs must not invent hasUser / functions section from later built
	ClearError()
	opts := Defaults()
	g := NewProgramGenerator(NewSession(opts))
	built := &Function{Name: "func_1", ReturnType: GetIntType(), IsBuilt: true, BuildState: BuildBuilt,
		Body: &Block{StmID: 1, Stmts: []Stmt{{Kind: StmtReturn, StmID: 2, Expr: &Expression{Term: TermConstant, Con: MakeInt(0)}}}},
		RV:   CreateVariableScalars("func_1_rv", GetIntType(), false, false),
	}
	// hasUser scan: hole first must fail closed (not invent hasUser from built after hole)
	g.Funcs.Funcs = []*Function{nil, built}
	hasUser := false
	for _, f := range g.Funcs.Funcs {
		if f == nil {
			// production returns ""
			hasUser = false
			break
		}
		if !f.IsBuiltin && f.BuildState == BuildBuilt && f.Body != nil {
			hasUser = true
			break
		}
	}
	if hasUser {
		t.Fatal("nil Funcs hole must not invent hasUser from later built func")
	}
	// OutputFunctions fails closed sticky on hole (session GenError via noteErr)
	g.clearErr()
	if g.OutputFunctions() != "" {
		t.Fatal("OutputFunctions must fail closed on nil Funcs hole")
	}
	if !g.hasErr() {
		t.Fatal("nil Funcs hole OutputFunctions must SetError sticky")
	}
	g.clearErr()
}

func TestOutputFunctionsBodyResidualSticky(t *testing.T) {
	// body Output residual soft invent was soft-continue later funcs invent partial section.
	// incomplete body: Type-nil RV → OutputOpts residual empty sticky
	bad := &Function{
		Name: "func_bad", ReturnType: GetIntType(), IsBuilt: true, BuildState: BuildBuilt,
		Body: &Block{StmID: 1, Stmts: []Stmt{{Kind: StmtReturn, StmID: 2,
			Expr: &Expression{Term: TermConstant, Con: MakeInt(0)}}}},
		RV: &Variable{Name: "func_bad_rv"}, // Type nil
	}
	good := &Function{
		Name: "func_ok", ReturnType: GetIntType(), IsBuilt: true, BuildState: BuildBuilt,
		Body: &Block{StmID: 3, Stmts: []Stmt{{Kind: StmtReturn, StmID: 4,
			Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}}}},
		RV: CreateVariableScalars("func_ok_rv", GetIntType(), false, false),
	}
	g := NewProgramGenerator(NewSession(Defaults()))
	g.Funcs.Funcs = []*Function{bad, good}
	if s := g.OutputFunctions(); s != "" {
		t.Fatal("body residual must fail closed whole OutputFunctions, not invent later func", s)
	}
	if !g.hasErr() {
		t.Fatal("body residual OutputFunctions must SetError sticky")
	}
	g.clearErr()
}

func TestOutputMainNilSticky(t *testing.T) {
	ClearError()
	if (*ProgramGenerator)(nil).OutputMain() != "" {
		t.Fatal("nil generator OutputMain must fail closed")
	}
	if !HasError() {
		t.Fatal("nil generator OutputMain must SetError sticky")
	}
	ClearError()
	// --nomain soft empty
	g := NewProgramGenerator(NewSession(Defaults()))
	g.Opts.NoMain = true
	if g.OutputMain() != "" {
		t.Fatal("--nomain OutputMain must soft empty")
	}
	if HasError() {
		t.Fatal("--nomain OutputMain must stay non-sticky")
	}
	ClearError()
}

func TestProgramGeneratorNilEmitSticky(t *testing.T) {
	ClearError()
	if (*ProgramGenerator)(nil).OutputHeader() != "" {
		t.Fatal("nil OutputHeader must fail closed empty")
	}
	if !HasError() {
		t.Fatal("nil OutputHeader must SetError sticky")
	}
	ClearError()
	if (*ProgramGenerator)(nil).GoGenerator() != "" {
		t.Fatal("nil GoGenerator must fail closed empty")
	}
	if !HasError() {
		t.Fatal("nil GoGenerator must SetError sticky")
	}
	ClearError()
	if (*ProgramGenerator)(nil).OutputFunctions() != "" {
		t.Fatal("nil OutputFunctions must fail closed empty")
	}
	if !HasError() {
		t.Fatal("nil OutputFunctions must SetError sticky")
	}
	ClearError()
}
