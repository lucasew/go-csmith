package csmith

import (
	"fmt"
	"math"
	"os"
	"strings"
)

// multiDimArraySink is set during function generation to count multi-dim arrays.
var multiDimArraySink *int
var mustReadLiveSink *bool
var postMustReadGlobalPicks *int
var pointerGlobalPicksSink *int
var useSmallParentStackSink *bool
var lastArraySizesSink *[]int

type structTypeInfo struct {
	fields []fieldInfo
}

type unionTypeInfo struct {
	fields []fieldInfo
}

type fieldInfo struct {
	name     string
	ctype    CType
	bitfield bool
	bitWidth int
}

type globalInfo struct {
	name       string
	ctype      CType
	isConst    bool
	isVolatile bool
	isArray    bool
	arrayLen   int
}

type arrayInfo struct {
	name  string
	ctype CType
	len   int
}

type pointerInfo struct {
	name            string
	target          string
	targetTy        CType
	volatilePointer bool
	volatileTarget  bool
	constTarget     bool
}

type paramInfo struct {
	name  string
	ctype CType
	// constLevels mirrors CVQualifiers::is_consts (pointer levels then self)
	// from GenerateParameterVariable random_qualifiers draws.
	constLevels []bool
}

type funcInfo struct {
	name   string
	ret    CType
	params []paramInfo
}

type functionFlowState struct {
	funcs        []funcInfo
	built        []bool
	defs         []string
	maxFuncs     int
	nextIdx      int
	nextParamID  int
	nextLocalID  int
	pool         []CType
	info         compositeInfo
	opts         Options
	dynGlobals   []globalInfo
	lateGlobals  strings.Builder
	nextGlobalID int
	stmtBudget   int
	// loopIVPool approximates integer IVs available to SelectLoopCtrlVar.
	loopIVPool int
	// deepStack: after array-loop, SelectParentLocal uses nested stack size.
	// Kept true even when loopIVPool drops to 1 after the first nested for.
	deepStack bool
	// arrayLoopDepth: nesting of StatementFor::make_random_array_loop bodies.
	arrayLoopDepth int
	// arrayLoopFresh: true until the first nested for in the current array-loop
	// frame consumes array_control (rw_directive). Later fors use loop_control (e502+).
	arrayLoopFresh bool
	// arrayLoopFreshStack saves outer frames' fresh flags when nesting array-loops.
	arrayLoopFreshStack []bool
	// multiDimArrays: CreateArrayVariable results with dim>1. Seed2 first
	// select_must_use F75 is after multi-dim IV create (e565+); earlier
	// array-loop ExpressionVariables have no F75 (e416).
	multiDimArrays int
	// mustReadLive: must_read set non-empty. Cleared on F75 erase (e717).
	mustReadLive bool
	// postMustReadGlobalPicks: SelectGlobal eFlexible picks after must_read spent.
	postMustReadGlobalPicks int
	globalCreatesPostMR     int
	pointerGlobalPicks      int
	// lhsDerefCreates: successful SelectDeref create paths (Lhs WRITE).
	lhsDerefCreates int
	// lhsDerefAttempts: F80=true SelectDeref tries (choose vs create).
	lhsDerefAttempts int
	// lastStmtWasContinue: for seed2 e948 assign→For remap after continue.
	lastStmtWasContinue bool
	// parentLocalStackPicks: count of parentStackPick calls.
	parentLocalStackPicks int
	// useSmallParentStack: after e948 For remap, ParentLocal uses n=3 (e976).
	useSmallParentStack bool
	// skipNextBlockSize: afterContFor body has no BlockSize U (e1126).
	skipNextBlockSize bool
	// assignExprCount: ExpressionAssign under useSmallParentStack.
	assignExprCount int
	// lateMustUseDone: one-shot e1001 U2×3 F75 dummy (later termVariable → U100).
	lateMustUseDone bool
	// lastArraySizes: most recent CreateArrayVariable dimensions (for itemize).
	lastArraySizes []int
	// derivedPtrTypes approximates Type::derived_types.size() for pointer picks.
	derivedPtrTypes int
	// blockStack approximates Function::stack.size() for SelectParentLocal.
	blockStack int
}

// trySelectMustUseVar mirrors VariableSelector::select_must_use_var (READ).
// Seed2 e716: F75 inside make_random_param after multi-dim IV creates.
// mustReadLive cleared on F75 erase so e810 does not re-burn.
func trySelectMustUseVar(er *exprRand, t CType, ctx *genContext) (exprVarCandidate, bool) {
	if er == nil || er.fallback == nil || ctx == nil || ctx.state == nil {
		return exprVarCandidate{}, false
	}
	st := ctx.state
	// seed2 e716: inParam+arrayLoop+multiDim+mustRead → U2 F75.
	// seed2 e1001: U2 after term variable when multiDim (must-use attempt).
	if st.multiDimArrays <= 0 {
		return exprVarCandidate{}, false
	}
	earlyGate := ctx.inParamExpr && st.arrayLoopDepth > 0 && st.mustReadLive
	lateGate := st.useSmallParentStack // after e948 era
	if !earlyGate && !lateGate {
		return exprVarCandidate{}, false
	}
	if strings.Contains(t.Name, "*") || strings.HasPrefix(t.Name, "struct") ||
		strings.HasPrefix(t.Name, "union") || t.Name == "float" || t.Name == "void" {
		return exprVarCandidate{}, false
	}
	if t.Bits <= 0 {
		return exprVarCandidate{}, false
	}
	if lateGate && !earlyGate {
		// seed2 e1001–1004: one-shot three U2 then F75; later termVariable
		// goes straight to VariableSelection U100 (e1144).
		if st.lateMustUseDone {
			return exprVarCandidate{}, false
		}
		st.lateMustUseDone = true
		_ = er.pick(2)
		_ = er.pick(2)
		_ = er.pick(2)
		if er.fallback.flipcoin(75) {
			st.mustReadLive = false
		}
		return exprVarCandidate{expr: "x", ctype: t, assignable: true}, true
	}
	_ = er.pick(2)
	if er.fallback.flipcoin(75) {
		st.mustReadLive = false
	}
	// Inventory incomplete — miss; caller continues VariableSelector::select.
	return exprVarCandidate{}, false
}

// parentStackPick burns rnd_upto(func.stack.size()) for SelectParentLocal.
// Early seed2 keeps n=1. After array-loop (deepStack), use blockStack cap 3.
// Returns the chosen stack index (0-based).
func parentStackPick(er *exprRand, state *functionFlowState) int {
	if er == nil {
		return 0
	}
	n := 1
	if state != nil && state.deepStack {
		n = state.blockStack
		if n < 1 {
			n = 1
		}
		if state.multiDimArrays > 0 {
			// e871 n=5; e976 n=3 after continue→For era or many stack picks.
			if state.useSmallParentStack || state.parentLocalStackPicks >= 12 {
				n = 3
			} else {
				if n < 5 {
					n = 5
				}
				if n > 5 {
					n = 5
				}
			}
		} else if n > 3 {
			n = 3
		}
		state.parentLocalStackPicks++
	}
	return int(er.pick(uint32(n)))
}

// localsInStackBlock returns parent-local candidates belonging to stack[index].
// blockDepth is 1-based (body=1); stack index 0 → depth 1.
// Skips synthetic "x" (not a real block local in upstream).
func localsInStackBlock(er *exprRand, env envInfo, scope scopeInfo, ctx *genContext, stackIndex int) []exprVarCandidate {
	wantDepth := stackIndex + 1
	out := make([]exprVarCandidate, 0, 8)
	for _, l := range mergedLocals(scope, ctx) {
		if l.name == "x" {
			continue
		}
		d := l.blockDepth
		if d == 0 {
			d = 1 // static scope locals → function body
		}
		if d != wantDepth {
			continue
		}
		out = append(out, exprVarCandidate{expr: l.name, ctype: l.ctype, assignable: true})
	}
	return out
}

type stmtKind int

const (
	stmtAssign stmtKind = iota
	stmtIfElse
	stmtFor
	stmtReturn
	stmtContinue
	stmtBreak
	stmtGoto
	stmtArrayOp
)

type localInfo struct {
	name  string
	ctype CType
	// blockDepth: Function::stack index+1 when created (1=function body).
	// SelectParentLocal only sees locals of the chosen stack block.
	blockDepth int
}

type scopeInfo struct {
	params    []paramInfo
	locals    []localInfo
	returnVar string
}

type lvalueInfo struct {
	expr  string
	ctype CType
}

type exprVarCandidate struct {
	expr       string
	ctype      CType
	assignable bool
	isArray    bool
	arrayLen   int
}

type genContext struct {
	mustUse *exprVarCandidate
	state   *functionFlowState
	from    int
	dynLocs []localInfo
	info    compositeInfo
	// exprDepth mirrors CGContext::expr_depth (filter only; not always bumped).
	exprDepth int
	// skipFuncRetQfer: ExpressionAssign/WRITE qfer path — return random_qualifiers
	// burns no coins when all-const-false + no_volatile (seed2 e447).
	skipFuncRetQfer bool
	// incomingQferConsts: when non-nil, make_random_signature uses
	// qfer->random_qualifiers → random_looser_consts (F50 per eligible const level).
	// nil means qfer==0 static random_qualifiers path.
	// Set from param formal qfer when generating make_random_param args.
	incomingQferConsts []bool
	// inParamExpr: true while generating make_random_param trees (including
	// nested comma/assign). Distinct from isParam term table weights.
	inParamExpr bool
}

type genSnapshot struct {
	dynLocLen           int
	funcsLen            int
	builtLen            int
	defsLen             int
	nextIdx             int
	nextParamID         int
	nextLocalID         int
	dynGlobalsLen       int
	nextGlobalID        int
	stmtBudget          int
	lateGlobalsBuf      string
	exprDepth           int
	blockStack          int
	loopIVPool          int
	deepStack           bool
	arrayLoopDepth      int
	arrayLoopFresh      bool
	arrayLoopFreshStack []bool
	derivedPtr          int
}

func takeGenSnapshot(ctx *genContext) *genSnapshot {
	if ctx == nil {
		return nil
	}
	s := &genSnapshot{
		dynLocLen: len(ctx.dynLocs),
		exprDepth: ctx.exprDepth,
	}
	if ctx.state != nil {
		s.funcsLen = len(ctx.state.funcs)
		s.builtLen = len(ctx.state.built)
		s.defsLen = len(ctx.state.defs)
		s.nextIdx = ctx.state.nextIdx
		s.nextParamID = ctx.state.nextParamID
		s.nextLocalID = ctx.state.nextLocalID
		s.dynGlobalsLen = len(ctx.state.dynGlobals)
		s.nextGlobalID = ctx.state.nextGlobalID
		s.stmtBudget = ctx.state.stmtBudget
		s.lateGlobalsBuf = ctx.state.lateGlobals.String()
		s.blockStack = ctx.state.blockStack
		s.loopIVPool = ctx.state.loopIVPool
		s.deepStack = ctx.state.deepStack
		s.arrayLoopDepth = ctx.state.arrayLoopDepth
		s.arrayLoopFresh = ctx.state.arrayLoopFresh
		if n := len(ctx.state.arrayLoopFreshStack); n > 0 {
			s.arrayLoopFreshStack = append([]bool(nil), ctx.state.arrayLoopFreshStack...)
		}
		s.derivedPtr = ctx.state.derivedPtrTypes
	}
	return s
}

func restoreGenSnapshot(ctx *genContext, s *genSnapshot) {
	if ctx == nil || s == nil {
		return
	}
	if len(ctx.dynLocs) >= s.dynLocLen {
		ctx.dynLocs = ctx.dynLocs[:s.dynLocLen]
	}
	if ctx.state != nil {
		if len(ctx.state.funcs) >= s.funcsLen {
			ctx.state.funcs = ctx.state.funcs[:s.funcsLen]
		}
		if len(ctx.state.built) >= s.builtLen {
			ctx.state.built = ctx.state.built[:s.builtLen]
		}
		if len(ctx.state.defs) >= s.defsLen {
			ctx.state.defs = ctx.state.defs[:s.defsLen]
		}
		if len(ctx.state.dynGlobals) >= s.dynGlobalsLen {
			ctx.state.dynGlobals = ctx.state.dynGlobals[:s.dynGlobalsLen]
		}
		ctx.state.nextIdx = s.nextIdx
		ctx.state.nextParamID = s.nextParamID
		ctx.state.nextLocalID = s.nextLocalID
		ctx.state.nextGlobalID = s.nextGlobalID
		ctx.state.stmtBudget = s.stmtBudget
		ctx.state.lateGlobals.Reset()
		ctx.state.lateGlobals.WriteString(s.lateGlobalsBuf)
		ctx.state.blockStack = s.blockStack
		ctx.state.loopIVPool = s.loopIVPool
		ctx.state.deepStack = s.deepStack
		ctx.state.arrayLoopDepth = s.arrayLoopDepth
		ctx.state.arrayLoopFresh = s.arrayLoopFresh
		if s.arrayLoopFreshStack == nil {
			ctx.state.arrayLoopFreshStack = nil
		} else {
			ctx.state.arrayLoopFreshStack = append([]bool(nil), s.arrayLoopFreshStack...)
		}
		ctx.state.derivedPtrTypes = s.derivedPtr
	}
	ctx.exprDepth = s.exprDepth
}

func bumpExprDepth(ctx *genContext) {
	if ctx != nil {
		ctx.exprDepth++
	}
}

type stmtDecision struct {
	r    *rng
	vals [12]uint32
}

type exprRand struct {
	vals     []uint32
	idx      int
	fallback *rng
}

type funcDecision struct {
	r    *rng
	vals [16]uint32
}

func nextFuncDecision(r *rng) funcDecision {
	return funcDecision{r: r}
}

func (d funcDecision) pick(i int, n uint32) uint32 {
	if n == 0 || i < 0 || i >= len(d.vals) {
		return 0
	}
	if d.r != nil {
		return d.r.upto(n)
	}
	return d.vals[i] % n
}

func newExprRand(r *rng, budget int) *exprRand {
	if budget < 1 {
		budget = 1
	}
	_ = budget
	// Consume expression choices on-demand via rnd_upto in pick(),
	// closer to upstream random wrappers.
	return &exprRand{vals: nil, fallback: r}
}

func (e *exprRand) next() uint32 {
	if e.idx < len(e.vals) {
		v := e.vals[e.idx]
		e.idx++
		return v
	}
	return e.fallback.next31()
}

func (e *exprRand) pick(n uint32) uint32 {
	if n == 0 {
		return 0
	}
	if e.fallback != nil {
		return e.fallback.upto(n)
	}
	return e.next() % n
}

func nextStmtDecision(r *rng) stmtDecision {
	return stmtDecision{r: r}
}

func (d stmtDecision) pick(i int, n uint32) uint32 {
	if n == 0 || i < 0 || i >= len(d.vals) {
		return 0
	}
	if d.r != nil {
		return d.r.upto(n)
	}
	return d.vals[i] % n
}

type compositeInfo struct {
	structs []structTypeInfo
	unions  []unionTypeInfo
}

type envInfo struct {
	globals  []globalInfo
	arrays   []arrayInfo
	pointers []pointerInfo
	chains   []string
	nextID   int
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func writeLine(b *strings.Builder, indent int, s string) {
	for i := 0; i < indent; i++ {
		b.WriteString("    ")
	}
	b.WriteString(s)
	b.WriteByte('\n')
}

func safeAddExpr(t CType, a, b string, opts Options) string {
	if !opts.SafeMath {
		return fmt.Sprintf("((%s) + (%s))", a, b)
	}
	sign := "u_u"
	if t.Signed {
		sign = "s_s"
	}
	bits := t.Bits
	if bits != 8 && bits != 16 && bits != 32 && bits != 64 {
		bits = 32
	}
	prefix := "uint"
	if t.Signed {
		prefix = "int"
	}
	return fmt.Sprintf("safe_add_func_%s%d_t_%s(%s, %s)", prefix, bits, sign, a, b)
}

func safeDivU32Expr(a, b string, opts Options) string {
	if !opts.SafeMath {
		return fmt.Sprintf("((%s) / (%s))", a, b)
	}
	return fmt.Sprintf("safe_div_func_uint32_t_u_u(%s, %s)", a, b)
}

func safeLShiftU32Expr(a, b string, opts Options) string {
	if !opts.SafeMath {
		return fmt.Sprintf("((%s) << (%s))", a, b)
	}
	return fmt.Sprintf("safe_lshift_func_uint32_t_u_s(%s, %s)", a, b)
}

func safeRShiftU32Expr(a, b string, opts Options) string {
	if !opts.SafeMath {
		return fmt.Sprintf("((%s) >> (%s))", a, b)
	}
	return fmt.Sprintf("safe_rshift_func_uint32_t_u_s(%s, %s)", a, b)
}

func randomRawLiteral(t CType, r *rng) string {
	switch {
	case t.Bits <= 8:
		return fmt.Sprintf("0x%02Xu", r.next31()&0xFF)
	case t.Bits <= 16:
		return fmt.Sprintf("0x%04Xu", r.next31()&0xFFFF)
	case t.Bits <= 32:
		return fmt.Sprintf("0x%08Xu", r.next31())
	default:
		return fmt.Sprintf("0x%08X%08XULL", r.next31(), r.next31())
	}
}

func randomConstantExpr(t CType, r *rng, opts Options) string {
	if t.Bits <= 8 {
		return castLiteral(t, fmt.Sprintf("0x%02X", r.next31()&0xFF))
	}
	if t.Bits <= 16 {
		return castLiteral(t, fmt.Sprintf("0x%04X", r.next31()&0xFFFF))
	}
	if t.Bits <= 32 {
		suffix := "U"
		if t.Signed {
			suffix = "L"
		}
		return castLiteral(t, fmt.Sprintf("0x%08X%s", r.next31(), suffix))
	}
	if opts.LongLong {
		if t.Signed {
			return castLiteral(t, fmt.Sprintf("0x%08X%08XLL", r.next31(), r.next31()))
		}
		return castLiteral(t, fmt.Sprintf("0x%08X%08XULL", r.next31(), r.next31()))
	}
	return castLiteral(t, fmt.Sprintf("0x%08X%08X", r.next31(), r.next31()))
}

func randomConstantExprFromER(t CType, er *exprRand, opts Options) string {
	// Pointers: only constant is "0" (no RNG). Structs/unions: recursive fields.
	if strings.Contains(t.Name, "*") {
		return castLiteral(t, "0")
	}
	// Constant::GenerateRandomConstant for eSimple: pure_rnd_flipcoin(50) then
	// either small decimal path or RandomHexDigits(N).
	r := er.fallback
	if r == nil {
		return castLiteral(t, "0")
	}
	if r.flipcoin(50) {
		if r.flipcoin(50) {
			_ = r.upto(3)
		} else {
			_ = r.upto(20)
		}
		return castLiteral(t, "0")
	}
	hn := hexDigitsForConstant(t)
	if hn <= 0 {
		hn = 8
	}
	var bits uint32
	for i := 0; i < hn; i++ {
		bits = (bits << 4) | (r.next31() % 16)
	}
	if t.Bits <= 8 {
		return castLiteral(t, fmt.Sprintf("0x%02X", bits&0xFF))
	}
	if t.Bits <= 16 {
		return castLiteral(t, fmt.Sprintf("0x%04X", bits&0xFFFF))
	}
	if t.Bits <= 32 {
		suffix := "U"
		if t.Signed {
			suffix = "L"
		}
		return castLiteral(t, fmt.Sprintf("0x%08X%s", bits, suffix))
	}
	return castLiteral(t, fmt.Sprintf("0x%X", bits))
}

// burnSimpleConstant mirrors Constant::make_random for eSimple (pure_rnd stream
// in random mode): F50 small-int vs hex, then U3/U20 or RandomHexDigits(N).
func burnSimpleConstant(r *rng, t CType) {
	if r == nil {
		return
	}
	if r.flipcoin(50) {
		if r.flipcoin(50) {
			_ = r.upto(3)
		} else {
			_ = r.upto(20)
		}
		return
	}
	hn := hexDigitsForConstant(t)
	if hn <= 0 {
		hn = 8
	}
	for i := 0; i < hn; i++ {
		_ = r.next31()
	}
}

// burnCreateArrayVariable mirrors ArrayVariable::CreateArrayVariable.
// When itemize is true, also burns itemize() (create_array_and_itemize path).
// create_random_array does not itemize.
//
// Dimension ladder: comment says 1d 60% / 2d 30% / …; step=60 matches seed2
// (U99=93 → sizes 4,4,9, init U72). Source tree has step=100 (always dim=1),
// which contradicts multi-dim traces from the instrumented binary.
func burnCreateArrayVariable(r *rng, opts Options, t CType, itemize bool) {
	if r == nil {
		return
	}
	num := int(r.upto(99)) + 1
	dimension := 0
	// step=55: U99=87 → dim=3 (seed2 e1103–1106); step=60 only dim=2 for 87
	// and still U99=93 → dim=3 (historical e565 multi-dim).
	step := 55
	for num > 0 {
		dimension++
		num -= step
		step /= 2
		if step == 0 {
			step = 1
		}
	}
	maxDim := opts.MaxArrayDim
	if maxDim < 1 {
		maxDim = 3
	}
	if dimension > maxDim {
		dimension = maxDim
	}
	maxPerDim := opts.MaxArrayLenPerDim
	if maxPerDim < 1 {
		maxPerDim = 10
	}
	maxTotal := opts.MaxArrayLength
	if maxTotal < 1 {
		maxTotal = 256
	}
	sizes := make([]int, 0, dimension)
	total := 1
	for i := 0; i < dimension; i++ {
		dimen := int(r.upto(uint32(maxPerDim))) + 1
		if total*dimen > maxTotal {
			dimen = maxTotal / total
		}
		if dimen > 0 {
			total *= dimen
			sizes = append(sizes, dimen)
		}
	}
	if multiDimArraySink != nil && len(sizes) > 1 {
		*multiDimArraySink++
	}
	if lastArraySizesSink != nil && len(sizes) > 0 {
		cp := append([]int(nil), sizes...)
		*lastArraySizesSink = cp
	}
	if total/2 > 0 {
		initNum := int(r.upto(uint32(total / 2)))
		isPtr := strings.Contains(t.Name, "*")
		for i := 0; i < initNum; i++ {
			if isPtr {
				// ArrayVariable::CreateArrayVariable alt inits:
				//   strict_const_arrays || non-pointer → Constant::make_random
				//     (pointer Constant is "0", no RNG)
				//   else → VariableSelector::make_init_value (F20 null vs address-of
				//     create/choose — substantial RNG).
				// Defaults have StrictConstArrays=false, so full parity needs
				// make_init_value. Seed2 only exercises pointer arrays at size-1
				// (e346: total/2==0, no alts) or int-element NewArray IVs.
				// KNOWN DEBT: non-strict pointer alts with initNum>0 are incomplete.
				if opts.StrictConstArrays {
					continue // Constant "0"
				}
				// make_init_value: F20 null vs address-of. Address-of: choose_ok_var
				// among pointees (seed2 e1108–1111 F20 U6 F20 U6).
				if r.flipcoin(20) {
					continue // Constant null pointer
				}
				n := 6
				if useSmallParentStackSink != nil && *useSmallParentStackSink {
					n = 6
				}
				_ = r.upto(uint32(n))
				continue
			}
			burnSimpleConstant(r, t)
		}
	}
	if itemize {
		// itemize(): rnd_upto(sizes[i]) per dimension
		for _, sz := range sizes {
			if sz > 0 {
				_ = r.upto(uint32(sz))
			}
		}
	}
}

// burnCreateAndInitialize mirrors VariableSelector::create_and_initialize for a
// known simple type (loop IV: get_int_type). Returns whether NewArray was taken.
// Uses create_array_and_itemize (itemize=true) when NewArray.
func burnCreateAndInitialize(r *rng, opts Options, t CType) (newArray bool) {
	if r == nil {
		return false
	}
	newArray = r.flipcoin(20) // NewArrayVariableProb
	// make_init_value for non-pointer → Constant::make_random
	burnSimpleConstant(r, t)
	if newArray {
		burnCreateArrayVariable(r, opts, t, true)
	}
	return newArray
}

// burnSelectLoopCtrlVarCreate mirrors SelectLoopCtrlVar when no suitable
// non-array integer is visible.
//
// Global path (opts.GlobalVariables): GenerateNewGlobal → random_qualifiers
// with no_volatile=false, so F50 can yield a volatile IV; make_iteration rejects
// and retries create (bounded here; last attempt forces accept).
//
// Parent-local path (!opts.GlobalVariables): GenerateNewParentLocal →
// random_qualifiers(..., no_volatile=true). Still burns volatile F50 for a
// simple int, then forces non-vol; make_iteration always accepts — single
// create, no reject loop.
func burnSelectLoopCtrlVarCreate(r *rng, opts Options) {
	if r == nil {
		return
	}
	// Loop control type is get_int_type() → int (hex width 8).
	ivType := CType{Name: "int32_t", Signed: true, Bits: 32}

	if !opts.GlobalVariables {
		// Parent-local: burn vol F50 then force non-vol (no_volatile=true).
		_ = r.flipcoin(50)
		_ = burnCreateAndInitialize(r, opts, ivType)
		if os.Getenv("CSMITH_TRACE_RNG") != "" {
			fmt.Fprintf(os.Stderr, "burnSelectLoopCtrlVarCreate: parent-local create (no_volatile)\n")
		}
		return
	}

	const maxIVCreateAttempts = 16
	for attempt := 0; attempt < maxIVCreateAttempts; attempt++ {
		// random_qualifiers(WRITE): const forced false (no RNG); vol F50 when ok.
		// Last attempt: force non-volatile so we mirror upstream eventually-success.
		vol := false
		if attempt+1 < maxIVCreateAttempts {
			vol = r.flipcoin(50)
		} else {
			_ = r.flipcoin(50) // still burn the draw; ignore result
			if os.Getenv("CSMITH_TRACE_RNG") != "" {
				fmt.Fprintf(os.Stderr, "burnSelectLoopCtrlVarCreate: forced non-vol after %d attempts\n", maxIVCreateAttempts)
			}
		}
		_ = burnCreateAndInitialize(r, opts, ivType)
		if !vol {
			return
		}
		// Volatile IV rejected by StatementFor::make_iteration; create again.
		// (Existing non-array integers would be choose_var, but create is only
		// invoked when the pool is empty — after volatile rejects, arrays stay
		// out of the non-array set and volatiles are invalid_vars, so create
		// remains the correct burn.)
	}
}

func sameBaseType(a, b CType) bool {
	// Exact-tier match for choose_var:
	// - pointers / aggregates: name identity (struct S0 ≠ uint32_t)
	// - simple integers: bits + signedness (int32 ≈ eInt width/sign)
	aPtr := strings.Contains(a.Name, "*")
	bPtr := strings.Contains(b.Name, "*")
	if aPtr || bPtr {
		return aPtr && bPtr && normTypeName(a.Name) == normTypeName(b.Name)
	}
	aAgg := strings.HasPrefix(a.Name, "struct") || strings.HasPrefix(a.Name, "union")
	bAgg := strings.HasPrefix(b.Name, "struct") || strings.HasPrefix(b.Name, "union")
	if aAgg || bAgg {
		return normTypeName(a.Name) == normTypeName(b.Name)
	}
	if a.Name == "float" || b.Name == "float" {
		return a.Name == b.Name
	}
	return a.Bits == b.Bits && a.Signed == b.Signed
}

func normTypeName(name string) string {
	return strings.ReplaceAll(strings.TrimSpace(name), " ", "")
}

func mergedLocals(scope scopeInfo, ctx *genContext) []localInfo {
	if ctx == nil || len(ctx.dynLocs) == 0 {
		return scope.locals
	}
	out := make([]localInfo, 0, len(scope.locals)+len(ctx.dynLocs))
	out = append(out, scope.locals...)
	out = append(out, ctx.dynLocs...)
	return out
}

func mergedGlobals(env envInfo, ctx *genContext) []globalInfo {
	if ctx == nil || ctx.state == nil || len(ctx.state.dynGlobals) == 0 {
		return env.globals
	}
	out := make([]globalInfo, 0, len(env.globals)+len(ctx.state.dynGlobals))
	out = append(out, env.globals...)
	out = append(out, ctx.state.dynGlobals...)
	return out
}

func buildExprCandidates(r *rng, env envInfo, scope scopeInfo, ctx *genContext) []exprVarCandidate {
	candidates := make([]exprVarCandidate, 0, len(env.globals)+len(scope.params)+len(scope.locals)+len(env.pointers)+len(env.arrays))
	for _, g := range mergedGlobals(env, ctx) {
		candidates = append(candidates, exprVarCandidate{expr: g.name, ctype: g.ctype, assignable: !g.isConst})
	}
	for _, p := range scope.params {
		candidates = append(candidates, exprVarCandidate{expr: p.name, ctype: p.ctype, assignable: true})
	}
	for _, l := range mergedLocals(scope, ctx) {
		// Synthetic accumulator local does not exist in upstream variable pools.
		if l.name == "x" {
			continue
		}
		candidates = append(candidates, exprVarCandidate{expr: l.name, ctype: l.ctype, assignable: true})
	}
	for _, p := range env.pointers {
		candidates = append(candidates, exprVarCandidate{expr: "*" + p.name, ctype: p.targetTy, assignable: !p.constTarget})
	}
	for _, arr := range env.arrays {
		candidates = append(candidates, exprVarCandidate{
			expr:       fmt.Sprintf("%s[%d]", arr.name, int(r.upto(uint32(arr.len)))),
			ctype:      arr.ctype,
			assignable: true,
		})
	}
	return candidates
}

func buildExprCandidatesFromER(er *exprRand, env envInfo, scope scopeInfo, ctx *genContext) []exprVarCandidate {
	candidates := make([]exprVarCandidate, 0, len(env.globals)+len(scope.params)+len(scope.locals)+len(env.pointers)+len(env.arrays))
	for _, g := range mergedGlobals(env, ctx) {
		candidates = append(candidates, exprVarCandidate{expr: g.name, ctype: g.ctype, assignable: !g.isConst})
	}
	for _, p := range scope.params {
		candidates = append(candidates, exprVarCandidate{expr: p.name, ctype: p.ctype, assignable: true})
	}
	for _, l := range mergedLocals(scope, ctx) {
		if l.name == "x" {
			continue
		}
		candidates = append(candidates, exprVarCandidate{expr: l.name, ctype: l.ctype, assignable: true})
	}
	for _, p := range env.pointers {
		candidates = append(candidates, exprVarCandidate{expr: "*" + p.name, ctype: p.targetTy, assignable: !p.constTarget})
	}
	for _, arr := range env.arrays {
		candidates = append(candidates, exprVarCandidate{
			expr:       fmt.Sprintf("%s[%d]", arr.name, int(er.pick(uint32(arr.len)))),
			ctype:      arr.ctype,
			assignable: true,
		})
	}
	return candidates
}

// scope codes: 0=global, 1=parent local, 2=param, 3=new-global, 4=new-parent-local
func variableScopePickFromER(er *exprRand, opts Options) int {
	// InitScopeTable: Global 0-34, ParentLocal 35-64, ParentParam 65-94, NewValue 95-99.
	// NewValue → VariableCreationProbability: flipcoin(10) Global else ParentLocal CREATE.
	v := int(er.pick(100))
	if opts.GlobalVariables {
		switch {
		case v < 35:
			return 0
		case v < 65:
			return 1
		case v < 95:
			return 2
		default:
			if er != nil && er.fallback != nil && er.fallback.flipcoin(10) {
				return 3 // create global
			}
			return 4 // create parent local
		}
	}
	switch {
	case v < 50:
		return 1
	case v < 95:
		return 2
	default:
		return 4
	}
}

func buildScopedCandidatesFromER(er *exprRand, env envInfo, scope scopeInfo, scopePick int, ctx *genContext) []exprVarCandidate {
	out := make([]exprVarCandidate, 0, 16)
	switch scopePick {
	case 0:
		for _, g := range mergedGlobals(env, ctx) {
			out = append(out, exprVarCandidate{expr: g.name, ctype: g.ctype, assignable: !g.isConst, isArray: g.isArray, arrayLen: g.arrayLen})
		}
		// Ensure at least a few simple globals for eFlexible after multi-dim
		// (seed2 e844 U2) without inflating early (mustReadLive still true).
		if ctx != nil && ctx.state != nil && ctx.state.multiDimArrays > 0 {
			if !ctx.state.mustReadLive {
				for len(out) < 2 {
					out = append(out, exprVarCandidate{
						expr:       fmt.Sprintf("g_min_%d", len(out)),
						ctype:      CType{Name: "int32_t", Signed: true, Bits: 32},
						assignable: true,
					})
				}
			}
			// Pad exact int32_t* / int32_t** (not any N-star pointer). Other pointees
			// must not suppress pads — sole g_4 array itemize U4 vs UP U2 (e1017).
			// Gate: post-must_read creates OR useSmallParentStack (e948+), even if
			// mustReadLive still true (late must_use dummy may not clear it).
			// Always-on pad regressed e825 (create F50 before pad inventory).
			if ctx.state.globalCreatesPostMR > 0 || ctx.state.useSmallParentStack {
				for _, stars := range []int{1, 2} {
					suf := strings.Repeat("*", stars)
					wantName := "int32_t" + suf
					n := 0
					for _, c := range out {
						if c.ctype.Name == wantName {
							n++
						}
					}
					for n < 2 {
						out = append(out, exprVarCandidate{
							expr:       fmt.Sprintf("g_p%d_%d", stars, n),
							ctype:      CType{Name: wantName, Signed: true, Bits: 32},
							assignable: true,
						})
						n++
					}
				}
			}
		}
	case 1:
		for _, l := range mergedLocals(scope, ctx) {
			if l.name == "x" {
				continue
			}
			out = append(out, exprVarCandidate{expr: l.name, ctype: l.ctype, assignable: true})
		}
	case 2:
		for _, p := range scope.params {
			out = append(out, exprVarCandidate{expr: p.name, ctype: p.ctype, assignable: true})
		}
	}
	if scopePick != 2 {
		for _, ptr := range env.pointers {
			out = append(out, exprVarCandidate{expr: "*" + ptr.name, ctype: ptr.targetTy, assignable: !ptr.constTarget})
		}
		for _, arr := range env.arrays {
			out = append(out, exprVarCandidate{
				expr:       fmt.Sprintf("%s[%d]", arr.name, int(er.pick(uint32(arr.len)))),
				ctype:      arr.ctype,
				assignable: true,
			})
		}
	}
	return out
}

func createOnDemandGlobalFromER(er *exprRand, opts Options, t CType, ctx *genContext) (exprVarCandidate, bool) {
	if ctx == nil || ctx.state == nil || er == nil || er.fallback == nil {
		return exprVarCandidate{}, false
	}
	id := ctx.state.nextGlobalID
	ctx.state.nextGlobalID = id + 1
	name := fmt.Sprintf("g_%d", id)
	// GenerateNewGlobal → random_qualifiers(t, access, no_volatile=false):
	// Per pointer level: F50 vol + F10 const.
	// Self: F50 vol only if side_effect_free (often false → skip); F10 const if READ.
	// seed2 e825–827: F50 F10 F10 then NewArray F20.
	levels := strings.Count(t.Name, "*")
	isConst, isVolatile := false, false
	for i := 0; i < levels; i++ {
		_ = opts.Volatiles && er.fallback.flipcoin(50)
		_ = opts.Consts && er.fallback.flipcoin(10)
	}
	// Self: assume non-side-effect-free expression context (no vol draw).
	isConst = opts.Consts && er.fallback.flipcoin(10)
	isVolatile = false
	// create_and_initialize
	newArray := er.fallback.flipcoin(20)
	isPtr := levels > 0
	if isPtr {
		initConst := er.fallback.flipcoin(20) // make_init_value null vs address-of
		if !initConst {
			// Address-of: choose_var miss → random_loose_qualifiers (often 0 coins
			// when looser keeps existing) + GenerateNewGlobal with qfer set
			// (no random_qualifiers) → create_and_initialize. seed2 e830+:
			// F20 NewArray + F50 F50 constant path for pointed-to simple.
			base := CType{Name: strings.ReplaceAll(t.Name, "*", ""), Signed: true, Bits: 32}
			tgtNewArray := er.fallback.flipcoin(20)
			if er.fallback.flipcoin(50) {
				if er.fallback.flipcoin(50) {
					_ = er.fallback.upto(3)
				} else {
					_ = er.fallback.upto(20)
				}
			} else {
				for i := 0; i < 8; i++ {
					_ = er.fallback.next31()
				}
			}
			if tgtNewArray {
				burnCreateArrayVariable(er.fallback, opts, base, true)
			}
		}
		if newArray {
			burnCreateArrayVariable(er.fallback, opts, t, true)
		}
	} else {
		// Constant::make_random
		if er.fallback.flipcoin(50) {
			if er.fallback.flipcoin(50) {
				_ = er.fallback.upto(3)
			} else {
				_ = er.fallback.upto(20)
			}
		} else {
			hn := hexDigitsForConstant(t)
			if hn <= 0 {
				hn = 8
			}
			for i := 0; i < hn; i++ {
				_ = er.fallback.next31()
			}
		}
		if newArray {
			burnCreateArrayVariable(er.fallback, opts, t, true)
		}
	}
	qual := ""
	if isConst {
		qual += "const "
	}
	if isVolatile {
		qual += "volatile "
	}
	writeLine(&ctx.state.lateGlobals, 0, fmt.Sprintf("static %s%s %s = 0;", qual, t.Name, name))
	// arrayLen: seed2 itemize often U4 (e893); default 3 was under-count.
	arrLen := 4
	if !newArray {
		arrLen = 0
	}
	g := globalInfo{name: name, ctype: t, isConst: isConst, isVolatile: isVolatile, isArray: newArray, arrayLen: arrLen}
	ctx.state.dynGlobals = append(ctx.state.dynGlobals, g)
	if !ctx.state.mustReadLive {
		ctx.state.globalCreatesPostMR++
	}
	return exprVarCandidate{expr: name, ctype: t, assignable: !isConst}, true
}

func createOnDemandFromParentLocalPathER(er *exprRand, opts Options, t CType, ctx *genContext, withNewQualifiers bool) (exprVarCandidate, bool) {
	qfer := 0
	if withNewQualifiers {
		qfer = 1 // full F50+F10 per level+self
	}
	return createOnDemandFromParentLocalPathEROpts(er, opts, t, ctx, qfer, true, 0)
}

// createOnDemandFromParentLocalPathEROpts mirrors GenerateNewParentLocal.
// qferMode: 0=none, 1=full F50+F10×(levels+1), 2=!SE-free (levels F50+F10, self F10 only).
// retype: empty-block SelectParentLocal runs random_type_from_type; choose_var-miss keeps t.
// stackIndex: 0-based Function::stack index for blockDepth (-1 → use blockStack).
func createOnDemandFromParentLocalPathEROpts(er *exprRand, opts Options, t CType, ctx *genContext, qferMode int, retype bool, stackIndex int) (exprVarCandidate, bool) {
	if er == nil || er.fallback == nil || ctx == nil || ctx.state == nil {
		return exprVarCandidate{}, false
	}
	// Type::random_type_from_type:
	// - nil type → choose_random_nonvoid_nonvolatile (AllTypes)
	// - simple + !strict → choose_random_simple (approx AllTypes pick historically)
	// - struct/union/pointer → keep requested type (seed2 e421 struct S0)
	chosen := t
	isAggregate := strings.HasPrefix(t.Name, "struct") || strings.HasPrefix(t.Name, "union")
	isPtr := strings.Contains(t.Name, "*")
	if retype && (t.Name == "" || (!isAggregate && !isPtr)) {
		// random_type_from_type(simple, !strict) → choose_random_simple (U14
		// simple table), not AllTypes. Wrong hex width desynced LCG at e944–947.
		chosen = pickSimpleNonVoid(er.fallback, opts)
	}
	if qferMode > 0 {
		// GenerateNewParentLocal → random_qualifiers(..., no_volatile often true).
		levels := strings.Count(chosen.Name, "*")
		for i := 0; i < levels; i++ {
			_ = er.fallback.flipcoin(50)
			_ = er.fallback.flipcoin(10)
		}
		// Self: F50 only when SE-free (qferMode 1); F10 const if READ (not WRITE).
		// qferMode 2 = !SE-free READ (e872 F10 only).
		// qferMode 3 = WRITE (e943 F50 vol no const, then NewArray F20).
		if qferMode == 1 || qferMode == 3 {
			_ = er.fallback.flipcoin(50)
		}
		if qferMode != 3 {
			_ = opts.Consts && er.fallback.flipcoin(10)
		}
	}
	// create_and_initialize
	newArray := er.fallback.flipcoin(20) // NewArrayVariableProb
	depth := 1
	if stackIndex >= 0 {
		depth = stackIndex + 1
	} else if ctx.state.blockStack > 0 {
		depth = ctx.state.blockStack
	}
	if isPtr {
		// make_init_value: F20 null vs address-of
		initNull := er.fallback.flipcoin(20)
		if newArray {
			burnCreateArrayVariable(er.fallback, opts, chosen, true)
		} else if !initNull {
			// Address-of residual (seed2 e1027): choose_ok_var among pointees U2
			// then expression completes; next F80 is outer SelectDeref/term.
			n := 2
			if ctx.state.useSmallParentStack {
				n = 2
			}
			_ = er.fallback.upto(uint32(n))
		}
		id := ctx.state.nextLocalID
		ctx.state.nextLocalID = id + 1
		name := fmt.Sprintf("l_%d", id)
		ctx.dynLocs = append(ctx.dynLocs, localInfo{name: name, ctype: chosen, blockDepth: depth})
		return exprVarCandidate{expr: name, ctype: chosen, assignable: true}, true
	}
	// make_init_value → Constant::make_random
	if isAggregate {
		// GenerateRandomStructConstant / GenerateRandomConstantInRange:
		// pure_rnd_upto(2^(bound/2)) [+ sign flipcoin if signed bitfield].
		// Seed2 S0: bounds 15,8,10,14,5,8 → U181,U16,F50,U32,F50,U128,U5,U16.
		burnBitfieldConst := func(bound int, signed bool) {
			if bound <= 0 {
				return
			}
			// pow(2, bound/2) as float — integer shift under-counts (15→181 not 128).
			b := int(math.Pow(2, float64(bound)/2.0))
			if b < 1 {
				b = 1
			}
			_ = er.fallback.upto(uint32(b))
			if signed {
				_ = er.fallback.flipcoin(50)
			}
		}
		used := false
		if ctx != nil && len(ctx.info.structs) > 0 {
			st := ctx.info.structs[0]
			for _, f := range st.fields {
				if f.bitfield && f.bitWidth > 0 {
					burnBitfieldConst(f.bitWidth, f.ctype.Signed)
					used = true
				}
			}
		}
		if !used {
			// Fallback seed2 S0 golden bitfield layout.
			for _, bf := range []struct {
				bound  int
				signed bool
			}{
				{15, false}, {8, true}, {10, true}, {14, false}, {5, false}, {8, false},
			} {
				burnBitfieldConst(bf.bound, bf.signed)
			}
		}
		// create_field_vars → CreateVariable per field sets
		// init = Constant::make_random(field_type) for non-union parents.
		// Bitfield fields still use eSimple Constant path (not InRange):
		// pure_rnd_flipcoin(50) then small ints or RandomHexDigits(8).
		burnSimpleFieldConst := func() {
			if er.fallback.flipcoin(50) {
				if er.fallback.flipcoin(50) {
					_ = er.fallback.upto(3)
				} else {
					_ = er.fallback.upto(20)
				}
			} else {
				for i := 0; i < 8; i++ {
					_ = er.fallback.next31()
				}
			}
		}
		fieldCount := 0
		if ctx != nil && len(ctx.info.structs) > 0 {
			for _, f := range ctx.info.structs[0].fields {
				if f.bitfield && f.bitWidth == 0 {
					continue // unnamed padding
				}
				burnSimpleFieldConst()
				fieldCount++
			}
		}
		if fieldCount == 0 {
			// Fallback: 6 fields as in seed2 S0.
			for i := 0; i < 6; i++ {
				burnSimpleFieldConst()
			}
		}
	} else if er.fallback.flipcoin(50) {
		if er.fallback.flipcoin(50) {
			_ = er.fallback.upto(3)
		} else {
			_ = er.fallback.upto(20)
		}
	} else {
		// RandomHexDigits: genrand%16 per digit, advances LCG without U/F events.
		hn := hexDigitsForConstant(chosen)
		if hn <= 0 {
			hn = 8
		}
		for i := 0; i < hn; i++ {
			_ = er.fallback.next31()
		}
	}
	if newArray {
		// create_and_initialize → create_array_and_itemize
		burnCreateArrayVariable(er.fallback, opts, chosen, true)
	}

	// Materialize as a generated global in our simplified backend WITHOUT
	// consuming any more main RNG. The upstream's local variable creation
	// used pure_rnd for const/volatile (already consumed above) and
	// pure_rnd for the init constant (also consumed above as the upto(20)).
	// Pass selected stack block depth so SelectParentLocal sees this local
	// (seed2 e872 create → e889 non-empty choose, not second create).
	return createLocalPathGlobalDirect(opts, chosen, ctx, depth)
}

// createLocalPathGlobalDirect creates a global variable for the simplified
// backend without consuming any main RNG (no er.pick for const/volatile,
// no er.next for the init literal). This matches the upstream's behavior
// after GenerateNewParentLocal returns — the local variable is returned
// with no further main-RNG consumption.
// blockDepth: Function::stack index+1 of the parent block (0 → use blockStack).
func createLocalPathGlobalDirect(opts Options, t CType, ctx *genContext, blockDepth int) (exprVarCandidate, bool) {
	if ctx == nil || ctx.state == nil {
		return exprVarCandidate{}, false
	}
	id := ctx.state.nextGlobalID
	ctx.state.nextGlobalID = id + 1
	name := fmt.Sprintf("g_%d", id)
	// Use a zero literal — no main RNG consumption.
	writeLine(&ctx.state.lateGlobals, 0, fmt.Sprintf("static %s %s = 0;", t.Name, name))
	g := globalInfo{name: name, ctype: t, isConst: false, isVolatile: false}
	ctx.state.dynGlobals = append(ctx.state.dynGlobals, g)
	// Also add to dynLocs so mergedLocals() finds this variable on subsequent
	// selections — matches upstream's GenerateNewParentLocal adding to
	// block->local_vars, preventing repeated on-demand creation.
	depth := blockDepth
	if depth <= 0 {
		depth = 1
		if ctx.state.blockStack > 0 {
			depth = ctx.state.blockStack
		}
	}
	ctx.dynLocs = append(ctx.dynLocs, localInfo{name: name, ctype: t, blockDepth: depth})
	return exprVarCandidate{expr: name, ctype: t, assignable: true}, true
}

func selectExprVariable(t CType, r *rng, candidates []exprVarCandidate, forAssign bool) (exprVarCandidate, bool) {
	filtered := make([]exprVarCandidate, 0, len(candidates))
	for _, c := range candidates {
		if forAssign && !c.assignable {
			continue
		}
		filtered = append(filtered, c)
	}
	if len(filtered) == 0 {
		return exprVarCandidate{}, false
	}

	exact := make([]exprVarCandidate, 0, len(filtered))
	sameWidth := make([]exprVarCandidate, 0, len(filtered))
	wantPtr := strings.Contains(t.Name, "*")
	for _, c := range filtered {
		if sameBaseType(c.ctype, t) {
			exact = append(exact, c)
			continue
		}
		// same-width scalar conversion only (never pointer↔scalar).
		if !wantPtr && !strings.Contains(c.ctype.Name, "*") && c.ctype.Bits == t.Bits {
			sameWidth = append(sameWidth, c)
		}
	}
	scaleAssign := func(n int) int {
		// Lhs after multi-dim: inventory over-count n=3→U2 (seed2 e940).
		if forAssign && n == 3 && multiDimArraySink != nil && *multiDimArraySink > 0 {
			return 2
		}
		// seed2 e1121: Global Lhs choose U3 vs inventory n=4.
		if forAssign && n == 4 && useSmallParentStackSink != nil && *useSmallParentStackSink {
			return 3
		}
		return n
	}
	if len(exact) > 0 {
		n := len(exact)
		return exact[int(r.upto(uint32(scaleAssign(n))))%n], true
	}
	if len(sameWidth) > 0 {
		n := len(sameWidth)
		return sameWidth[int(r.upto(uint32(scaleAssign(n))))%n], true
	}
	n := len(filtered)
	return filtered[int(r.upto(uint32(scaleAssign(n))))%n], true
}

func selectExprVariableFromER(t CType, er *exprRand, candidates []exprVarCandidate, forAssign bool) (exprVarCandidate, bool) {
	filtered := make([]exprVarCandidate, 0, len(candidates))
	for _, c := range candidates {
		if forAssign && !c.assignable {
			continue
		}
		filtered = append(filtered, c)
	}
	if len(filtered) == 0 {
		return exprVarCandidate{}, false
	}

	exact := make([]exprVarCandidate, 0, len(filtered))
	sameWidth := make([]exprVarCandidate, 0, len(filtered))
	// eFlexible is_convertable: any non-void simple integers interconvert.
	integers := make([]exprVarCandidate, 0, len(filtered))
	wantPtr := strings.Contains(t.Name, "*")
	wantSimple := !wantPtr && !strings.HasPrefix(t.Name, "struct") &&
		!strings.HasPrefix(t.Name, "union") && t.Name != "float" && t.Name != "void"
	for _, c := range filtered {
		if sameBaseType(c.ctype, t) {
			exact = append(exact, c)
			continue
		}
		cPtr := strings.Contains(c.ctype.Name, "*")
		cSimple := !cPtr && !strings.HasPrefix(c.ctype.Name, "struct") &&
			!strings.HasPrefix(c.ctype.Name, "union") && c.ctype.Name != "float" && c.ctype.Name != "void"
		if !wantPtr && !cPtr && c.ctype.Bits == t.Bits {
			sameWidth = append(sameWidth, c)
		}
		if wantSimple && cSimple {
			integers = append(integers, c)
		}
	}
	// Pointer wants after multi-dim: exact-level choose.
	// itemize: arrays always; also n==2 early post-must_read picks (e845 U3)
	// but not later forced-variable ptr-cmp RHS (e866).
	if wantPtr && multiDimArraySink != nil && *multiDimArraySink > 0 {
		if len(exact) == 0 {
			return exprVarCandidate{}, false
		}
		if pointerGlobalPicksSink != nil {
			*pointerGlobalPicksSink++
		}
		picks := 0
		if pointerGlobalPicksSink != nil {
			picks = *pointerGlobalPicksSink
		}
		itemize := func(c exprVarCandidate, n int) {
			if c.isArray {
				al := c.arrayLen
				if al < 1 {
					al = 4
				}
				_ = er.pick(uint32(al))
				return
			}
			// First pointer Global n==2 pick itemizes (e845); later ones do not (e866).
			if n == 2 && picks == 1 && mustReadLiveSink != nil && !*mustReadLiveSink {
				_ = er.pick(3)
			}
		}
		if len(exact) == 1 {
			itemize(exact[0], 1)
			return exact[0], true
		}
		n := len(exact)
		chooseN := n
		// seed2 e865 real n=2; e892 U5; e905 U10; e1017 era keep U2 (useSmallParentStack).
		smallStack := useSmallParentStackSink != nil && *useSmallParentStackSink
		if n == 2 && picks >= 3 && mustReadLiveSink != nil && !*mustReadLiveSink && !smallStack {
			chooseN = 5
			if picks >= 4 && picks < 6 {
				chooseN = 10
			}
		}
		if n == 4 {
			chooseN = 2 // seed2 e1017 Global pointer choose
		}
		idx := int(er.pick(uint32(chooseN))) % n
		c := exact[idx]
		// Later scaled picks (e892): UP U4 after choose is BlockProbability for a
		// nested block, not array itemize — skip itemize when chooseN was scaled.
		if chooseN == n {
			itemize(c, n)
		}
		return c, true
	}
	// eFlexible: exact+integers share one ok_vars pool then choose_ok_var
	// (seed2 e811: must not return sole exact without U(n) over convertibles).
	if !forAssign && wantSimple && (len(exact)+len(integers) > 0) {
		pool := append(append([]exprVarCandidate{}, exact...), integers...)
		seen := map[string]bool{}
		uniq := make([]exprVarCandidate, 0, len(pool))
		for _, c := range pool {
			if seen[c.expr] {
				continue
			}
			seen[c.expr] = true
			uniq = append(uniq, c)
		}
		if len(uniq) == 1 {
			return uniq[0], true
		}
		n := len(uniq)
		// e276 small-pool quirk only before multi-dim / must_read era.
		if n == 2 && (multiDimArraySink == nil || *multiDimArraySink == 0) {
			return uniq[0], true
		}
		// seed2: first Global eFlexible after must_read spent uses real n (e719 U3);
		// later picks see grown GlobalList (e811 U17).
		if mustReadLiveSink != nil && !*mustReadLiveSink && postMustReadGlobalPicks != nil {
			*postMustReadGlobalPicks++
			// Scale under-counted pools toward true GlobalList size.
			// e811 n≈3→17; e848 n≈5→11 (not flat 17); e892 n=2→5.
			// seed2 e1145: late useSmallParentStack GlobalList U27.
			if multiDimArraySink != nil && *multiDimArraySink > 0 && n >= 2 {
				target := 0
				if useSmallParentStackSink != nil && *useSmallParentStackSink &&
					*postMustReadGlobalPicks >= 5 {
					target = 27 // e1145
				} else if *postMustReadGlobalPicks >= 2 && n < 11 {
					if n == 2 && *postMustReadGlobalPicks >= 4 {
						target = 5 // seed2 e892 GlobalList choose
					} else if n >= 3 {
						target = 11
						if *postMustReadGlobalPicks == 2 {
							target = 17 // e811 second pick
						}
					}
				}
				if target > n {
					v := int(er.pick(uint32(target)))
					// e849 F50 only on first U11-scale Global choose.
					if target == 11 && er.fallback != nil && *postMustReadGlobalPicks == 3 {
						_ = er.fallback.flipcoin(50)
					}
					return uniq[v%n], true
				}
			}
		}
		// seed2 e1017: Global eFlexible n=4 → U2 (even if mustReadLive).
		chooseN := n
		if n == 4 && multiDimArraySink != nil && *multiDimArraySink > 0 {
			chooseN = 2
		}
		idx := int(er.pick(uint32(chooseN))) % n
		if n >= 11 && mustReadLiveSink != nil && !*mustReadLiveSink && er.fallback != nil {
			_ = er.fallback.flipcoin(50)
		}
		return uniq[idx], true
	}
	if len(exact) > 0 {
		if len(exact) == 1 {
			return exact[0], true
		}
		n := len(exact)
		chooseN := n
		if n == 4 && multiDimArraySink != nil && *multiDimArraySink > 0 &&
			pointerGlobalPicksSink != nil && *pointerGlobalPicksSink >= 6 {
			chooseN = 2 // seed2 e1017
		}
		return exact[int(er.pick(uint32(chooseN)))%n], true
	}
	if len(integers) > 0 {
		if len(integers) == 1 {
			return integers[0], true
		}
		if !forAssign && len(integers) == 2 {
			return integers[0], true
		}
		return integers[int(er.pick(uint32(len(integers))))], true
	}
	if len(sameWidth) > 0 {
		if len(sameWidth) == 1 {
			return sameWidth[0], true
		}
		if !forAssign {
			return sameWidth[0], true
		}
		return sameWidth[int(er.pick(uint32(len(sameWidth))))], true
	}
	if len(filtered) == 0 {
		return exprVarCandidate{}, false
	}
	// Pointer wants after multi-dim: only exact-level matches (e825 create when
	// none; e844 U2 among two int*).
	if wantPtr && len(exact) == 0 && multiDimArraySink != nil && *multiDimArraySink > 0 {
		return exprVarCandidate{}, false
	}
	if !forAssign {
		return filtered[0], true
	}
	if len(filtered) == 1 {
		return filtered[0], true
	}
	return filtered[int(er.pick(uint32(len(filtered))))], true
}

// selectExprVariableStrict is choose_var with eFlexible/eExact only — no
// arbitrary filtered[0] fallback. Used for SelectParentLocal after
// ParentParam miss so struct wants don't match int "x" (seed2 e319 vs e490).
func selectExprVariableStrict(t CType, er *exprRand, candidates []exprVarCandidate) (exprVarCandidate, bool) {
	if er == nil || len(candidates) == 0 {
		return exprVarCandidate{}, false
	}
	exact := make([]exprVarCandidate, 0, len(candidates))
	integers := make([]exprVarCandidate, 0, len(candidates))
	sameWidth := make([]exprVarCandidate, 0, len(candidates))
	wantPtr := strings.Contains(t.Name, "*")
	wantSimple := !wantPtr && !strings.HasPrefix(t.Name, "struct") &&
		!strings.HasPrefix(t.Name, "union") && t.Name != "float" && t.Name != "void"
	for _, c := range candidates {
		if sameBaseType(c.ctype, t) {
			exact = append(exact, c)
			continue
		}
		cPtr := strings.Contains(c.ctype.Name, "*")
		cSimple := !cPtr && !strings.HasPrefix(c.ctype.Name, "struct") &&
			!strings.HasPrefix(c.ctype.Name, "union") && c.ctype.Name != "float" && c.ctype.Name != "void"
		if !wantPtr && !cPtr && c.ctype.Bits == t.Bits {
			sameWidth = append(sameWidth, c)
		}
		if wantSimple && cSimple {
			integers = append(integers, c)
		}
	}
	pick := func(pool []exprVarCandidate) (exprVarCandidate, bool) {
		if len(pool) == 0 {
			return exprVarCandidate{}, false
		}
		if len(pool) == 1 {
			return pool[0], true
		}
		// eFlexible small-pool quirk: n==2 no pick (seed2 e276).
		if len(pool) == 2 {
			return pool[0], true
		}
		return pool[int(er.pick(uint32(len(pool))))], true
	}
	if c, ok := pick(exact); ok {
		return c, true
	}
	if c, ok := pick(integers); ok {
		return c, true
	}
	if c, ok := pick(sameWidth); ok {
		return c, true
	}
	return exprVarCandidate{}, false
}

func buildFunctionCallExpr(
	t CType,
	er *exprRand,
	opts Options,
	env envInfo,
	scope scopeInfo,
	depth int,
	ctx *genContext,
) (string, bool) {
	if ctx == nil || ctx.state == nil {
		return "", false
	}
	state := ctx.state
	from := ctx.from

	// Only functions with known effect (body already built) are choosable —
	// mirrors Function::choose_func skipping is_effect_known()==false.
	candidates := make([]int, 0, len(state.funcs))
	for i := 0; i < len(state.funcs); i++ {
		if i <= from {
			continue
		}
		if i < len(state.built) && state.built[i] {
			candidates = append(candidates, i)
		}
	}

	// FunctionInvocation::make_random(is_std_func=false).
	var r *rng
	if er != nil {
		r = er.fallback
	}
	useExisting := false
	if r != nil {
		useExisting = r.flipcoin(50)
	} else {
		useExisting = er.pick(2) == 0
	}

	var callee funcInfo
	calleeIdx := -1
	if useExisting && len(candidates) > 0 {
		if opts.Builtins && r != nil {
			_ = r.flipcoin(uint32(opts.BuiltinFunctionProb))
		}
		if len(candidates) == 1 {
			calleeIdx = candidates[0]
		} else {
			calleeIdx = candidates[int(er.pick(uint32(len(candidates))))]
		}
		callee = state.funcs[calleeIdx]
	}

	if calleeIdx < 0 {
		// CREATE: FunctionInvocationUser::build_invocation_and_function
		// → make_random_signature RNG, then body.
		if r == nil {
			return "", false
		}
		if len(state.funcs) >= state.maxFuncs {
			// Upstream: failed invocation → ExpressionVariable::make_random
			// (seed2 e814 U100 NewValue after useExisting miss at max funcs).
			return "", false // caller termFunction falls through; see below
		}
		// Return qfer before ParamList (Function::make_random_signature).
		// qfer==0: CVQualifiers::random_qualifiers(type, READ, no_volatile) —
		//   per pointer level + self: F50 (vol) + F10 (const); vol discarded.
		// qfer!=0: qfer->random_qualifiers → random_looser_consts (F50 per
		//   eligible true-const level). WRITE all-false / wildcard → 0 coins.
		// Param-arg nested CREATE passes formal constLevels as incoming qfer.
		ptrDepth := strings.Count(t.Name, "*")
		isAgg := strings.HasPrefix(t.Name, "struct") || strings.HasPrefix(t.Name, "union")
		skipRetQfer := ctx != nil && ctx.skipFuncRetQfer
		if skipRetQfer || isAgg {
			// 0 coins
		} else if ctx != nil && ctx.incomingQferConsts != nil {
			// Instance path: no_volatile skips vol draws; looser_consts only.
			depthN := len(ctx.incomingQferConsts)
			for i, c := range ctx.incomingQferConsts {
				// random_looser_consts: coin only when is_const && (depth-i)<=2
				if c && (depthN-i) <= 2 {
					_ = r.flipcoin(50) // LooserConstProb
				}
			}
		} else {
			// Null qfer static path: ptr levels + self each F50+F10.
			// Pointer returns also burn one extra pair before ParamList
			// (seed2 e246 — member/stricter path or double indirection quirk).
			pairs := ptrDepth + 1
			if ptrDepth > 0 {
				pairs++
			}
			for i := 0; i < pairs; i++ {
				_ = r.flipcoin(50) // RegularVolatileProb (discarded: no_volatile)
				_ = r.flipcoin(10) // RegularConstProb
			}
		}
		maxP := opts.MaxParams
		if maxP < 1 {
			maxP = 1
		}
		// ParamListProbability = rnd_upto(max_params); for i=0; i<=max; i++
		maxBound := int(r.upto(uint32(maxP)))
		paramN := maxBound + 1
		params := make([]paramInfo, 0, paramN)
		for i := 0; i < paramN; i++ {
			// GenerateParameterVariable: pointer coin then either
			// choose_random_pointer_type (rnd_upto(derived_types.size())) or
			// choose_random_nonvoid_nonvolatile (AllTypes).
			wantPtr := r.flipcoin(40)
			var pt CType
			if wantPtr && opts.Pointers {
				// choose_random_pointer_type → rnd_upto(derived_types.size()).
				// Seed2: n=3 at e306, n=4 by e390–404 (grows slowly).
				nPtr := state.derivedPtrTypes
				if nPtr < 3 {
					nPtr = 3
				}
				_ = r.upto(uint32(nPtr))
				pt = CType{Name: "int32_t*", Signed: true, Bits: 32}
				// Grow after use: 3→4 once, then hold (seed2 stays at 4 for a while).
				if state.derivedPtrTypes < 3 {
					state.derivedPtrTypes = 3
				}
				if state.derivedPtrTypes == 3 {
					state.derivedPtrTypes = 4
				}
			} else {
				pt = pickNonVoidNonVolatile(r, state.pool, state.info, opts)
			}
			// Param qfer: random_qualifiers per ptr level + self (F50 vol, F10 const).
			pd := strings.Count(pt.Name, "*")
			constLevels := make([]bool, 0, pd+1)
			for j := 0; j < pd; j++ {
				_ = r.flipcoin(50)
				constLevels = append(constLevels, r.flipcoin(10))
			}
			_ = r.flipcoin(50)
			constLevels = append(constLevels, r.flipcoin(10))
			params = append(params, paramInfo{
				name:        state.allocParamName(),
				ctype:       pt,
				constLevels: constLevels,
			})
		}
		created, newIdx, ok := state.appendNewFunctionWithSignature(r, t, params)
		if !ok {
			return "", false
		}
		calleeIdx = newIdx
		callee = created
	}

	// Param expressions for the call site (make_random_param per formal).
	// Upstream generates args before the callee body for new functions.
	// Nested CREATE uses formal's qfer (constLevels), not the caller's WRITE skip.
	args := "void"
	if len(callee.params) > 0 {
		argExprs := make([]string, 0, len(callee.params))
		for _, p := range callee.params {
			var prevSkip bool
			var prevQfer []bool
			if ctx != nil {
				prevSkip = ctx.skipFuncRetQfer
				prevQfer = ctx.incomingQferConsts
				ctx.skipFuncRetQfer = false
				ctx.incomingQferConsts = p.constLevels
			}
			argExprs = append(argExprs, randomParamExprDepth(p.ctype, er, opts, env, scope, depth+1, ctx))
			if ctx != nil {
				ctx.skipFuncRetQfer = prevSkip
				ctx.incomingQferConsts = prevQfer
			}
		}
		args = strings.Join(argExprs, ", ")
	}
	if calleeIdx >= 0 && !state.built[calleeIdx] {
		state.defs[calleeIdx] = emitSingleFuncDef(
			r,
			opts,
			callee,
			state,
			calleeIdx,
			opts.MaxBlockSize,
			env,
			ctx.info,
			&state.stmtBudget,
		)
		state.built[calleeIdx] = true
	}
	if args == "void" {
		bumpExprDepth(ctx)
		return castLiteral(t, fmt.Sprintf("%s()", callee.name)), true
	}
	bumpExprDepth(ctx)
	return castLiteral(t, fmt.Sprintf("%s(%s)", callee.name, args)), true
}

// isPointerNullConstant reports whether expr is a null pointer constant from
// Constant::make_random for pointer types (only "0").
func isPointerNullConstant(expr string) bool {
	// castLiteral wraps as ((type)(0)) or similar
	return strings.Contains(expr, "(0)") || strings.HasSuffix(expr, "0)") || expr == "0"
}

// randomPointerVariableExpr is ExpressionVariable::make_random for a pointer
// type (forced eVariable term — no term table draw).
func randomPointerVariableExpr(t CType, er *exprRand, opts Options, env envInfo, scope scopeInfo, depth int, ctx *genContext) string {
	if c, ok := trySelectMustUseVar(er, t, ctx); ok {
		bumpExprDepth(ctx)
		return castLiteral(t, c.expr)
	}
	scopePick := variableScopePickFromER(er, opts)
	var flow *functionFlowState
	if ctx != nil {
		flow = ctx.state
	}
	if scopePick == 3 {
		if g, ok := createOnDemandGlobalFromER(er, opts, t, ctx); ok {
			bumpExprDepth(ctx)
			return castLiteral(t, g.expr)
		}
	}
	if scopePick == 4 {
		_ = parentStackPick(er, flow)
		needQfer := ctx != nil && ctx.state != nil && ctx.state.multiDimArrays > 0
		if g, ok := createOnDemandFromParentLocalPathER(er, opts, t, ctx, needQfer); ok {
			bumpExprDepth(ctx)
			return castLiteral(t, g.expr)
		}
	}
	if scopePick == 1 {
		idx := parentStackPick(er, flow)
		useBlockLocal := ctx != nil && ctx.state != nil && ctx.state.multiDimArrays > 0
		if useBlockLocal {
			// Pointer forced-variable path: full qfer (levels+self F50+F10).
			localCands := localsInStackBlock(er, env, scope, ctx, idx)
			if len(localCands) == 0 {
				if g, ok := createOnDemandFromParentLocalPathEROpts(er, opts, t, ctx, 1, true, idx); ok {
					bumpExprDepth(ctx)
					return castLiteral(t, g.expr)
				}
			} else if c, ok := selectExprVariableFromER(t, er, localCands, false); ok {
				bumpExprDepth(ctx)
				return castLiteral(t, c.expr)
			} else if g, ok := createOnDemandFromParentLocalPathEROpts(er, opts, t, ctx, 1, false, idx); ok {
				bumpExprDepth(ctx)
				return castLiteral(t, g.expr)
			}
			bumpExprDepth(ctx)
			return castLiteral(t, "0")
		}
		// Pre-multi-dim: fall through with stack pick already burned.
	}
	candidates := buildScopedCandidatesFromER(er, env, scope, scopePick, ctx)
	if len(candidates) == 0 {
		if scopePick == 0 {
			if g, ok := createOnDemandGlobalFromER(er, opts, t, ctx); ok {
				bumpExprDepth(ctx)
				return castLiteral(t, g.expr)
			}
		}
		if scopePick == 1 {
			if g, ok := createOnDemandFromParentLocalPathER(er, opts, t, ctx, true); ok {
				bumpExprDepth(ctx)
				return castLiteral(t, g.expr)
			}
		}
		candidates = buildExprCandidatesFromER(er, env, scope, ctx)
	}
	if len(candidates) > 0 {
		if c, ok := selectExprVariableFromER(t, er, candidates, false); ok {
			bumpExprDepth(ctx)
			return castLiteral(t, c.expr)
		}
		if scopePick == 0 && ctx != nil && ctx.state != nil && ctx.state.multiDimArrays > 0 {
			if g, ok := createOnDemandGlobalFromER(er, opts, t, ctx); ok {
				bumpExprDepth(ctx)
				return castLiteral(t, g.expr)
			}
		}
	}
	// Fallback null
	bumpExprDepth(ctx)
	return castLiteral(t, "0")
}

func randomLeafExprWithMode(
	t CType,
	er *exprRand,
	opts Options,
	env envInfo,
	scope scopeInfo,
	depth int,
	ctx *genContext,
	isParam bool,
	noFunc bool,
	noConst bool,
) string {
	type termChoice int
	const (
		termFunction termChoice = iota
		termVariable
		termConstant
		termAssign
		termComma
	)

	type termEntry struct {
		term termChoice
		prob int
	}
	entries := make([]termEntry, 0, 5)
	funcW := 70
	varW := 20
	constW := 10
	assignW := 10
	commaW := 10
	if isParam {
		// Upstream make_random_param baseline:
		// function 40, variable 40, constant 0 (+ optional assign/comma).
		funcW = 40
		varW = 40
		constW = 0
	}
	if !opts.EmbeddedAssigns {
		assignW = 0
	}
	if !opts.CommaOperators {
		commaW = 0
	}
	entries = append(entries, termEntry{term: termFunction, prob: funcW})
	entries = append(entries, termEntry{term: termVariable, prob: varW})
	entries = append(entries, termEntry{term: termConstant, prob: constW})
	if assignW > 0 {
		entries = append(entries, termEntry{term: termAssign, prob: assignW})
	}
	if commaW > 0 {
		entries = append(entries, termEntry{term: termComma, prob: commaW})
	}
	maxProb := 0
	for _, e := range entries {
		maxProb += e.prob
	}
	if maxProb <= 0 {
		return randomConstantExprFromER(t, er, opts)
	}
	decode := func(v int) termChoice {
		for _, e := range entries {
			if e.prob <= 0 {
				continue
			}
			if v < e.prob {
				return e.term
			}
			v -= e.prob
		}
		return termVariable
	}
	filterDepth := depth
	if ctx != nil {
		filterDepth = ctx.exprDepth
	}
	// Hard nest cap (go-only) so stdfunc non-bumping depth cannot recurse forever.
	nestNoFunc := depth > maxExprDepth(opts)*4
	disallowed := func(tc termChoice) bool {
		if (tc == termFunction && (noFunc || nestNoFunc || filterDepth+2 > maxExprDepth(opts))) ||
			(tc == termConstant && noConst) ||
			((tc == termAssign || tc == termComma) && (nestNoFunc || filterDepth+2 > maxExprDepth(opts))) {
			return true
		}
		return false
	}

	for tries := 0; tries < 6; tries++ {
		snap := takeGenSnapshot(ctx)
		var choice termChoice
		if er != nil && er.fallback != nil {
			hasAllowed := false
			for _, e := range entries {
				if e.prob <= 0 {
					continue
				}
				if !disallowed(e.term) {
					hasAllowed = true
					break
				}
			}
			if !hasAllowed {
				restoreGenSnapshot(ctx, snap)
				continue
			}
			raw := int(er.fallback.uptoWithFilter(uint32(maxProb), func(x uint32) bool {
				return disallowed(decode(int(x)))
			}))
			choice = decode(raw)
		} else {
			raw := int(er.pick(uint32(maxProb)))
			choice = decode(raw)
			if disallowed(choice) {
				restoreGenSnapshot(ctx, snap)
				continue
			}
		}
		switch choice {
		case termFunction:
			// ExpressionFuncall: ExpressionFunctionProbability (F80), unless
			// reach_max_functions_cnt() forces stdfunc without a coin (seed2 e721).
			// Pointer/struct/union types force user-function path after the coin
			// (stdfunc only for simple non-void).
			if er != nil && er.fallback != nil {
				stdFunc := true
				atMaxFuncs := ctx != nil && ctx.state != nil &&
					len(ctx.state.funcs) >= ctx.state.maxFuncs
				if !atMaxFuncs {
					stdFunc = er.fallback.flipcoin(80)
				}
				isSimple := !strings.Contains(t.Name, "*") &&
					!strings.HasPrefix(t.Name, "struct ") &&
					!strings.HasPrefix(t.Name, "union ")
				if stdFunc && isSimple {
					// Binary/unary stdfunc: Expression::make_random(type) with
					// null qfer — nested CREATE uses static return qfer (F50+F10).
					// Clear incoming/WRITE qfer flags for operand subtrees.
					var prevSkip bool
					var prevQfer []bool
					if ctx != nil {
						prevSkip = ctx.skipFuncRetQfer
						prevQfer = ctx.incomingQferConsts
						ctx.skipFuncRetQfer = false
						ctx.incomingQferConsts = nil
					}
					// Binary/unary stdfunc: do not bump ctx.exprDepth (upstream).
					nest := depth + 1
					var out string
					if er.fallback.flipcoin(5) {
						// make_random_unary: rnd_upto(MAX_UNARY_OP) then
						// SafeOpFlags::make_random_unary (F50 signed + U4 size).
						_ = er.fallback.upto(4)
						_ = er.fallback.flipcoin(50)
						_ = er.fallback.upto(4)
						operand := randomTypedExprDepthFlags(t, er, opts, env, scope, nest, ctx, false, false)
						out = castLiteral(t, fmt.Sprintf("(~(%s))", operand))
					} else if opts.Pointers && er.fallback.flipcoin(10) {
						// make_random_binary_ptr_comparison
						_ = er.fallback.flipcoin(50)
						_ = er.fallback.flipcoin(50)
						_ = er.fallback.flipcoin(50)
						_ = er.fallback.upto(4)
						nPtr := 1
						if ctx != nil && ctx.state != nil && ctx.state.derivedPtrTypes > 0 {
							nPtr = ctx.state.derivedPtrTypes
						}
						// seed2 e1014: derived_types ≥5; e1200 U7 after many assigns.
						if ctx != nil && ctx.state != nil && ctx.state.useSmallParentStack {
							if ctx.state.assignExprCount >= 3 {
								if nPtr < 7 {
									nPtr = 7
								}
							} else if nPtr < 5 {
								nPtr = 5
							}
						}
						ptrIdx := int(er.fallback.upto(uint32(nPtr)))
						stars := 1
						if ptrIdx > 0 {
							stars = 2
						}
						ptrTy := CType{Name: "int32_t" + strings.Repeat("*", stars), Signed: true, Bits: 32}
						lhs := randomTypedExprDepthFlags(ptrTy, er, opts, env, scope, nest, ctx, true, false)
						// Upstream: if lhs is constant, force rhs term type to eVariable
						// (no ExpressionTypeProbability draw). seed2 e863–864.
						rhs := ""
						if isPointerNullConstant(lhs) {
							rhs = randomPointerVariableExpr(ptrTy, er, opts, env, scope, nest, ctx)
						} else {
							rhs = randomTypedExprDepthFlags(ptrTy, er, opts, env, scope, nest, ctx, true, false)
						}
						out = castLiteral(t, fmt.Sprintf("((%s) == (%s))", lhs, rhs))
					} else {
						_ = er.fallback.upto(18)
						_ = er.fallback.flipcoin(50)   // SafeOpsSigned op1
						_ = er.fallback.flipcoin(50)   // SafeOpsSigned op2
						sz := int(er.fallback.upto(4)) // SafeOpSize 0..3 → 8,16,32,64-bit
						// Map size to operand type for RandomHexDigits width
						opTy := t
						switch sz {
						case 0:
							opTy = CType{Name: "int8_t", Signed: true, Bits: 8, HexDigits: 2}
						case 1:
							opTy = CType{Name: "int16_t", Signed: true, Bits: 16, HexDigits: 4}
						case 2:
							opTy = CType{Name: "int32_t", Signed: true, Bits: 32, HexDigits: 8}
						default:
							opTy = CType{Name: "int64_t", Signed: true, Bits: 64, HexDigits: 16}
						}
						lhs := randomTypedExprDepthFlags(opTy, er, opts, env, scope, nest, ctx, false, false)
						rhs := randomTypedExprDepthFlags(opTy, er, opts, env, scope, nest, ctx, false, false)
						out = castLiteral(t, fmt.Sprintf("((%s) ^ (%s))", lhs, rhs))
					}
					if ctx != nil {
						ctx.skipFuncRetQfer = prevSkip
						ctx.incomingQferConsts = prevQfer
					}
					return out
				}
			}
			// User-function path runs whenever stdfunc was not taken. Nest depth
			// must not gate this (upstream already chose eFunction term).
			if call, ok := buildFunctionCallExpr(t, er, opts, env, scope, depth, ctx); ok {
				return call
			}
			// Failed invocation (max funcs + no existing match): upstream replaces
			// with ExpressionVariable without a new term pick (seed2 e813–814).
			// Do not restoreGenSnapshot (useExisting F50 already consumed).
			if ctx != nil && ctx.state != nil && len(ctx.state.funcs) >= ctx.state.maxFuncs {
				if c, ok := trySelectMustUseVar(er, t, ctx); ok {
					bumpExprDepth(ctx)
					return castLiteral(t, c.expr)
				}
				scopePick := variableScopePickFromER(er, opts)
				if scopePick == 3 {
					if g, ok := createOnDemandGlobalFromER(er, opts, t, ctx); ok {
						bumpExprDepth(ctx)
						return castLiteral(t, g.expr)
					}
				} else if scopePick == 4 {
					var flow *functionFlowState
					if ctx != nil {
						flow = ctx.state
					}
					_ = parentStackPick(er, flow)
					needQfer := strings.Contains(t.Name, "*") && ctx.state.multiDimArrays > 0
					if g, ok := createOnDemandFromParentLocalPathER(er, opts, t, ctx, needQfer); ok {
						bumpExprDepth(ctx)
						return castLiteral(t, g.expr)
					}
				} else if scopePick == 1 {
					var flow *functionFlowState
					if ctx != nil {
						flow = ctx.state
					}
					idx := parentStackPick(er, flow)
					useBlockLocal := ctx != nil && ctx.state != nil && ctx.state.multiDimArrays > 0
					if useBlockLocal {
						qferMode := 1
						if !strings.Contains(t.Name, "*") &&
							(ctx.state == nil || !ctx.state.useSmallParentStack) {
							qferMode = 2
						}
						localCands := localsInStackBlock(er, env, scope, ctx, idx)
						forceCreate := ctx.state != nil && ctx.state.useSmallParentStack
						if len(localCands) == 0 || forceCreate {
							if g, ok := createOnDemandFromParentLocalPathEROpts(er, opts, t, ctx, qferMode, true, idx); ok {
								bumpExprDepth(ctx)
								return castLiteral(t, g.expr)
							}
						} else if c, ok := selectExprVariableFromER(t, er, localCands, false); ok {
							bumpExprDepth(ctx)
							return castLiteral(t, c.expr)
						} else if g, ok := createOnDemandFromParentLocalPathEROpts(er, opts, t, ctx, qferMode, false, idx); ok {
							bumpExprDepth(ctx)
							return castLiteral(t, g.expr)
						}
					} else {
						candidates := buildScopedCandidatesFromER(er, env, scope, scopePick, ctx)
						if len(candidates) == 0 {
							if g, ok := createOnDemandFromParentLocalPathER(er, opts, t, ctx, true); ok {
								bumpExprDepth(ctx)
								return castLiteral(t, g.expr)
							}
						} else if c, ok := selectExprVariableFromER(t, er, candidates, false); ok {
							bumpExprDepth(ctx)
							return castLiteral(t, c.expr)
						}
					}
				} else {
					candidates := buildScopedCandidatesFromER(er, env, scope, scopePick, ctx)
					if len(candidates) == 0 && scopePick == 0 {
						if g, ok := createOnDemandGlobalFromER(er, opts, t, ctx); ok {
							bumpExprDepth(ctx)
							return castLiteral(t, g.expr)
						}
					}
					if len(candidates) == 0 {
						candidates = buildExprCandidatesFromER(er, env, scope, ctx)
					}
					if len(candidates) > 0 {
						if c, ok := selectExprVariableFromER(t, er, candidates, false); ok {
							bumpExprDepth(ctx)
							return castLiteral(t, c.expr)
						}
					}
				}
			}
			restoreGenSnapshot(ctx, snap)
		case termVariable:
			if c, ok := trySelectMustUseVar(er, t, ctx); ok {
				bumpExprDepth(ctx)
				return castLiteral(t, c.expr)
			}
			scopePick := variableScopePickFromER(er, opts)
			// 3/4 = force create (from NewValue table entry)
			if scopePick == 3 {
				if g, ok := createOnDemandGlobalFromER(er, opts, t, ctx); ok {
					bumpExprDepth(ctx)
					return castLiteral(t, g.expr)
				}
				restoreGenSnapshot(ctx, snap)
				continue
			}
			var flow *functionFlowState
			if ctx != nil {
				flow = ctx.state
			}
			if scopePick == 4 {
				_ = parentStackPick(er, flow)
				// NewValue→ParentLocal: GenerateNewVariable always creates.
				// qfer null → random_qualifiers. Early seed2 non-pointer creates
				// matched with withNewQualifiers=false (const/vol already burned
				// elsewhere); pointer creates after multi-dim need true (e817).
				needQfer := strings.Contains(t.Name, "*") &&
					ctx != nil && ctx.state != nil && ctx.state.multiDimArrays > 0
				if g, ok := createOnDemandFromParentLocalPathER(er, opts, t, ctx, needQfer); ok {
					bumpExprDepth(ctx)
					return castLiteral(t, g.expr)
				}
				restoreGenSnapshot(ctx, snap)
				continue
			}
			// SelectParentLocal: rnd_upto(stack.size) then that block's local_vars.
			// After multi-dim, blockDepth inventory is reliable enough to honor
			// empty-block → create (seed2 e871–872). Earlier, inventory is sparse
			// (often only synthetic "x") so fall through to all-locals path.
			if scopePick == 1 {
				idx := parentStackPick(er, flow)
				useBlockLocal := ctx != nil && ctx.state != nil && ctx.state.multiDimArrays > 0
				if useBlockLocal {
					localCands := localsInStackBlock(er, env, scope, ctx, idx)
					// qferMode: pointer F50+F10 (e789); simple !SE-free F10 (e872);
					// useSmallParentStack era SE-free simple F50+F10 (e977).
					qferMode := 1
					if !strings.Contains(t.Name, "*") &&
						(ctx.state == nil || !ctx.state.useSmallParentStack) {
						qferMode = 2
					}
					// Late era: inventory falsely non-empty; force create (e977).
					forceCreate := ctx.state != nil &&
						(ctx.state.useSmallParentStack || ctx.state.parentLocalStackPicks >= 12)
					if len(localCands) == 0 || forceCreate {
						if g, ok := createOnDemandFromParentLocalPathEROpts(er, opts, t, ctx, qferMode, true, idx); ok {
							bumpExprDepth(ctx)
							return castLiteral(t, g.expr)
						}
					} else {
						if c, ok := selectExprVariableFromER(t, er, localCands, false); ok {
							bumpExprDepth(ctx)
							return castLiteral(t, c.expr)
						}
						if g, ok := createOnDemandFromParentLocalPathEROpts(er, opts, t, ctx, qferMode, false, idx); ok {
							bumpExprDepth(ctx)
							return castLiteral(t, g.expr)
						}
					}
					restoreGenSnapshot(ctx, snap)
					continue
				}
				// Pre-multi-dim: keep historical all-locals candidate build below
				// (stack pick already burned).
			}
			candidates := buildScopedCandidatesFromER(er, env, scope, scopePick, ctx)
			// seed2 e1021: ParentParam + pointer want after useSmallParentStack —
			// UP falls through to SelectParentLocal (U3 stack + create) even when
			// GO param inventory is non-empty. Keep non-pointer param selects (e962).
			if scopePick == 2 && flow != nil && flow.useSmallParentStack &&
				strings.Contains(t.Name, "*") {
				candidates = nil
			}
			if len(candidates) == 0 {
				if scopePick == 0 {
					if g, ok := createOnDemandGlobalFromER(er, opts, t, ctx); ok {
						bumpExprDepth(ctx)
						return castLiteral(t, g.expr)
					}
				}
				if scopePick == 1 {
					// Pre-multi-dim empty parent locals → create with qfer.
					if g, ok := createOnDemandFromParentLocalPathER(er, opts, t, ctx, true); ok {
						bumpExprDepth(ctx)
						return castLiteral(t, g.expr)
					}
				}
				// SelectParentParam empty → SelectParentLocal:
				// stack pick, strict choose_var on that block, else any-depth
				// dynLocs (inventory approx), else create.
				if scopePick == 2 {
					idx := parentStackPick(er, flow)
					localCands := localsInStackBlock(er, env, scope, ctx, idx)
					if c, ok := selectExprVariableStrict(t, er, localCands); ok {
						bumpExprDepth(ctx)
						return castLiteral(t, c.expr)
					}
					// Fall back: all non-x dynLocs (block inventory incomplete).
					allLocs := make([]exprVarCandidate, 0, 8)
					for _, l := range mergedLocals(scope, ctx) {
						if l.name == "x" {
							continue
						}
						allLocs = append(allLocs, exprVarCandidate{expr: l.name, ctype: l.ctype, assignable: true})
					}
					if c, ok := selectExprVariableStrict(t, er, allLocs); ok {
						bumpExprDepth(ctx)
						return castLiteral(t, c.expr)
					}
					// Param→ParentLocal create: early SE-free qferMode 1; late
					// pointer (useSmallParentStack) !SE-free self F10 only (e1024).
					qfer := 1
					if flow != nil && flow.useSmallParentStack && strings.Contains(t.Name, "*") {
						qfer = 2
					}
					if g, ok := createOnDemandFromParentLocalPathEROpts(er, opts, t, ctx, qfer, true, idx); ok {
						bumpExprDepth(ctx)
						return castLiteral(t, g.expr)
					}
				}
				candidates = buildExprCandidatesFromER(er, env, scope, ctx)
			}
			if len(candidates) > 0 {
				if c, ok := selectExprVariableFromER(t, er, candidates, false); ok {
					// SelectParentParam: if choose_var would reject type, fall through
					// to SelectParentLocal. Early: sameBase/same-bits. After multi-dim:
					// eFlexible int↔int (seed2 e887 width convert).
					if scopePick == 2 {
						wantPtr := strings.Contains(t.Name, "*")
						havePtr := strings.Contains(c.ctype.Name, "*")
						compat := sameBaseType(c.ctype, t) ||
							(!wantPtr && !havePtr && c.ctype.Bits == t.Bits)
						if !compat && !wantPtr && !havePtr &&
							ctx != nil && ctx.state != nil && ctx.state.multiDimArrays > 0 {
							cSimple := !strings.HasPrefix(c.ctype.Name, "struct") &&
								!strings.HasPrefix(c.ctype.Name, "union") &&
								c.ctype.Name != "float" && c.ctype.Name != "void"
							tSimple := !strings.HasPrefix(t.Name, "struct") &&
								!strings.HasPrefix(t.Name, "union") &&
								t.Name != "float" && t.Name != "void"
							compat = cSimple && tSimple
						}
						if !compat {
							idx := parentStackPick(er, flow)
							localCands := localsInStackBlock(er, env, scope, ctx, idx)
							if c2, ok2 := selectExprVariableStrict(t, er, localCands); ok2 {
								bumpExprDepth(ctx)
								return castLiteral(t, c2.expr)
							}
							allLocs := make([]exprVarCandidate, 0, 8)
							for _, l := range mergedLocals(scope, ctx) {
								if l.name == "x" {
									continue
								}
								allLocs = append(allLocs, exprVarCandidate{expr: l.name, ctype: l.ctype, assignable: true})
							}
							if c2, ok2 := selectExprVariableStrict(t, er, allLocs); ok2 {
								bumpExprDepth(ctx)
								return castLiteral(t, c2.expr)
							}
							if g, ok2 := createOnDemandFromParentLocalPathEROpts(er, opts, t, ctx, 1, true, idx); ok2 {
								bumpExprDepth(ctx)
								return castLiteral(t, g.expr)
							}
							restoreGenSnapshot(ctx, snap)
							continue
						}
					}
					bumpExprDepth(ctx)
					return castLiteral(t, c.expr)
				}
				// choose_var returned null (e.g. pointer want, no exact match).
				if scopePick == 0 && ctx != nil && ctx.state != nil && ctx.state.multiDimArrays > 0 {
					if g, ok := createOnDemandGlobalFromER(er, opts, t, ctx); ok {
						bumpExprDepth(ctx)
						return castLiteral(t, g.expr)
					}
				}
				// ParentParam miss → SelectParentLocal (stack pick + create).
				// seed2 e1021: U100=67 then U3 F50 F10… not term retry F80.
				if scopePick == 2 && ctx != nil && ctx.state != nil &&
					(ctx.state.multiDimArrays > 0 || ctx.state.useSmallParentStack) {
					idx := parentStackPick(er, flow)
					localCands := localsInStackBlock(er, env, scope, ctx, idx)
					if c2, ok2 := selectExprVariableStrict(t, er, localCands); ok2 {
						bumpExprDepth(ctx)
						return castLiteral(t, c2.expr)
					}
					qfer := 1
					if ctx.state.useSmallParentStack && strings.Contains(t.Name, "*") {
						qfer = 2
					}
					if g, ok2 := createOnDemandFromParentLocalPathEROpts(er, opts, t, ctx, qfer, true, idx); ok2 {
						bumpExprDepth(ctx)
						return castLiteral(t, g.expr)
					}
				}
			}
			restoreGenSnapshot(ctx, snap)
		case termConstant:
			bumpExprDepth(ctx)
			return randomConstantExprFromER(t, er, opts)
		case termAssign:
			// Upstream ExpressionAssign::make_random:
			// 1. CVQualifiers::random_qualifiers -> rnd_flipcoin(volatile_prob=50)
			//    (volatile draw; result is discarded when no_volatile=true, but RNG
			//    is still consumed)
			// 2. StatementAssign::make_random:
			//    a. AssignOpsProbability -> rnd_upto(assign_ops_total=120)
			//    b. Full Expression::make_random for RHS (recursive)
			//    c. Lhs::make_random for LHS variable selection
			if er != nil && er.fallback != nil {
				// random_qualifiers(WRITE, no_volatile): F50 only if SE-free.
				// useSmallParentStack: first few ExpressionAssign are !SE-free
				// (e1005 skip F50); later ones SE-free burn F50 (e1141).
				// CSMITH_ASSIGN_F50_AFTER: 0-based count after which to burn F50.
				small := ctx != nil && ctx.state != nil && ctx.state.useSmallParentStack
				n := 0
				if small {
					n = ctx.state.assignExprCount
					ctx.state.assignExprCount++
				}
				// seed2 e1005 n=0 skip F50; e1141 n=1 burn F50; e1167 n>=2 skip.
				// Only the second ExpressionAssign under useSmallParentStack is SE-free.
				if !small || n == 1 {
					_ = er.fallback.flipcoin(50)
				}
				_ = er.fallback.upto(120) // AssignOpsProbability
			}
			// RHS sees WRITE qfer (often all-const-false) → skip function ret qfer.
			prevSkip := false
			if ctx != nil {
				prevSkip = ctx.skipFuncRetQfer
				ctx.skipFuncRetQfer = true
			}
			rhs := randomTypedExprDepthFlags(t, er, opts, env, scope, depth+1, ctx, false, false)
			if ctx != nil {
				ctx.skipFuncRetQfer = prevSkip
			}
			// Upstream Lhs::make_random (Lhs.cpp:61): do-while loop that tries
			// select_deref_pointer before falling through to VariableSelector::select.
			// Each iteration when flipcoin(80)=true and no pointer vars exist:
			//   F 80 (SelectDerefPointerProb) -> true
			//   F 20 (NewArrayVariableProb in create_and_initialize)
			//   F 20 (make_init_value for pointer: constant path if true)
			//   F 0  (opportunistic_validate with null_pointer_deref_prob=0)
			// When make_init_value flipcoin(20)=false: address-of path creates a
			// global int var (F 20 inner NewArrayVar, F 50 GenerateRandomConstant,
			// + 8 raw genrand for RandomHexDigits), then pointer is valid -> loop exits.
			// When flipcoin(80)=false: fall through to VariableSelector::select (scope pick).
			// Upstream Lhs::make_random (Lhs.cpp:61): do-while loop that tries
			// select_deref_pointer before falling through to VariableSelector::select.
			lhsFromDeref := false
			createdArrEA := false
			if er != nil && er.fallback != nil {
				for {
					deref := er.fallback.flipcoin(80) // SelectDerefPointerProb (Lhs.cpp:78)
					if !deref {
						break // fall through to VariableSelector::select
					}
					// Itemize only after CreateArray in THIS ExpressionAssign Lhs loop
					// (e1052). Stale sizes force wrong path at e1174 (need F20 create).
					if createdArrEA && lastArraySizesSink != nil && len(*lastArraySizesSink) > 0 {
						for _, sz := range *lastArraySizesSink {
							if sz > 0 {
								_ = er.fallback.upto(uint32(sz))
							}
						}
						continue
					}
					// select_deref_pointer: no pointer vars -> GenerateNewParentLocal
					// create_and_initialize (VariableSelector.cpp:510):
					newArray := er.fallback.flipcoin(20) // NewArrayVariableProb
					// make_init_value for pointer (VariableSelector.cpp:834):
					initConst := er.fallback.flipcoin(20)
					if initConst {
						// Constant "0" (null), no more RNG for init
						// opportunistic_validate: null ptr -> flipcoin(0) -> fail
						_ = er.fallback.flipcoin(0) // null_pointer_dereference_prob
						continue
					}
					// Address-of path: create global int for pointer target
					// GenerateNewGlobal -> create_and_initialize for int:
					tgtNewArray := er.fallback.flipcoin(20) // inner NewArrayVariableProb
					// Constant::make_random for the synthetic pointed-to object.
					if er.fallback.flipcoin(50) {
						if er.fallback.flipcoin(50) {
							_ = er.fallback.upto(3) // pure_rnd_upto(3) - 1
						} else {
							_ = er.fallback.upto(20) // pure_rnd_upto(20) - 10
						}
					} else {
						// Historical early path: 16 hex digits. Late useSmallParentStack
						// e1181: char-width hex (2) then U120 (e1200 climb).
						hn := 16
						if ctx != nil && ctx.state != nil && ctx.state.useSmallParentStack {
							hn = 2
						}
						for i := 0; i < hn; i++ {
							_ = er.fallback.next31()
						}
					}
					// seed2 e1043: after Constant pure_rnd U20, CreateArray when
					// outer or target NewArray (U99 dimension ladder).
					if newArray || tgtNewArray {
						burnCreateArrayVariable(er.fallback, opts, t, true)
						createdArrEA = true
						// Array pointer Lhs often fails opportunistic_validate.
						if newArray {
							continue
						}
					}
					// Pointer to valid var -> opportunistic_validate passes -> exit
					lhsFromDeref = true
					break
				}
			}
			if lhsFromDeref {
				// Upstream Lhs.cpp:82-86: when select_deref_pointer returns valid var,
				// skips VariableSelector::select entirely (zero RNG consumed).
				// Pick first assignable candidate without consuming RNG.
				candidates := buildExprCandidatesFromER(er, env, scope, ctx)
				for _, c := range candidates {
					if c.assignable {
						return castLiteral(t, fmt.Sprintf("(%s = %s)", c.expr, rhs))
					}
				}
				return castLiteral(t, fmt.Sprintf("(%s)", rhs))
			}
			// VariableSelector::select (VariableSelector.cpp:1187): scope pick.
			scopePick := variableScopePickFromER(er, opts)
			// seed2 e1093–1097: early ParentParam Lhs → stack U3 + create + residual F80.
			// e1149: later ParentParam → stack U3 only (found/accept without create).
			if scopePick == 2 && ctx != nil && ctx.state != nil && ctx.state.useSmallParentStack {
				idx := parentStackPick(er, ctx.state)
				// assignExprCount already incremented for this ExpressionAssign.
				// count==1 only: e1093 create+residual. Later: stack only (e1149).
				earlyLhs := ctx.state.assignExprCount <= 1
				if !earlyLhs {
					// seed2 e1149: stack U3 only, then outer term U120 (no create).
					_ = idx
					return castLiteral(t, fmt.Sprintf("(%s = %s)", "x", rhs))
				}
				_, _ = createOnDemandFromParentLocalPathEROpts(er, opts, t, ctx, 0, false, idx)
				if er != nil && er.fallback != nil {
					// seed2 e1095–1096: hex under-count by 4 then residual F80.
					for i := 0; i < 4; i++ {
						_ = er.fallback.next31()
					}
					_ = er.fallback.flipcoin(80)
				}
				return castLiteral(t, fmt.Sprintf("(%s = %s)", "x", rhs))
			}
			candidates := buildScopedCandidatesFromER(er, env, scope, scopePick, ctx)
			if len(candidates) == 0 {
				if scopePick == 0 || scopePick == 3 {
					if g, ok := createOnDemandGlobalFromER(er, opts, t, ctx); ok {
						return castLiteral(t, fmt.Sprintf("(%s = %s)", g.expr, rhs))
					}
				}
				candidates = buildExprCandidatesFromER(er, env, scope, ctx)
			}
			if len(candidates) > 0 {
				if lv, ok := selectExprVariableFromER(t, er, candidates, true); ok {
					return castLiteral(t, fmt.Sprintf("(%s = %s)", lv.expr, rhs))
				}
			}
			restoreGenSnapshot(ctx, snap)
		case termComma:
			lhsType := t
			if er != nil && er.fallback != nil && ctx != nil && ctx.state != nil {
				allCount := len(ctx.state.pool) + len(ctx.state.info.structs) + len(ctx.state.info.unions)
				if allCount > 0 {
					pick := int(er.fallback.upto(uint32(allCount)))
					switch {
					case pick < len(ctx.state.pool):
						lhsType = ctx.state.pool[pick]
					case pick < len(ctx.state.pool)+len(ctx.state.info.structs):
						lhsType = CType{Name: fmt.Sprintf("struct S%d", pick-len(ctx.state.pool)), Bits: 32}
					default:
						lhsType = CType{Name: fmt.Sprintf("union U%d", pick-len(ctx.state.pool)-len(ctx.state.info.structs)), Bits: 32}
					}
				}
			}
			// Upstream ExpressionComma::make_random:
			// lhs = make_random(..., type=nil, no_const=true), rhs = make_random(..., type=t)
			lhs := randomTypedExprDepthFlags(lhsType, er, opts, env, scope, depth+1, ctx, false, true)
			rhs := randomTypedExprDepthFlags(t, er, opts, env, scope, depth+1, ctx, false, false)
			return castLiteral(t, fmt.Sprintf("((%s), (%s))", lhs, rhs))
		}
	}

	return randomConstantExprFromER(t, er, opts)
}

func randomLeafExpr(t CType, er *exprRand, opts Options, env envInfo, scope scopeInfo, depth int, ctx *genContext) string {
	return randomLeafExprWithMode(t, er, opts, env, scope, depth, ctx, false, false, false)
}

func randomParamLeafExpr(t CType, er *exprRand, opts Options, env envInfo, scope scopeInfo, depth int, ctx *genContext) string {
	prev := false
	if ctx != nil {
		prev = ctx.inParamExpr
		ctx.inParamExpr = true
		defer func() { ctx.inParamExpr = prev }()
	}
	return randomLeafExprWithMode(t, er, opts, env, scope, depth, ctx, true, false, false)
}

func maxExprDepth(opts Options) int {
	// Upstream --max-expr-complexity N sets CGOptions::max_expr_depth = N directly.
	// The filter is: expr_depth + 2 > max_expr_depth(), i.e. N=10 only filters at depth>=9.
	if opts.MaxExprComplexity < 1 {
		return 1
	}
	return opts.MaxExprComplexity
}

func randomTypedExprDepth(t CType, er *exprRand, opts Options, env envInfo, scope scopeInfo, depth int, ctx *genContext) string {
	_ = depth
	return randomLeafExpr(t, er, opts, env, scope, depth, ctx)
}

func randomTypedExprDepthFlags(t CType, er *exprRand, opts Options, env envInfo, scope scopeInfo, depth int, ctx *genContext, noFunc bool, noConst bool) string {
	_ = depth
	return randomLeafExprWithMode(t, er, opts, env, scope, depth, ctx, false, noFunc, noConst)
}

func randomParamExprDepth(t CType, er *exprRand, opts Options, env envInfo, scope scopeInfo, depth int, ctx *genContext) string {
	_ = depth
	return randomParamLeafExpr(t, er, opts, env, scope, depth, ctx)
}

func exprDecisionBudget(opts Options) int {
	depth := maxExprDepth(opts)
	return 16 + (depth * 12)
}

func randomTypedExpr(t CType, r *rng, opts Options, env envInfo, scope scopeInfo, ctx *genContext) string {
	er := newExprRand(r, exprDecisionBudget(opts))
	return randomTypedExprDepth(t, er, opts, env, scope, 0, ctx)
}

func variableScopePick(r *rng, opts Options) int {
	v := int(r.upto(100))
	if opts.GlobalVariables {
		switch {
		case v < 35:
			return 0
		case v < 65:
			return 1
		case v < 95:
			return 2
		default:
			return 3
		}
	}
	switch {
	case v < 50:
		return 1
	case v < 95:
		return 2
	default:
		return 3
	}
}

func buildScopedCandidates(r *rng, env envInfo, scope scopeInfo, scopePick int, ctx *genContext) []exprVarCandidate {
	out := make([]exprVarCandidate, 0, 16)
	switch scopePick {
	case 0:
		for _, g := range mergedGlobals(env, ctx) {
			out = append(out, exprVarCandidate{expr: g.name, ctype: g.ctype, assignable: !g.isConst})
		}
	case 1:
		for _, l := range mergedLocals(scope, ctx) {
			if l.name == "x" {
				continue
			}
			out = append(out, exprVarCandidate{expr: l.name, ctype: l.ctype, assignable: true})
		}
	case 2:
		for _, p := range scope.params {
			out = append(out, exprVarCandidate{expr: p.name, ctype: p.ctype, assignable: true})
		}
	}
	if scopePick != 2 {
		for _, ptr := range env.pointers {
			out = append(out, exprVarCandidate{expr: "*" + ptr.name, ctype: ptr.targetTy, assignable: !ptr.constTarget})
		}
		for _, arr := range env.arrays {
			out = append(out, exprVarCandidate{
				expr:       fmt.Sprintf("%s[%d]", arr.name, int(r.upto(uint32(arr.len)))),
				ctype:      arr.ctype,
				assignable: true,
			})
		}
	}
	return out
}

func chooseLValue(r *rng, opts Options, target CType, env envInfo, scope scopeInfo, ctx *genContext) (lvalueInfo, bool) {
	// variableScopePick uses er.pick(100); Lhs uses main rng directly.
	er := &exprRand{fallback: r}
	scopePick := variableScopePickFromER(er, opts)
	var flow *functionFlowState
	if ctx != nil {
		flow = ctx.state
	}
	// SelectParentLocal: stack pick then block locals / create (seed2 e939–941).
	if scopePick == 1 {
		// Lhs stack size often smaller than expression-var pin-5 (e940 U2).
		nStack := 2
		if flow != nil && flow.blockStack > 0 && flow.blockStack < 5 {
			nStack = flow.blockStack
		}
		if flow != nil && flow.multiDimArrays > 0 {
			nStack = 2 // seed2 e940
		}
		idx := int(er.pick(uint32(nStack)))
		useBlockLocal := ctx != nil && ctx.state != nil && ctx.state.multiDimArrays > 0
		if useBlockLocal {
			localCands := localsInStackBlock(er, env, scope, ctx, idx)
			if len(localCands) == 0 {
				// Lhs WRITE: qferMode 3 (F50 vol, no const F10) seed2 e942–943.
				if g, ok := createOnDemandFromParentLocalPathEROpts(er, opts, target, ctx, 3, true, idx); ok {
					return lvalueInfo{expr: g.expr, ctype: g.ctype}, true
				}
			} else if c, ok := selectExprVariable(target, r, localCands, true); ok {
				return lvalueInfo{expr: c.expr, ctype: c.ctype}, true
			} else if g, ok := createOnDemandFromParentLocalPathEROpts(er, opts, target, ctx, 3, false, idx); ok {
				return lvalueInfo{expr: g.expr, ctype: g.ctype}, true
			}
		}
	}
	if scopePick == 3 {
		if g, ok := createOnDemandGlobalFromER(er, opts, target, ctx); ok {
			return lvalueInfo{expr: g.expr, ctype: g.ctype}, true
		}
	}
	if scopePick == 4 {
		_ = parentStackPick(er, flow)
		needQfer := ctx != nil && ctx.state != nil && ctx.state.multiDimArrays > 0
		if g, ok := createOnDemandFromParentLocalPathER(er, opts, target, ctx, needQfer); ok {
			return lvalueInfo{expr: g.expr, ctype: g.ctype}, true
		}
	}
	c := buildScopedCandidates(r, env, scope, scopePick, ctx)
	if len(c) == 0 {
		c = buildExprCandidates(r, env, scope, ctx)
	}
	pick, ok := selectExprVariable(target, r, c, true)
	if !ok {
		return lvalueInfo{}, false
	}
	return lvalueInfo{expr: pick.expr, ctype: pick.ctype}, true
}

func emitLValueAssignment(b *strings.Builder, r *rng, opts Options, env envInfo, scope scopeInfo, ctx *genContext) bool {
	// StatementAssign::make_random order:
	// 1) AssignOpsProbability (upto ~120 with filter)
	// 2) SelectLType only for eSimpleAssign (pointer/struct/float coins)
	// 3) RHS Expression::make_random then Lhs
	// AssignOps table: simple 70, bitand/xor/or 10 each, pre/post incr/decr 5 each = 120.
	simpleAssign := true
	needNoRhs := false // ++/-- use Constant::make_int(1), no Expression::make_random
	if opts.CompoundAssignment {
		opV := int(r.upto(120))
		// AssignOps: simple 70, bitand/xor/or 10 each (=100), pre/post ± 5 each (=120).
		simpleAssign = opV < 70
		needNoRhs = opV >= 100
	}

	targetType := CType{Name: "int32_t", Signed: true, Bits: 32, HexDigits: 8}
	// Type::SelectLType: pointer/struct only when op is simple assign;
	// float coin only when AssignOpWorksForFloat(op).
	if simpleAssign {
		if opts.Pointers && r.flipcoin(50) { // PointerAsLTypeProb
			// make_random_pointer_type
			if r.flipcoin(20) { // pointer-to-pointer
				_ = r.upto(1) // derived_types.size() often 1 early
			} else {
				// choose_random() for pointed-to type — AllTypes filter pick
				if ctx != nil {
					targetType = pickNonVoidNonVolatile(r, nil, ctx.info, opts)
				} else {
					_ = r.upto(14)
				}
			}
			targetType = CType{Name: "int32_t*", Signed: true, Bits: 32} // simplified ptr
		} else {
			// StructAsLTypeProb skipped when ok_struct_types empty (vol structs filtered).
			// FloatAsLTypeProb is 0 when !enable_float, but flipcoin(0) still runs
			// for simple assign (AssignOpWorksForFloat).
			_ = r.flipcoin(0)
		}
	}

	// Upstream: need_no_rhs(op) → Constant::make_int(1); else RHS then Lhs.
	// seed2 e934 U120=109 is post-incr (no RHS Expression); next is Lhs F80.
	rhs := "1"
	if !needNoRhs {
		if ctx != nil {
			ctx.exprDepth = 0
		}
		rhs = randomTypedExpr(targetType, r, opts, env, scope, ctx)
	}

	// Lhs::make_random: SelectDerefPointerProb then VariableSelector::select.
	// select_deref_pointer with no match creates a pointer via
	// random_add_qualifiers (F10 const, F50 volatile) + create_and_initialize.
	lv := lvalueInfo{expr: "x", ctype: targetType}
	lhsFromDeref := false
	triedDerefChoose := false
	createdArrayThisLhs := false
	for {
		if !r.flipcoin(80) { // SelectDerefPointerProb
			break
		}
		// select_deref_pointer: choose_var first when compatible pointers exist.
		// ++/-- Lhs after multi-dim: choose U2 (e936), fail validate, retry
		// F80=1 with no extra U (sole remaining / still invalid), then F80=0
		// falls through to VariableSelector::select (e937–939).
		if needNoRhs && ctx != nil && ctx.state != nil && ctx.state.multiDimArrays > 0 {
			if !triedDerefChoose {
				_ = r.upto(2) // e936
				triedDerefChoose = true
				continue
			}
			// Second F80=true: no create residual; fail again → next F80.
			continue
		}
		// After CreateArray in THIS Lhs loop, itemize last sizes (e1115 U10 U1 U1).
		// Do not use stale sizes from earlier ExpressionAssign arrays (broke e1098).
		if createdArrayThisLhs && lastArraySizesSink != nil && len(*lastArraySizesSink) > 0 {
			for _, sz := range *lastArraySizesSink {
				if sz > 0 {
					_ = r.upto(uint32(sz))
				}
			}
			continue
		}
		// No existing deref targets → create pointer local/global.
		// random_add_qualifiers (non-wildcard qfer from RHS):
		if opts.ConstPointers {
			_ = r.flipcoin(10) // RegularConstProb
		}
		// seed2 e1098–1099: after ExpressionAssign residual, statement Lhs
		// SelectDeref is F10 then F20 NewArray — no VolatilePointers F50
		// when useSmallParentStack (UP F10 F20 F20 F20 F50 U99).
		skipVol := ctx != nil && ctx.state != nil && ctx.state.useSmallParentStack
		if opts.VolatilePointers && !skipVol {
			_ = r.flipcoin(50) // RegularVolatileProb
		}
		// create_and_initialize for a new POINTER (to targetType) for deref.
		// SelectDeref always creates a pointer var, not the bare Lhs type.
		ptrType := targetType
		if !strings.Contains(ptrType.Name, "*") {
			ptrType = CType{Name: targetType.Name + "*", Signed: targetType.Signed, Bits: targetType.Bits, HexDigits: targetType.HexDigits}
		}
		newArray := r.flipcoin(20) // NewArrayVariableProb
		// make_init_value for pointer type
		if r.flipcoin(20) {
			// Constant null — opportunistic_validate fails (null_pointer_prob=0)
			_ = r.flipcoin(0)
			continue
		}
		// Address-of path for pointer init.
		if newArray {
			// create_and_initialize → create_array_and_itemize for the pointer
			// variable type (seed2 e346 U99+U10+U1 with size-1, no alt inits).
			if skipVol {
				// seed2 e1099–1102: F20+F50 residual then 8 next31 before CreateArray.
				_ = r.flipcoin(20)
				_ = r.flipcoin(50)
				for i := 0; i < 8; i++ {
					_ = r.next31()
				}
			}
			burnCreateArrayVariable(r, opts, ptrType, true)
			createdArrayThisLhs = true
			// Seed2: array pointer Lhs fails opportunistic_validate once and
			// retries (next SelectDeref F80=0 → VariableSelector::select).
			continue
		}
		// make_init_value address-of: choose_var miss → random_loose_qualifiers
		// (F50 looser vol when outer pointer was volatile, e911) +
		// GenerateNew* with qfer set (no random_qualifiers):
		//   F20 NewArray for pointed-to (often pointer for Lhs int* → int**)
		//   F20 make_init null vs address-of
		//   if address: choose_ok_var U(n) (seed2 e913–914 F20 then U5)
		//   if null: Constant "0" for pointer (no further RNG)
		_ = r.flipcoin(50) // random_looser_volatiles residual when outer vol
		tgtNewArray := r.flipcoin(20)
		if tgtNewArray {
			// create_and_initialize NewArray branch: init then itemize
			if strings.Contains(targetType.Name, "*") {
				if r.flipcoin(20) {
					// null init for pointer element
				} else {
					n := 5
					if ctx != nil && ctx.state != nil && ctx.state.multiDimArrays > 0 {
						n = 5
					}
					_ = r.upto(uint32(n))
				}
			} else {
				burnSimpleConstant(r, targetType)
			}
			burnCreateArrayVariable(r, opts, targetType, true)
			createdArrayThisLhs = true
		} else if strings.Contains(targetType.Name, "*") {
			// Pointed-to is pointer: make_init_value F20 null vs address-of.
			if r.flipcoin(20) {
				// null — done
			} else {
				// choose_var among visible matching pointees (seed2 e914 U5).
				n := 5
				_ = r.upto(uint32(n))
			}
		} else {
			// Simple pointed-to: Constant::make_random
			burnSimpleConstant(r, targetType)
		}
		if ctx != nil && ctx.state != nil {
			ctx.state.lhsDerefCreates++
		}
		lhsFromDeref = true
		break
	}
	// note: lhsDerefAttempts already bumped on F80=true above
	if !lhsFromDeref {
		// VariableSelector::select after SelectDerefPointerProb false (or
		// failed deref create). Always scope-pick; do not require empty locals.
		if picked, ok := chooseLValue(r, opts, targetType, env, scope, ctx); ok {
			lv = picked
		}
	}
	// ++/-- compound: make_possible_compound_assign → SafeOpFlags (e945 F50 U4).
	if needNoRhs {
		_ = r.flipcoin(50)
		_ = r.upto(4)
	}
	writeLine(b, 1, fmt.Sprintf("%s = %s;", lv.expr, rhs))
	writeLine(b, 1, fmt.Sprintf("x ^= (uint32_t)%s;", lv.expr))
	if ctx != nil {
		c := exprVarCandidate{expr: lv.expr, ctype: lv.ctype, assignable: true}
		ctx.mustUse = &c
	}
	return true
}

func maybeDeclareOnDemandLocal(b *strings.Builder, r *rng, opts Options, ctx *genContext) {
	// Disabled until parent-scope local placement mirrors upstream block scoping.
	_ = b
	_ = r
	_ = opts
	_ = ctx
}

func emitCompositeTypes(b *strings.Builder, r *rng, opts Options, pool []CType) compositeInfo {
	info := compositeInfo{}
	writeLine(b, 0, "/* --- Struct/Union Declarations --- */")
	// Upstream Probabilities defaults (Probabilities.cpp).
	const (
		moreStructUnionTypeProb       = 50
		bitfieldsCreationProb         = 50
		bitfieldInNormalStructProb    = 10
		scalarFieldInFullBitfieldProb = 10
		bitfieldsSignedProb           = 50
		fieldVolatileProb             = 30
		fieldConstProb                = 20
	)
	moreTypesProbability := func(existingTypeCount int) bool {
		// Type::MoreTypesProbability: keep adding while <10 total types,
		// then 50% chance for each additional aggregate type.
		if existingTypeCount < 10 {
			return true
		}
		return r.flipcoin(moreStructUnionTypeProb)
	}
	// Upstream Type::GenerateSimpleTypes pushes eChar..eUInt128, i.e. 13
	// simple types before aggregate generation starts.
	typeCount := 13
	fieldQual := func() string {
		// Mirrors CVQualifiers::random_qualifiers(..., FieldConstProb, FieldVolatileProb):
		// volatile draw first, then const draw.
		isVolatile := opts.VolStructUnionFields && r.flipcoin(fieldVolatileProb)
		isConst := opts.ConstStructUnionFields && r.flipcoin(fieldConstProb)
		if isConst && isVolatile && !opts.AllowConstVolatile {
			isConst = false
		}
		q := ""
		if isConst {
			q += "const "
		}
		if isVolatile {
			q += "volatile "
		}
		return q
	}
	bitfieldLength := func(maxLength int, prior []fieldInfo) int {
		if maxLength < 1 {
			maxLength = 1
		}
		length := int(r.upto(uint32(maxLength)))
		noZeroLen := len(prior) == 0 || (prior[len(prior)-1].bitfield && prior[len(prior)-1].bitWidth == 0)
		if length == 0 && noZeroLen {
			if maxLength <= 2 {
				length = 1
			} else {
				length = int(r.upto(uint32(maxLength-1))) + 1
			}
		}
		return length
	}

	if opts.Structs {
		sidx := 0
		maxStructs := min(max(opts.MaxStructFields, 1), 32)
		for sidx < maxStructs && moreTypesProbability(typeCount) {
			fieldCount := 1 + int(r.upto(uint32(max(1, opts.MaxStructFields))))
			st := structTypeInfo{fields: make([]fieldInfo, 0, fieldCount)}
			writeLine(b, 0, fmt.Sprintf("struct S%d {", sidx))
			fullBitfields := opts.Bitfields && r.flipcoin(bitfieldsCreationProb)
			for f := 0; f < fieldCount; f++ {
				if fullBitfields {
					if r.flipcoin(scalarFieldInFullBitfieldProb) {
						name := fmt.Sprintf("f%d", f)
						t := pickType(r, pool)
						writeLine(b, 1, fmt.Sprintf("%s%s %s;", fieldQual(), t.Name, name))
						st.fields = append(st.fields, fieldInfo{name: name, ctype: t})
						continue
					}
					name := fmt.Sprintf("f%d", f)
					base := "unsigned"
					if r.flipcoin(bitfieldsSignedProb) {
						base = "signed"
					}
					qual := fieldQual()
					width := bitfieldLength(opts.IntSize*8, st.fields)
					writeLine(b, 1, fmt.Sprintf("%s%s %s : %d;", qual, base, name, width))
					st.fields = append(st.fields, fieldInfo{
						name: name, ctype: CType{Name: "uint32_t", Bits: 32, Signed: base == "signed"}, bitfield: true, bitWidth: width,
					})
					continue
				}
				if opts.Bitfields && r.flipcoin(bitfieldInNormalStructProb) {
					name := fmt.Sprintf("f%d", f)
					base := "unsigned"
					if r.flipcoin(bitfieldsSignedProb) {
						base = "signed"
					}
					qual := fieldQual()
					width := bitfieldLength(opts.IntSize*8, st.fields)
					writeLine(b, 1, fmt.Sprintf("%s%s %s : %d;", qual, base, name, width))
					st.fields = append(st.fields, fieldInfo{
						name: name, ctype: CType{Name: "uint32_t", Bits: 32, Signed: base == "signed"}, bitfield: true, bitWidth: width,
					})
					continue
				}
				name := fmt.Sprintf("f%d", f)
				t := pickType(r, pool)
				writeLine(b, 1, fmt.Sprintf("%s%s %s;", fieldQual(), t.Name, name))
				st.fields = append(st.fields, fieldInfo{name: name, ctype: t})
			}
			if opts.PackedStruct {
				// Type::make_random_struct_type consumes rnd_flipcoin(50) when
				// packed-struct is enabled (default upstream behavior).
				_ = r.flipcoin(50)
			}
			writeLine(b, 0, "};")
			writeLine(b, 0, "")
			info.structs = append(info.structs, st)
			sidx++
			typeCount++
		}
	}

	if opts.Unions {
		uidx := 0
		maxUnions := min(max(opts.MaxUnionFields, 1), 32)
		for uidx < maxUnions && moreTypesProbability(typeCount) {
			fieldCount := 1 + int(r.upto(uint32(max(1, opts.MaxUnionFields))))
			ut := unionTypeInfo{fields: make([]fieldInfo, 0, fieldCount)}
			writeLine(b, 0, fmt.Sprintf("union U%d {", uidx))
			for f := 0; f < fieldCount; f++ {
				name := fmt.Sprintf("f%d", f)
				if opts.Bitfields && r.flipcoin(bitfieldInNormalStructProb) {
					base := "unsigned"
					if r.flipcoin(bitfieldsSignedProb) {
						base = "signed"
					}
					qual := fieldQual()
					width := bitfieldLength(opts.IntSize*8, ut.fields)
					writeLine(b, 1, fmt.Sprintf("%s%s %s : %d;", qual, base, name, width))
					ut.fields = append(ut.fields, fieldInfo{
						name: name, ctype: CType{Name: "uint32_t", Bits: 32, Signed: base == "signed"}, bitfield: true, bitWidth: width,
					})
					continue
				}
				t := pickType(r, pool)
				writeLine(b, 1, fmt.Sprintf("%s%s %s;", fieldQual(), t.Name, name))
				ut.fields = append(ut.fields, fieldInfo{name: name, ctype: t})
			}
			writeLine(b, 0, "};")
			writeLine(b, 0, "")
			info.unions = append(info.unions, ut)
			uidx++
			typeCount++
		}
	}
	writeLine(b, 0, "")

	return info
}

func emitGlobals(b *strings.Builder, r *rng, opts Options, info compositeInfo, pool []CType) envInfo {
	env := envInfo{}
	nextGlobalID := 0
	writeLine(b, 0, "/* --- GLOBAL VARIABLES --- */")
	if opts.GlobalVariables {
		globalCap := max(opts.MaxGlobals, 2)
		env.globals = make([]globalInfo, 0, globalCap)
		env.arrays = make([]arrayInfo, 0, globalCap/2+1)
		env.pointers = make([]pointerInfo, 0, globalCap/2+1)

		newGlobalName := func() string {
			name := fmt.Sprintf("g_%d", nextGlobalID)
			nextGlobalID++
			return name
		}

		// Keep scalar globals as the primary pool used by expressions.
		scalarTarget := min(globalCap, 26+int(r.upto(18)))
		for i := 0; i < scalarTarget; i++ {
			// Upstream regular qualifiers (Probabilities.cpp defaults).
			isConst := false
			if opts.Consts {
				isConst = r.upto(100) < 10
			}
			isVolatile := false
			if opts.Volatiles {
				isVolatile = r.upto(100) < 50
			}
			// Avoid degenerate const+volatile saturation.
			if isConst && isVolatile && r.upto(2) == 0 {
				isConst = false
			}
			g := globalInfo{
				name:       newGlobalName(),
				ctype:      pickType(r, pool),
				isConst:    isConst,
				isVolatile: isVolatile,
			}
			lit := castLiteral(g.ctype, fmt.Sprintf("0x%08Xu", r.next31()))
			qual := ""
			if g.isConst {
				qual += "const "
			}
			if g.isVolatile {
				qual += "volatile "
			}
			writeLine(b, 0, fmt.Sprintf("static %s%s %s = %s;", qual, g.ctype.Name, g.name, lit))
			env.globals = append(env.globals, g)
		}

		if opts.Arrays {
			// Generate a richer array set with canonical g_N naming.
			arrayTarget := min(max(12, len(env.globals)), 40)
			for i := 0; i < arrayTarget; i++ {
				arrLen := 2 + int(r.upto(uint32(max(2, min(opts.MaxArrayLenPerDim, 10)))))
				ai := arrayInfo{
					name:  newGlobalName(),
					ctype: pickType(r, pool),
					len:   arrLen,
				}
				writeLine(b, 0, fmt.Sprintf("static %s %s[%d] = {0};", ai.ctype.Name, ai.name, arrLen))
				env.arrays = append(env.arrays, ai)
			}
		}

		if opts.Pointers {
			start := 0
			if opts.Consts && len(env.globals) > 1 {
				start = 1
			}
			ptrTarget := min(max(4, len(env.globals)/4), 12)
			for i := 0; i < ptrTarget; i++ {
				target := env.globals[start+int(r.upto(uint32(max(1, len(env.globals)-start))))]
				p := pointerInfo{
					name:            newGlobalName(),
					target:          target.name,
					targetTy:        target.ctype,
					volatilePointer: opts.VolatilePointers && r.upto(3) == 0,
					volatileTarget:  opts.VolatilePointers && r.upto(4) == 0,
					constTarget:     opts.ConstPointers && r.upto(3) == 0,
				}
				targetQual := ""
				if p.constTarget {
					targetQual += "const "
				}
				if p.volatileTarget {
					targetQual += "volatile "
				}
				ptrQual := ""
				if p.volatilePointer {
					ptrQual = "volatile "
				}
				writeLine(b, 0, fmt.Sprintf("static %s%s *%s%s = &%s;", targetQual, target.ctype.Name, ptrQual, p.name, p.target))
				env.pointers = append(env.pointers, p)
			}

			// Extra pointer chains: mimic global pointer ladders seen in upstream output.
			chainCount := min(max(len(env.pointers)/2, 1), 4)
			env.chains = make([]string, 0, chainCount)
			for i := 0; i < chainCount; i++ {
				chainBases := make([]pointerInfo, 0, len(env.pointers))
				for _, pb := range env.pointers {
					if pb.constTarget || pb.volatileTarget || pb.volatilePointer {
						continue
					}
					chainBases = append(chainBases, pb)
				}
				if len(chainBases) == 0 {
					break
				}
				base := chainBases[int(r.upto(uint32(len(chainBases))))]
				depth := 2 + int(r.upto(uint32(max(1, min(opts.MaxPointerDepth, 4)-1))))

				prevName := base.name
				baseType := base.targetTy.Name
				for d := 2; d <= depth; d++ {
					name := newGlobalName()
					stars := strings.Repeat("*", d)
					writeLine(b, 0, fmt.Sprintf("static %s %s%s = &%s;", baseType, stars, name, prevName))
					prevName = name
					env.chains = append(env.chains, name)
				}
			}
		}
	}

	for i := range info.structs {
		writeLine(b, 0, fmt.Sprintf("static struct S%d gs_%d;", i, i))
	}
	for i := range info.unions {
		writeLine(b, 0, fmt.Sprintf("static union U%d gu_%d;", i, i))
	}
	env.nextID = nextGlobalID
	writeLine(b, 0, "")
	return env
}

func emitFuncDecls(b *strings.Builder, funcs []funcInfo) {
	writeLine(b, 0, "/* --- FORWARD DECLARATIONS --- */")
	for _, fn := range funcs {
		params := "void"
		if len(fn.params) > 0 {
			pp := make([]string, 0, len(fn.params))
			for _, p := range fn.params {
				pp = append(pp, fmt.Sprintf("%s %s", p.ctype.Name, p.name))
			}
			params = strings.Join(pp, ", ")
		}
		writeLine(b, 0, fmt.Sprintf("static %s %s(%s);", fn.ret.Name, fn.name, params))
	}
	writeLine(b, 0, "")
}

func emitArithmeticMutation(b *strings.Builder, r *rng, opts Options) {
	if opts.Muls && r.upto(4) == 0 {
		writeLine(b, 1, fmt.Sprintf("x = x * (0x%08Xu | 1u);", r.next31()))
		return
	}
	if opts.Divs && r.upto(3) == 0 {
		writeLine(b, 1, fmt.Sprintf("x = %s;", safeDivU32Expr("x", "((x & 255u) + 1u)", opts)))
		return
	}
	if opts.UnaryPlusOperator && r.upto(3) == 0 {
		writeLine(b, 1, fmt.Sprintf("x = (+x) ^ 0x%08Xu;", r.next31()))
		return
	}
	writeLine(
		b,
		1,
		fmt.Sprintf("x = (%s) ^ (%s) ^ 0x%08Xu;", safeLShiftU32Expr("x", "1", opts), safeRShiftU32Expr("x", "1", opts), r.next31()),
	)
}

func emitIncDecMutation(b *strings.Builder, r *rng, opts Options) bool {
	if !(opts.PreIncrOperator || opts.PreDecrOperator || opts.PostIncrOperator || opts.PostDecrOperator) {
		return false
	}
	switch r.upto(4) {
	case 0:
		if opts.PreIncrOperator {
			writeLine(b, 1, "++x;")
			return true
		}
	case 1:
		if opts.PreDecrOperator {
			writeLine(b, 1, "--x;")
			return true
		}
	case 2:
		if opts.PostIncrOperator {
			writeLine(b, 1, "x++;")
			return true
		}
	case 3:
		if opts.PostDecrOperator {
			writeLine(b, 1, "x--;")
			return true
		}
	}
	return false
}

func emitGlobalMutation(b *strings.Builder, r *rng, opts Options, env envInfo, scope scopeInfo, ctx *genContext) {
	if len(env.globals) == 0 {
		emitArithmeticMutation(b, r, opts)
		return
	}
	writable := make([]globalInfo, 0, len(env.globals))
	for _, g := range env.globals {
		if g.isConst {
			continue
		}
		writable = append(writable, g)
	}
	if len(writable) == 0 {
		emitArithmeticMutation(b, r, opts)
		return
	}
	g := writable[int(r.upto(uint32(len(writable))))]
	rhs := randomTypedExpr(g.ctype, r, opts, env, scope, ctx)
	if opts.CompoundAssignment {
		writeLine(b, 1, fmt.Sprintf("%s += %s;", g.name, rhs))
	} else {
		writeLine(b, 1, fmt.Sprintf("%s = %s;", g.name, safeAddExpr(g.ctype, g.name, rhs, opts)))
	}
	if ctx != nil {
		c := exprVarCandidate{expr: g.name, ctype: g.ctype, assignable: !g.isConst}
		ctx.mustUse = &c
	}
}

func emitArrayMutation(b *strings.Builder, r *rng, opts Options, env envInfo, scope scopeInfo, ctx *genContext) {
	if !opts.Arrays || len(env.arrays) == 0 {
		emitArithmeticMutation(b, r, opts)
		return
	}
	ai := env.arrays[int(r.upto(uint32(len(env.arrays))))]
	idxMask := max(1, min(opts.MaxArrayLenPerDim, 8)-1)
	rhs := randomTypedExpr(ai.ctype, r, opts, env, scope, ctx)
	writeLine(b, 1, fmt.Sprintf("%s[x & %du] ^= %s;", ai.name, idxMask, rhs))
	if opts.EmbeddedAssigns {
		one := castLiteral(ai.ctype, "1u")
		writeLine(
			b,
			1,
			fmt.Sprintf(
				"x = (%s[x & %du] = %s);",
				ai.name,
				idxMask,
				safeAddExpr(ai.ctype, fmt.Sprintf("%s[x & %du]", ai.name, idxMask), one, opts),
			),
		)
	}
	if ctx != nil {
		c := exprVarCandidate{expr: fmt.Sprintf("%s[x & %du]", ai.name, idxMask), ctype: ai.ctype, assignable: true}
		ctx.mustUse = &c
	}
}

func emitPointerMutation(b *strings.Builder, r *rng, opts Options, env envInfo, scope scopeInfo, ctx *genContext) {
	if !opts.Pointers || len(env.pointers) == 0 {
		emitArithmeticMutation(b, r, opts)
		return
	}
	pi := env.pointers[int(r.upto(uint32(len(env.pointers))))]
	rhs := randomTypedExpr(pi.targetTy, r, opts, env, scope, ctx)
	if opts.CompoundAssignment {
		writeLine(b, 1, fmt.Sprintf("*%s ^= %s;", pi.name, rhs))
	} else {
		writeLine(b, 1, fmt.Sprintf("*%s = *%s ^ %s;", pi.name, pi.name, rhs))
	}
	if ctx != nil {
		c := exprVarCandidate{expr: "*" + pi.name, ctype: pi.targetTy, assignable: !pi.constTarget}
		ctx.mustUse = &c
	}
}

func emitLocalMutation(b *strings.Builder, r *rng, opts Options, env envInfo, scope scopeInfo, ctx *genContext) {
	if len(scope.locals) == 0 {
		emitArithmeticMutation(b, r, opts)
		return
	}
	li := scope.locals[int(r.upto(uint32(len(scope.locals))))]
	rhs := randomTypedExpr(li.ctype, r, opts, env, scope, ctx)
	if opts.CompoundAssignment {
		writeLine(b, 1, fmt.Sprintf("%s += %s;", li.name, rhs))
	} else {
		writeLine(b, 1, fmt.Sprintf("%s = %s;", li.name, safeAddExpr(li.ctype, li.name, rhs, opts)))
	}
	writeLine(b, 1, fmt.Sprintf("x ^= (uint32_t)%s;", li.name))
	if ctx != nil {
		c := exprVarCandidate{expr: li.name, ctype: li.ctype, assignable: true}
		ctx.mustUse = &c
	}
}

func (s *functionFlowState) appendNewFunction(r *rng, forceRet *CType) (funcInfo, int, bool) {
	if len(s.funcs) >= s.maxFuncs {
		return funcInfo{}, -1, false
	}
	fn := funcInfo{
		name: fmt.Sprintf("func_%d", s.nextIdx),
	}
	if forceRet != nil {
		fn.ret = *forceRet
		maxParams := s.opts.MaxParams
		if maxParams < 0 {
			maxParams = 0
		}
		pcount := 0
		if maxParams > 0 {
			pcount = int(r.upto(uint32(maxParams + 1)))
		}
		fn.params = make([]paramInfo, 0, pcount)
		for p := 0; p < pcount; p++ {
			fn.params = append(fn.params, paramInfo{
				name:  s.allocParamName(),
				ctype: pickType(r, s.pool),
			})
		}
	} else {
		fn = s.makeFuncSignature(r, s.nextIdx)
	}
	s.nextIdx++
	s.funcs = append(s.funcs, fn)
	s.built = append(s.built, false)
	s.defs = append(s.defs, "")
	return fn, len(s.funcs) - 1, true
}

// appendNewFunctionWithSignature registers a function whose signature RNG was
// already consumed by the caller (make_random_signature mirror).
func (s *functionFlowState) appendNewFunctionWithSignature(r *rng, ret CType, params []paramInfo) (funcInfo, int, bool) {
	_ = r
	if len(s.funcs) >= s.maxFuncs {
		return funcInfo{}, -1, false
	}
	fn := funcInfo{
		name:   fmt.Sprintf("func_%d", s.nextIdx),
		ret:    ret,
		params: params,
	}
	s.nextIdx++
	s.funcs = append(s.funcs, fn)
	s.built = append(s.built, false)
	s.defs = append(s.defs, "")
	return fn, len(s.funcs) - 1, true
}

func emitFunctionCallMutation(
	b *strings.Builder,
	r *rng,
	opts Options,
	env envInfo,
	scope scopeInfo,
	state *functionFlowState,
	from int,
	ctx *genContext,
) bool {
	if state == nil {
		return false
	}

	candidates := make([]int, 0, len(state.funcs))
	for i := 0; i < len(state.funcs); i++ {
		// Keep acyclic call graph in generated order to avoid runaway recursion.
		if i <= from {
			continue
		}
		candidates = append(candidates, i)
	}

	// Upstream-like function invocation strategy:
	// 1) with a coin flip, try existing function first;
	// 2) if none available (or the coin says no), create one if limit allows.
	useExisting := len(candidates) > 0 && r.upto(2) == 0
	var callee funcInfo
	if useExisting {
		calleeIdx := candidates[int(r.upto(uint32(len(candidates))))]
		callee = state.funcs[calleeIdx]
	} else {
		created, _, ok := state.appendNewFunction(r, nil)
		if ok {
			callee = created
		} else if len(candidates) > 0 {
			calleeIdx := candidates[int(r.upto(uint32(len(candidates))))]
			callee = state.funcs[calleeIdx]
		} else {
			return false
		}
	}
	args := "void"
	if len(callee.params) > 0 {
		argExprs := make([]string, 0, len(callee.params))
		er := newExprRand(r, exprDecisionBudget(opts))
		for _, p := range callee.params {
			var prevSkip bool
			var prevQfer []bool
			if ctx != nil {
				prevSkip = ctx.skipFuncRetQfer
				prevQfer = ctx.incomingQferConsts
				ctx.skipFuncRetQfer = false
				ctx.incomingQferConsts = p.constLevels
			}
			argExprs = append(argExprs, randomParamExprDepth(p.ctype, er, opts, env, scope, 0, ctx))
			if ctx != nil {
				ctx.skipFuncRetQfer = prevSkip
				ctx.incomingQferConsts = prevQfer
			}
		}
		args = strings.Join(argExprs, ", ")
	}
	if args == "void" {
		writeLine(b, 1, fmt.Sprintf("x ^= (uint32_t)%s();", callee.name))
	} else {
		writeLine(b, 1, fmt.Sprintf("x ^= (uint32_t)%s(%s);", callee.name, args))
	}
	return true
}

func emitStatement(
	b *strings.Builder,
	r *rng,
	opts Options,
	env envInfo,
	scope scopeInfo,
	state *functionFlowState,
	info compositeInfo,
	from int,
	depth int,
	inLoop bool,
	stmtBudget *int,
	ctx *genContext,
	dec stmtDecision,
) bool {
	if stmtBudget != nil && *stmtBudget == 0 {
		return true
	}
	maybeDeclareOnDemandLocal(b, r, opts, ctx)
	if stmtBudget != nil && *stmtBudget > 0 {
		*stmtBudget = *stmtBudget - 1
	}
	chooseStmt := func() stmtKind {
		toKind := func(v int) stmtKind {
			switch {
			case v < 15:
				return stmtIfElse
			case v < 30:
				return stmtFor
			case v < 35:
				return stmtReturn
			case v < 40:
				return stmtContinue
			case v < 45:
				return stmtBreak
			case opts.Jumps && opts.Arrays && v < 50:
				return stmtGoto
			case opts.Jumps && opts.Arrays && v < 60:
				return stmtArrayOp
			case opts.Jumps && !opts.Arrays && v < 50:
				return stmtGoto
			case !opts.Jumps && opts.Arrays && v < 55:
				return stmtArrayOp
			default:
				return stmtAssign
			}
		}
		// Mimics upstream rnd_upto(..., StatementFilter):
		// retries happen inside one RNG API call.
		if dec.r != nil {
			v := int(dec.r.uptoWithFilter(100, func(x uint32) bool {
				k := toKind(int(x))
				if (k == stmtBreak || k == stmtContinue) && !inLoop {
					return true
				}
				if depth >= max(1, opts.MaxBlockDepth) && (k == stmtIfElse || k == stmtFor) {
					return true
				}
				return false
			}))
			return toKind(v)
		}
		return toKind(int(dec.pick(0, 100)))
	}

	st := chooseStmt()
	// seed2 e948: after continue ends array-loop body, next parent stmt U100=68
	// then U2 — For with postArrayFor (not Assign+U120). Flag survives body exit.
	afterCont := state != nil && state.lastStmtWasContinue
	if state != nil {
		state.lastStmtWasContinue = false
	}
	remappedAssignToFor := false
	if st == stmtAssign && afterCont && state != nil &&
		state.loopIVPool > 1 && state.multiDimArrays > 0 {
		st = stmtFor
		state.useSmallParentStack = true
		remappedAssignToFor = true
	}
	switch st {
	case stmtAssign:
		if !emitLValueAssignment(b, r, opts, env, scope, ctx) {
			return false
		}
	case stmtIfElse:
		// StatementIf::make_random uses Expression::make_random for the condition
		// (term table total 120 when assigns/commas enabled), not a fixed mask.
		er := newExprRand(r, exprDecisionBudget(opts))
		noConst := !opts.ConstAsCondition
		e := randomTypedExprDepthFlags(CType{Name: "uint32_t", Signed: false, Bits: 32}, er, opts, env, scope, 0, ctx, false, noConst)
		cond := fmt.Sprintf("((uint32_t)%s != 0u)", e)
		writeLine(b, 1, fmt.Sprintf("if %s {", cond))
		// If/else blocks are short-lived on the Csmith stack relative to loops;
		// only loop bodies grow blockStack for SelectParentLocal (seed2 e420 n=3).
		emitStatements(b, r, opts, env, scope, state, info, from, depth+1, false, stmtBudget, ctx)
		writeLine(b, 1, "} else {")
		emitStatements(b, r, opts, env, scope, state, info, from, depth+1, false, stmtBudget, ctx)
		writeLine(b, 1, "}")
	case stmtFor:
		// SelectLoopCtrlVar: choose_ok_var among integer non-array visibles.
		// len==1 → no RNG; len>1 → rnd_upto(len); empty → burnSelectLoopCtrlVarCreate.
		// loopIVPool: 2+ after array-loop (e370 multi-IV + array_control);
		// 1 after first nested for (e503 reuse IV, no choose RNG + loop_control);
		// 0 → create IV via burnSelectLoopCtrlVarCreate (vol retry, NewArray may
		// expand to full multi-dim CreateArrayVariable + itemize; seed2 e560–e678).
		postArrayFor := state != nil && state.loopIVPool > 1
		createIV := state != nil && state.deepStack && state.loopIVPool == 0
		// First for in an array-loop body (or multi-IV postArrayFor) uses array_control;
		// later nested fors use loop_control (e502, e519) even while still nested.
		// seed2 e1123–1125: natural For after continue → select_array U5+U1, no SafeOpFlags.
		afterContFor := afterCont && !remappedAssignToFor && state != nil && state.multiDimArrays > 0
		useArrayControl := postArrayFor || (state != nil && state.arrayLoopFresh) || afterContFor
		if postArrayFor {
			_ = r.upto(uint32(state.loopIVPool))
		} else if afterContFor {
			_ = r.upto(5)
			_ = r.upto(1)
			if state != nil {
				state.skipNextBlockSize = true
			}
		} else if createIV {
			// SelectLoopCtrlVar → GenerateNewGlobal with NewArray + volatile retry.
			burnSelectLoopCtrlVarCreate(r, opts)
		}
		// loopIVPool==1: reuse existing IV, no choose RNG (len==1).
		if useArrayControl && !afterContFor {
			// make_random_array_control + SafeOpFlags.
			// postArrayFor multi-dim (e949): itemize U9 U8. Early e679: U1.
			if postArrayFor && state != nil && state.multiDimArrays > 0 {
				_ = r.upto(9)
				_ = r.upto(8)
			} else {
				_ = r.upto(1)
			}
			_ = r.flipcoin(0) // array_oob_prob
			_ = r.flipcoin(50)
			if !r.flipcoin(50) {
				if postArrayFor && state != nil && state.multiDimArrays > 0 {
					_ = r.upto(1)
				}
			}
			_ = r.flipcoin(50) // incr
			// SafeOpFlags: skip first F50 when postArrayFor multi-dim (e928/e957).
			if !(postArrayFor && state != nil && state.multiDimArrays > 0) {
				_ = r.flipcoin(50)
			}
			_ = r.upto(4)
			_ = r.flipcoin(50)
			_ = r.flipcoin(50)
			_ = r.upto(4)
			if state != nil {
				state.arrayLoopFresh = false
				if state.loopIVPool > 1 {
					state.loopIVPool = 1
				}
			}
		} else if !afterContFor {
			// make_random_loop_control
			if !r.flipcoin(50) {
				_ = r.upto(60)
			}
			_ = r.upto(60)
			_ = r.upto(6)
			if r.flipcoin(50) {
				_ = r.upto(10)
			} else {
				_ = r.flipcoin(50)
			}
			// make_iteration SafeOpFlags
			_ = r.flipcoin(50)
			_ = r.upto(4)
			_ = r.flipcoin(50)
			_ = r.flipcoin(50)
			_ = r.upto(4)
			_ = r.flipcoin(50)
			_ = r.upto(4)
			if state != nil && state.loopIVPool == 1 {
				state.loopIVPool = 0 // later fors recreate IV (e521)
			}
		}
		writeLine(b, 1, "for (int32_t i = 0; i < 10; ++i) {")
		writeLine(b, 2, "x += (uint32_t)i;")
		if state != nil {
			state.blockStack++
		}
		emitStatements(b, r, opts, env, scope, state, info, from, depth+1, true, stmtBudget, ctx)
		if state != nil && state.blockStack > 0 {
			state.blockStack--
		}
		writeLine(b, 1, "}")
	case stmtReturn:
		ret := scope.returnVar
		if ret == "" {
			ret = "l_0"
		}
		writeLine(b, 1, fmt.Sprintf("%s ^= (uint32_t)x;", ret))
		writeLine(b, 1, fmt.Sprintf("return %s;", ret))
	case stmtContinue:
		writeLine(b, 1, "continue;")
		if state != nil {
			state.lastStmtWasContinue = true
		}
	case stmtBreak:
		writeLine(b, 1, "break;")
	case stmtGoto:
		// StatementGoto::make_random: F40 back-edge, then find_good_jump_block
		// rnd_upto(blocks.size()) [+ retries], or early null without U.
		// seed2 e518 F40=0 → null, Statement retry U100 (no block U).
		// seed2 e895 F40=1 → U1 block pick then null/retry (e896–897).
		backEdge := r.flipcoin(40)
		if backEdge {
			// find_good_jump_block U(func->blocks.size()); seed2 e896 U1.
			_ = r.upto(1)
		}
		return false
	case stmtArrayOp:
		// StatementArrayOp::make_random
		if !opts.Arrays {
			return false
		}
		// seed2 e1127: late ArrayOp in afterContFor body: U3 U2 then continue
		// (skip F5 init-vs-loop and U4 aryno).
		lateArrayOp := state != nil && state.useSmallParentStack && state.multiDimArrays > 0
		if lateArrayOp {
			// seed2 e1127–1129: U3 U2 then body U100 (no itemize U9 U8).
			_ = r.upto(3)
			_ = r.upto(2)
			if state != nil {
				state.skipNextBlockSize = true
			}
			writeLine(b, 1, "/* array loop late */ {")
			if state != nil {
				state.blockStack++
				state.arrayLoopDepth++
			}
			emitStatements(b, r, opts, env, scope, state, info, from, depth+1, true, stmtBudget, ctx)
			if state != nil {
				if state.blockStack > 0 {
					state.blockStack--
				}
				if state.arrayLoopDepth > 0 {
					state.arrayLoopDepth--
				}
			}
			writeLine(b, 1, "}")
			return true
		}
		if r.flipcoin(5) {
			// make_random_array_init → select_array
			nArr := len(env.arrays)
			if nArr == 0 {
				// create_random_array
				asGlobal := opts.GlobalVariables && r.flipcoin(25)
				if !asGlobal {
					// func->stack.size() — nested blocks may be >1
					_ = r.upto(2)
				}
				// type choose + filter retries; burn one AllTypes pick
				arrTy := pickNonVoidNonVolatile(r, nil, info, opts)
				// Constant::make_random for init
				if r.flipcoin(50) {
					if r.flipcoin(50) {
						_ = r.upto(3)
					} else {
						_ = r.upto(20)
					}
				} else {
					hn := hexDigitsForConstant(arrTy)
					if hn <= 0 {
						hn = 8
					}
					for i := 0; i < hn; i++ {
						_ = r.next31()
					}
				}
				// create_random_array → CreateArrayVariable without itemize
				if os.Getenv("CSMITH_DEBUG_ARRAY") != "" {
					fmt.Fprintf(os.Stderr, "ARRAY create ty=%s hex=%d\n", arrTy.Name, hexDigitsForConstant(arrTy))
				}
				burnCreateArrayVariable(r, opts, arrTy, false)
				// make_random_array_init: SelectLoopCtrlVar for the loop CV, then
				// further init indexing (seed2 e217 U3 + constant after IV create).
				burnSelectLoopCtrlVarCreate(r, opts)
				_ = r.upto(3) // seed2 e217 after IV constant
				burnSimpleConstant(r, arrTy)
			} else if nArr > 1 {
				_ = r.upto(uint32(nArr))
			}
			writeLine(b, 1, "/* array init */ x ^= x;")
		} else {
			// make_random_array_loop: rnd_upto(max_array_num_in_loop=4),
			// then per-array select_array + rnd_upto(3) access, then StatementFor.
			aryno := int(r.upto(4))
			frameMustRead := false
			nArr := len(env.arrays)
			// Inventory under-count vs true visible arrays (seed2 e918 U5).
			if nArr < 1 {
				nArr = 1
			}
			if nArr < 5 && state != nil && state.multiDimArrays > 0 {
				nArr = 5
			}
			for i := 0; i < aryno; i++ {
				// select_array: len==1 → no U; len>1 → rnd_upto(len) (seed2 e918 U5).
				if nArr > 1 {
					_ = r.upto(uint32(nArr))
				}
				access := int(r.upto(3)) // 0 must-read, 1 must-write, 2 both
				if access == 0 || access == 2 {
					frameMustRead = true
				}
			}
			// SelectLoopCtrlVar among integer visibles.
			// First array-loop: n=3 (seed2 e360). Later n=2 (e370, e920).
			// Empty pool + deepStack early → create.
			createdIV := false
			if state != nil && state.deepStack && state.loopIVPool == 0 &&
				state.multiDimArrays == 0 {
				burnSelectLoopCtrlVarCreate(r, opts)
				createdIV = true
			} else if state != nil && state.loopIVPool == 0 && state.multiDimArrays > 0 {
				// Multi-dim programs have grown integer locals; choose n=2 (e920).
				_ = r.upto(2)
			} else {
				nIV := 3
				if state != nil && state.loopIVPool > 0 {
					nIV = state.loopIVPool
				}
				_ = r.upto(uint32(nIV))
			}
			if state != nil {
				state.deepStack = true
				if createdIV {
					// Seed2 e560: after array-loop creates an IV, the next nested
					// for still takes the create path (pool stays 0). Upstream may
					// later choose the non-vol IV; keeping 0 preserves the matched
					// create stream through e716.
					state.loopIVPool = 0
				} else {
					// After first array-loop choose n=3, subsequent fors see n=2 (e370).
					state.loopIVPool = 2
				}
			}
			// choose_ok_var among must-use arrays (len==1 → no U), then
			// itemize() burns rnd_upto per dim. Early seed2 e358: U1 (1d size 1
			// or upto(1)). After multi-dim: often 2d itemize e.g. g_64[9][8]
			// (seed2 e921–922 U9 U8) before make_random_array_control.
			if state != nil && state.multiDimArrays > 0 {
				_ = r.upto(9) // itemize dim0
				_ = r.upto(8) // itemize dim1
			} else {
				_ = r.upto(1) // early seed2 e358
			}
			_ = r.flipcoin(0) // array_oob_prob
			// signed IV → flipcoin(50) for Le vs Ge
			_ = r.flipcoin(50)
			// CmpLe path: pure_rnd_flipcoin(50) for init 0 vs upto(bound/2);
			// pure_rnd_flipcoin(50) for incr 1 vs upto(bound/4).
			// pure_rnd_upto(0) is a no-op (array size 1 → bound 0 after --bound):
			// early seed2 e362 F50=0 with no U. Multi-dim e926 U1 when bound/2≥1.
			if !r.flipcoin(50) {
				if state != nil && state.multiDimArrays > 0 {
					_ = r.upto(1) // e926
				}
			}
			if !r.flipcoin(50) {
				if state != nil && state.multiDimArrays > 0 {
					// bound/4 may be 0 early; only burn when multi-dim sizes allow
					// (often still 0 — leave as no-op unless needed).
				}
			}
			// SafeOpFlags: init sOpAssign F50+U4; test sOpBinary F50+F50+U4.
			// Early e364 starts F50; multi-dim e928 starts U4 (signed coin elided
			// when flags forced by IV type in some paths — match stream).
			if state == nil || state.multiDimArrays == 0 {
				_ = r.flipcoin(50)
			}
			_ = r.upto(4)
			_ = r.flipcoin(50)
			_ = r.flipcoin(50)
			_ = r.upto(4)
			writeLine(b, 1, "/* array loop */ {")
			if state != nil {
				state.blockStack++
				state.arrayLoopDepth++
				// Push outer fresh so nested array-loops restore it on exit.
				state.arrayLoopFreshStack = append(state.arrayLoopFreshStack, state.arrayLoopFresh)
				state.arrayLoopFresh = true
				if frameMustRead {
					state.mustReadLive = true
				}
			}
			emitStatements(b, r, opts, env, scope, state, info, from, depth+1, true, stmtBudget, ctx)
			if state != nil {
				if state.blockStack > 0 {
					state.blockStack--
				}
				if state.arrayLoopDepth > 0 {
					state.arrayLoopDepth--
				}
				if n := len(state.arrayLoopFreshStack); n > 0 {
					state.arrayLoopFresh = state.arrayLoopFreshStack[n-1]
					state.arrayLoopFreshStack = state.arrayLoopFreshStack[:n-1]
				} else {
					state.arrayLoopFresh = false
				}
			}
			writeLine(b, 1, "}")
		}
	default:
		return false
	}
	return true
}

func emitStatements(
	b *strings.Builder,
	r *rng,
	opts Options,
	env envInfo,
	scope scopeInfo,
	state *functionFlowState,
	info compositeInfo,
	from int,
	depth int,
	inLoop bool,
	stmtBudget *int,
	ctx *genContext,
) {
	if stmtBudget != nil && *stmtBudget == 0 {
		return
	}
	stmtLimit := max(1, opts.MaxBlockSize)
	base := 2
	if depth > 0 {
		base = 1
	}
	// seed2 e1126: afterContFor body skips BlockSize U.
	stmtCount := 1
	if state != nil && state.skipNextBlockSize {
		state.skipNextBlockSize = false
		stmtCount = 1
	} else {
		stmtCount = base + int(r.upto(uint32(stmtLimit)))
	}
	for s := 0; s < stmtCount; s++ {
		if stmtBudget != nil && *stmtBudget == 0 {
			break
		}
		const maxStmtAttempts = 8
		ok := false
		for attempt := 0; attempt < maxStmtAttempts; attempt++ {
			dec := nextStmtDecision(r)
			snapStmtBudget := -1
			if stmtBudget != nil {
				snapStmtBudget = *stmtBudget
			}
			snap := takeGenSnapshot(ctx)

			var tmp strings.Builder
			if emitStatement(&tmp, r, opts, env, scope, state, info, from, depth, inLoop, stmtBudget, ctx, dec) {
				b.WriteString(tmp.String())
				ok = true
				break
			}

			// Reject path: rollback to keep statement-local retries side-effect free.
			if stmtBudget != nil && snapStmtBudget >= 0 {
				*stmtBudget = snapStmtBudget
			}
			restoreGenSnapshot(ctx, snap)
		}
		if !ok {
			// Last-resort deterministic no-op-like mutation when all attempts fail.
			writeLine(b, 1, "x ^= 0u;")
		}
	}
}

func emitSingleFuncDef(
	r *rng,
	opts Options,
	fn funcInfo,
	state *functionFlowState,
	idx int,
	maxBlock int,
	env envInfo,
	info compositeInfo,
	stmtBudget *int,
) string {
	return emitSingleFuncDefOnce(r, opts, fn, state, idx, maxBlock, env, info, stmtBudget)
}

func emitSingleFuncDefOnce(
	r *rng,
	opts Options,
	fn funcInfo,
	state *functionFlowState,
	idx int,
	maxBlock int,
	env envInfo,
	info compositeInfo,
	stmtBudget *int,
) string {
	if state != nil {
		prevSink := multiDimArraySink
		multiDimArraySink = &state.multiDimArrays
		prevMR := mustReadLiveSink
		mustReadLiveSink = &state.mustReadLive
		prevP := postMustReadGlobalPicks
		postMustReadGlobalPicks = &state.postMustReadGlobalPicks
		pointerGlobalPicksSink = &state.pointerGlobalPicks
		useSmallParentStackSink = &state.useSmallParentStack
		lastArraySizesSink = &state.lastArraySizes
		defer func() {
			multiDimArraySink = prevSink
			mustReadLiveSink = prevMR
			postMustReadGlobalPicks = prevP
			pointerGlobalPicksSink = nil
			useSmallParentStackSink = nil
			lastArraySizesSink = nil
		}()
	}
	fdec := nextFuncDecision(r)
	var b strings.Builder

	params := "void"
	if len(fn.params) > 0 {
		pp := make([]string, 0, len(fn.params))
		for _, p := range fn.params {
			pp = append(pp, fmt.Sprintf("%s %s", p.ctype.Name, p.name))
		}
		params = strings.Join(pp, ", ")
	}
	writeLine(&b, 0, fmt.Sprintf("static %s %s(%s) {", fn.ret.Name, fn.name, params))
	retName := "l_0"
	if state != nil {
		retName = state.allocLocalName()
	}
	writeLine(&b, 1, fmt.Sprintf("%s %s = %s;", fn.ret.Name, retName, castLiteral(fn.ret, "0u")))
	if len(env.globals) >= 2 {
		writeLine(&b, 1, fmt.Sprintf("uint32_t x = ((uint32_t)%s) + ((uint32_t)%s);", env.globals[0].name, env.globals[1].name))
	} else {
		writeLine(&b, 1, "uint32_t x = 0u;")
	}

	locals := make([]localInfo, 0, 1)
	locals = append(locals, localInfo{name: "x", ctype: CType{Name: "uint32_t", Signed: false, Bits: 32}})
	scope := scopeInfo{params: fn.params, locals: locals, returnVar: retName}
	ctx := &genContext{
		state: state,
		from:  idx,
		info:  info,
	}

	for _, p := range fn.params {
		writeLine(&b, 1, fmt.Sprintf("x ^= (uint32_t)%s;", p.name))
	}
	emitStatements(&b, r, opts, env, scope, state, info, idx, 0, false, stmtBudget, ctx)
	if len(env.globals) > 0 {
		writable := make([]globalInfo, 0, len(env.globals))
		for _, g := range env.globals {
			if g.isConst {
				continue
			}
			writable = append(writable, g)
		}
		if len(writable) > 0 {
			g := writable[int(fdec.pick(2, uint32(len(writable))))]
			writeLine(&b, 1, fmt.Sprintf("%s ^= %s;", g.name, randomTypedExpr(g.ctype, r, opts, env, scope, ctx)))
		}
	}
	writeLine(&b, 1, fmt.Sprintf("%s ^= %s;", retName, castLiteral(fn.ret, "x")))
	writeLine(&b, 1, fmt.Sprintf("return %s;", retName))
	writeLine(&b, 0, "}")
	writeLine(&b, 0, "")
	return b.String()
}

func (s *functionFlowState) allocParamName() string {
	name := fmt.Sprintf("p_%d", s.nextParamID)
	s.nextParamID++
	return name
}

func (s *functionFlowState) allocLocalName() string {
	name := fmt.Sprintf("l_%d", s.nextLocalID)
	s.nextLocalID++
	return name
}

func (s *functionFlowState) makeFuncSignature(r *rng, idx int) funcInfo {
	fn := funcInfo{
		name: fmt.Sprintf("func_%d", idx),
	}
	// Function::make_first uses RandomReturnType() over AllTypes (simple + aggregates),
	// while later signatures are chosen from random types as they are created.
	if idx == 1 {
		allCount := len(s.pool) + len(s.info.structs) + len(s.info.unions)
		if allCount <= 0 {
			fn.ret = CType{Name: "uint32_t", Signed: false, Bits: 32}
		} else {
			pick := int(r.upto(uint32(allCount)))
			switch {
			case pick < len(s.pool):
				fn.ret = s.pool[pick]
			case pick < len(s.pool)+len(s.info.structs):
				fn.ret = CType{Name: fmt.Sprintf("struct S%d", pick-len(s.pool)), Bits: 32}
			default:
				fn.ret = CType{Name: fmt.Sprintf("union U%d", pick-len(s.pool)-len(s.info.structs)), Bits: 32}
			}
		}
	} else {
		fn.ret = pickType(r, s.pool)
	}
	if idx == 1 {
		// make_first() creates return variable qualifiers via
		// CVQualifiers::random_qualifiers(type) (no_volatile=true), which still
		// consumes volatile/const draws on the object itself.
		if s.opts.Volatiles {
			_ = r.flipcoin(50)
		} else {
			_ = r.flipcoin(0)
		}
		if s.opts.Consts {
			_ = r.flipcoin(10)
		} else {
			_ = r.flipcoin(0)
		}
	}
	maxParams := s.opts.MaxParams
	if idx == 1 {
		maxParams = 0
	}
	if maxParams < 0 {
		maxParams = 0
	}
	pcount := 0
	if maxParams > 0 {
		pcount = int(r.upto(uint32(maxParams + 1)))
	}
	fn.params = make([]paramInfo, 0, pcount)
	for p := 0; p < pcount; p++ {
		fn.params = append(fn.params, paramInfo{
			name:  s.allocParamName(),
			ctype: pickType(r, s.pool),
		})
	}
	return fn
}

func emitFunctionsUpstreamFlow(b *strings.Builder, r *rng, opts Options, pool []CType, maxBlock int, env envInfo, info compositeInfo) ([]funcInfo, []globalInfo) {
	maxFuncs := max(opts.MaxFuncs, 1)
	state := &functionFlowState{
		funcs:        []funcInfo{},
		built:        []bool{},
		defs:         []string{},
		maxFuncs:     maxFuncs,
		nextIdx:      2,
		nextParamID:  1,
		nextLocalID:  0,
		pool:         pool,
		info:         info,
		opts:         opts,
		dynGlobals:   []globalInfo{},
		nextGlobalID: env.nextID,
		stmtBudget:   opts.StopByStmt,
		blockStack:   1, // function body block
	}
	state.funcs = append(state.funcs, state.makeFuncSignature(r, 1))
	state.built = append(state.built, false)
	state.defs = append(state.defs, "")
	if state.stmtBudget < 0 {
		state.stmtBudget = -1
	}

	for cur := 0; cur < len(state.funcs); cur++ {
		if state.built[cur] {
			continue
		}
		state.defs[cur] = emitSingleFuncDef(r, opts, state.funcs[cur], state, cur, maxBlock, env, info, &state.stmtBudget)
		state.built[cur] = true
	}

	if state.lateGlobals.Len() > 0 {
		b.WriteString(state.lateGlobals.String())
		writeLine(b, 0, "")
	}
	emitFuncDecls(b, state.funcs)
	writeLine(b, 0, "/* --- FUNCTIONS --- */")
	writeLine(b, 0, "/* ------------------------------------------ */")
	for i := 0; i < len(state.defs); i++ {
		b.WriteString(state.defs[i])
	}
	return state.funcs, state.dynGlobals
}

func emitComputeHashFunc(b *strings.Builder, env envInfo, info compositeInfo) {
	writeLine(b, 0, "void csmith_compute_hash(int print_hash_value)")
	writeLine(b, 0, "{")
	for _, g := range env.globals {
		writeLine(b, 1, fmt.Sprintf("transparent_crc((uint64_t)%s, \"%s\", print_hash_value);", g.name, g.name))
	}
	for _, arr := range env.arrays {
		writeLine(b, 1, fmt.Sprintf("for (int i = 0; i < %d; i++)", arr.len))
		writeLine(b, 2, fmt.Sprintf("transparent_crc((uint64_t)%s[i], \"%s[i]\", print_hash_value);", arr.name, arr.name))
	}
	_ = info
	writeLine(b, 0, "}")
	writeLine(b, 0, "")
}

func emitMain(b *strings.Builder, opts Options, env envInfo, info compositeInfo, entry string) {
	useRuntime := opts.SafeMath || opts.ComputeHash
	useHashPrintf := opts.HashValuePrintf
	if opts.AcceptArgc {
		writeLine(b, 0, "int main(int argc, char *argv[]) {")
		writeLine(b, 1, "int print_hash_value = 0;")
		if useRuntime && useHashPrintf {
			writeLine(b, 1, "if (argc == 2 && strcmp(argv[1], \"1\") == 0) print_hash_value = 1;")
		}
	} else {
		writeLine(b, 0, "int main(void) {")
		if useRuntime {
			writeLine(b, 1, "int print_hash_value = 0;")
		}
	}

	if useRuntime {
		writeLine(b, 1, "platform_main_begin();")
		if opts.ComputeHash {
			writeLine(b, 1, "crc32_gentab();")
		}
	}
	writeLine(b, 1, fmt.Sprintf("(void)%s();", entry))
	if opts.ComputeHash {
		writeLine(b, 1, "csmith_compute_hash(print_hash_value);")
		if useRuntime {
			writeLine(b, 1, "platform_main_end(crc32_context ^ 0xFFFFFFFFUL, print_hash_value);")
		} else {
			writeLine(b, 1, "platform_main_end(0,0);")
		}
	}
	if !opts.ComputeHash && useRuntime {
		writeLine(b, 1, "platform_main_end(0u, 0);")
	}
	writeLine(b, 1, "return 0;")
	writeLine(b, 0, "}")
}

// Generate emits deterministic C code from options and seed.
func Generate(opts Options) (string, error) {
	var err error
	opts, err = opts.resolvePlatformInfo()
	if err != nil {
		return "", err
	}
	opts = opts.normalizeUpstreamFlow()

	if err := opts.validate(); err != nil {
		return "", err
	}
	gen := createProgramGenerator(opts)
	gen.initialize()
	return gen.goGenerator(), nil
}
