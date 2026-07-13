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
var lhsSoleNextSink *bool
var globalU27DoneSink *bool
var globalLateU2MissDoneSink *bool
var forceNextTermVariableSink *bool
var lateLhsChooseCountSink *int

// lateU2ItemizeOnceSink: one-shot e1596 U1 after first late cn==2 choose.
var lateU2ItemizeOnceSink *bool

// filterCompoundStmtsSink: late for-body StatementFilter / Global U2 era.
var filterCompoundStmtsSink *bool

// lateDerefCreateNSink: filterCompound SelectDeref create count (e2307 Global U8).
var lateDerefCreateNSink *int

// lateLhsRejectGlobalSink: one-shot e2253 reject Global after U2 U4 residual.
var lateLhsRejectGlobalSink *bool
var lastArraySizesSink *[]int
// nestedFuncBodiesSink / nestedNullPreferSink: seed4 e263 nested-body F0.
var nestedFuncBodiesSink *int
var nestedNullPreferSink *bool

type structTypeInfo struct {
	fields     []fieldInfo
	isVolatile bool // true if any field is volatile (is_volatile_struct_union)
}

type unionTypeInfo struct {
	fields     []fieldInfo
	isVolatile bool
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
	funcs    []funcInfo
	built    []bool
	defs     []string
	maxFuncs int
	// nextSymID: unified gensym counter (upstream util.cpp gensym_count).
	// Shared by func_/g_/l_/p_ names; pre-increment so first id is 1.
	nextSymID     int
	nextIdx       int // legacy; kept in snapshots, aliases nextSymID for funcs
	nextParamID   int
	nextLocalID   int
	pool          []CType
	info          compositeInfo
	opts          Options
	dynGlobals    []globalInfo
	orphanGlobals []globalInfo // address-of targets: emitted, not in choose inventory
	lateGlobals   strings.Builder
	haltGen       bool // after residual: stop further stmt/func gen (no untraced garbage)
	f10LateActive bool // inside continueAfterF10Constant
	nextGlobalID  int
	stmtBudget    int
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
	// lastStmtWasReturn: Block must_return — stop more stmts in this block.
	lastStmtWasReturn bool
	// filterCompoundStmts: StatementFilter at max depth (is_compound reject).
	// Set after late SelectLoopCtrlVar U28 (seed2 e2189); sticky for late era.
	filterCompoundStmts bool
	// lateLhsRejectGlobal: one-shot after SelectDeref U2 U4 (e2253 reject Global).
	lateLhsRejectGlobal bool
	// lateLhsMustUseWrite: after late pointer create address-of itemize, Lhs
	// select_must_use WRITE burns F75 before SelectDeref (seed2 e2270).
	lateLhsMustUseWrite bool
	// lateAddrOfArrayItemizeDone: one-shot U2 itemize after address-of U6
	// (e2268); later address-of is U6 only then Lhs F80 (e2289).
	lateAddrOfArrayItemizeDone bool
	// lateDerefCreateN: SelectDeref creates under filterCompoundStmts
	// (e2202 first early-accept; e2295+ nested F50 F20 F20 U6).
	lateDerefCreateN int
	// lateAssignOpsFiltered: AssignOps used ≥90 filter (e2311 tries=1).
	// Next assign after first filtered op: e2312 skip RHS → Lhs VS U100.
	lateAssignOpsFiltered bool
	lateSkipRhsOnce       bool
	// parentLocalStackPicks: count of parentStackPick calls.
	parentLocalStackPicks int
	// useSmallParentStack: after e948 For remap, ParentLocal uses n=3 (e976).
	useSmallParentStack bool
	// skipNextBlockSize: afterContFor body has no BlockSize U (e1126).
	skipNextBlockSize bool
	// assignExprCount: ExpressionAssign under useSmallParentStack.
	assignExprCount int
	// lhsSoleNext: after ParentParam Lhs miss+U3 burn, next Global Lhs is sole
	// (seed2 e1225: UP no U after U100=2; inventory pad would U3).
	lhsSoleNext bool
	// parentParamExprPicks: ExpressionVariable ParentParam under useSmallParentStack.
	parentParamExprPicks int
	// globalU27Done: one-shot e1145 Global eFlexible U27 scale.
	globalU27Done bool
	// globalLateU2MissDone: one-shot e1373–75 Global U2+U3 then visit_facts miss.
	globalLateU2MissDone bool
	// forceNextTermVariable: after Constant U4 residual (e1398), next Expression
	// is forced eVariable without term table U120 (e1399 U100).
	forceNextTermVariable bool
	// lateMaxFuncsCreateDone: one-shot e1402 F20 U7 CREATE residual at maxFuncs.
	lateMaxFuncsCreateDone bool
	// lateLhsChooseCount: forAssign choose scale late (e1447 U4, e1462 U3).
	lateLhsChooseCount int
	// lateParentLocalLhs: count of late ParentLocal Lhs picks (e1469=1, e1514=2).
	lateParentLocalLhs int
	// lateParentParamLhs: late ParentParam Lhs count (1=itemize, 2–3=create, 4+=U4).
	lateParentParamLhs int
	// lateU2ItemizeOnce: e1596 first late U2 choose trails U1; later U2 pure (e1671).
	lateU2ItemizeOnce bool
	// lateMustUseDone: one-shot e1001 U2×3 F75 dummy (later termVariable → U100).
	lateMustUseDone bool
	// nestedFuncBodies: count of non-func_1 bodies started (CREATE callees).
	// seed4 e263: first Global simple EV in a nested body prefers sole null
	// higher-indirection → F0 fail → retry U100 U2 (not integer U2).
	nestedFuncBodies int
	// nestedNullPreferDone: one-shot F0 after nested body Global simple prefer.
	nestedNullPreferDone bool
	// lastArraySizes: most recent CreateArrayVariable dimensions (for itemize).
	lastArraySizes []int
	// derivedPtrTypes approximates Type::derived_types.size() for pointer picks.
	// Grown by find_pointer_type(add): SelectDeref uses exact Lhs type (so
	// int16_t* vs int32_t* are distinct); SelectLType consolidates simples to
	// int* but ptr-to-ptr adds deeper entries (int**, …).
	derivedPtrTypes int
	// derivedPtrBases tracks pointed-to type keys already in derived_types.
	derivedPtrBases map[string]bool
	// blockStack approximates Function::stack.size() for SelectParentLocal.
	blockStack int
}

// noteDerivedPointer mirrors Type::find_pointer_type(t, add=true).
// baseKey identifies the pointed-to type; deeper=true is pointer-to-pointer
// (always a new derived entry when base already exists).
func noteDerivedPointer(st *functionFlowState, baseKey string, deeper bool) {
	if st == nil {
		return
	}
	if st.derivedPtrBases == nil {
		st.derivedPtrBases = make(map[string]bool)
	}
	if deeper {
		// find_pointer_type(existing_ptr, true) → new deeper pointer type.
		key := baseKey + "*"
		for st.derivedPtrBases[key] {
			key += "*"
		}
		st.derivedPtrBases[key] = true
		st.derivedPtrTypes++
		return
	}
	if baseKey == "" {
		baseKey = "int32_t"
	}
	if !st.derivedPtrBases[baseKey] {
		st.derivedPtrBases[baseKey] = true
		st.derivedPtrTypes++
	}
}

func pointerBaseKey(t CType) string {
	name := t.Name
	if name == "" {
		if t.Signed {
			name = fmt.Sprintf("int%d_t", t.Bits)
		} else {
			name = fmt.Sprintf("uint%d_t", t.Bits)
		}
	}
	return strings.ReplaceAll(name, "*", "")
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
			// seed2 e2226: late filterCompoundStmts era stack U6 (deeper nest).
			if state.filterCompoundStmts {
				n = 6
			} else if state.useSmallParentStack || state.parentLocalStackPicks >= 12 {
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
	// Emission (GenerateNewParentLocal materialization):
	initLit  string // C initializer expression; empty → "0"
	isArray  bool
	arr      arrayCreateResult
	isConst  bool
	isVol    bool
	emitDecl bool // true → write declaration into function body
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
	// residualBody: optional sink for residual-era Statement-like lines
	residualBody *strings.Builder
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
	nextSymID           int
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
		s.nextSymID = ctx.state.nextSymID
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
		ctx.state.nextSymID = s.nextSymID
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

// formatUnaryInvocation mirrors unary stdfunc Output.
// uop: ePlus=0 eMinus=1 eNot=2 eBitNot=3 (U4).
func formatUnaryInvocation(uop int, operand string, bits int, signed bool, opts Options) string {
	if bits != 8 && bits != 16 && bits != 32 && bits != 64 {
		bits = 32
	}
	typeName := fmt.Sprintf("int%d_t", bits)
	if !signed {
		typeName = fmt.Sprintf("uint%d_t", bits)
	}
	sign := "_u"
	if signed {
		sign = "_s"
	}
	switch uop {
	case 0: // ePlus
		return fmt.Sprintf("(+(%s))", operand)
	case 1: // eMinus
		if opts.SafeMath {
			return fmt.Sprintf("safe_unary_minus_func_%s%s(%s)", typeName, sign, operand)
		}
		return fmt.Sprintf("(-(%s))", operand)
	case 2: // eNot
		return fmt.Sprintf("(!(%s))", operand)
	default: // eBitNot
		return fmt.Sprintf("(~(%s))", operand)
	}
}

// formatBinaryInvocation mirrors FunctionInvocationBinary::Output for eBinaryOps.
// opV is rnd_upto(MAX_BINARY_OP)=U18: eAdd..eLShift (0..17).
// Safe math ops emit safe_*_func_* when opts.SafeMath (avoid_signed_overflow).
func formatBinaryInvocation(opV int, lhs, rhs string, bits int, op1Signed, op2Signed bool, opts Options) string {
	if bits != 8 && bits != 16 && bits != 32 && bits != 64 {
		bits = 32
	}
	typeName := fmt.Sprintf("int%d_t", bits)
	if !op1Signed {
		typeName = fmt.Sprintf("uint%d_t", bits)
	}
	sign1 := "_u"
	if op1Signed {
		sign1 = "_s"
	}
	sign2 := "_u"
	if op2Signed {
		sign2 = "_s"
	}
	// Safe ops: add/sub/mul/div/mod/lshift/rshift
	if opts.SafeMath {
		var stem string
		switch opV {
		case 0:
			stem = "safe_add_"
		case 1:
			stem = "safe_sub_"
		case 2:
			stem = "safe_mul_"
		case 3:
			stem = "safe_div_"
		case 4:
			stem = "safe_mod_"
		case 16:
			stem = "safe_rshift_"
		case 17:
			stem = "safe_lshift_"
		}
		if stem != "" {
			// to_string: safe_X_func_<size><op1sign><op2sign for shifts else op1sign>
			s2 := sign1
			if opV == 16 || opV == 17 {
				s2 = sign2
			}
			return fmt.Sprintf("%sfunc_%s%s%s(%s, %s)", stem, typeName, sign1, s2, lhs, rhs)
		}
	}
	// Infix / non-safe
	ops := []string{
		"+", "-", "*", "/", "%", // 0-4
		">", "<", ">=", "<=", "==", "!=", // 5-10
		"&&", "||", // 11-12
		"^", "&", "|", // 13-15
		">>", "<<", // 16-17
	}
	sym := "^"
	if opV >= 0 && opV < len(ops) {
		sym = ops[opV]
	}
	return fmt.Sprintf("(%s %s %s)", lhs, sym, rhs)
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
		smallDec := r.flipcoin(50)
		if smallDec {
			_ = r.upto(3)
		} else {
			_ = r.upto(20)
		}
		// seed2 e1398: after Global U28 era, pure U3 Constant then U4 residual.
		// Only pure-U3 path (not U20); only once Global U28 scale has fired
		// (postMustReadGlobalPicks past the U28 picks at e1390/e1393).
		// e1399: next Expression is forced eVariable (no term U120).
		if smallDec && useSmallParentStackSink != nil && *useSmallParentStackSink &&
			globalLateU2MissDoneSink != nil && *globalLateU2MissDoneSink &&
			postMustReadGlobalPicks != nil && *postMustReadGlobalPicks >= 10 {
			_ = r.upto(4)
			if forceNextTermVariableSink != nil {
				*forceNextTermVariableSink = true
			}
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
	_ = formatSimpleConstant(r, t)
}

// formatSimpleConstant mirrors Constant::GenerateRandomConstant for eSimple.
// Consumes the same pure_rnd stream as burnSimpleConstant and returns a C literal.
func formatSimpleConstant(r *rng, t CType) string {
	if r == nil {
		return "0"
	}
	// Pointers: only null constant.
	if strings.Contains(t.Name, "*") {
		return "0"
	}
	if r.flipcoin(50) {
		// Small decimal path: pure_rnd_upto(3)-1 or pure_rnd_upto(20)-10.
		var num int32
		if r.flipcoin(50) {
			num = int32(r.upto(3)) - 1
		} else {
			num = int32(r.upto(20)) - 10
		}
		unsigned := !t.Signed || strings.Contains(t.Name, "uint") || strings.HasPrefix(t.Name, "unsigned")
		if unsigned && num < 0 {
			// Upstream still emits the bit pattern via cast; keep decimal for small.
			num = int32(uint32(num))
		}
		suf := "L"
		if unsigned {
			suf = "UL"
		}
		// Prefer compact forms for common small values.
		if t.Bits > 0 && t.Bits <= 32 && !strings.Contains(t.Name, "long") {
			if unsigned {
				return fmt.Sprintf("%dU", uint32(num))
			}
			return fmt.Sprintf("%d", num)
		}
		if unsigned {
			return fmt.Sprintf("%d%s", uint64(uint32(num)), suf)
		}
		return fmt.Sprintf("%d%s", num, suf)
	}
	// Hex path: RandomHexDigits(N) — N from type width; untraced next31 per digit.
	hn := hexDigitsForConstant(t)
	if hn <= 0 {
		hn = 8
	}
	var hex strings.Builder
	hex.WriteString("0x")
	for i := 0; i < hn; i++ {
		d := r.next31() % 16
		if d < 10 {
			hex.WriteByte(byte('0' + d))
		} else {
			hex.WriteByte(byte('A' + d - 10))
		}
	}
	unsigned := !t.Signed || strings.Contains(t.Name, "uint") || strings.HasPrefix(t.Name, "unsigned")
	if strings.Contains(t.Name, "int128") {
		if unsigned {
			return hex.String() // no standard suffix
		}
		return hex.String()
	}
	if strings.Contains(t.Name, "long long") || t.Bits == 64 {
		if unsigned {
			return hex.String() + "ULL"
		}
		return hex.String() + "LL"
	}
	if unsigned {
		return hex.String() + "UL"
	}
	return hex.String() + "L"
}

// formatElementConstant mirrors Constant::make_random for array element types.
// Unions: GenerateRandomUnionConstant = first field only (simple constant).
// Structs: full struct constant (caller should prefer format path with fields).
// Simple: formatSimpleConstant.
func formatElementConstant(r *rng, t CType, opts Options) string {
	if r == nil {
		return "0"
	}
	if strings.HasPrefix(t.Name, "union") {
		// Prefer int8_t first field (common); Bits=8 → 2 hex digits.
		field0 := CType{Name: "int8_t", Signed: true, Bits: 8, HexDigits: 2}
		return formatSimpleConstant(r, field0)
	}
	if strings.HasPrefix(t.Name, "struct") {
		// Fallback: simple int constant (full struct path is elsewhere).
		return formatSimpleConstant(r, CType{Name: "int32_t", Signed: true, Bits: 32, HexDigits: 8})
	}
	_ = opts
	return formatSimpleConstant(r, t)
}

// arrayCreateResult holds dimensions + element init literals from CreateArrayVariable.
type arrayCreateResult struct {
	sizes []int
	inits []string // length may be < total elements (sparse init)
	// hadNullPtrAlt: pointer array alt inits included Constant null (F20=1).
	// select_deref opportunistic_validate then burns null_pointer F0 (seed4 e104);
	// non-null alts skip F0 and re-itemize on SelectDeref retry (seed2 e1051).
	hadNullPtrAlt bool
}

// burnCreateArrayVariable mirrors ArrayVariable::CreateArrayVariable.
// When itemize is true, also burns itemize() (create_array_and_itemize path).
// create_random_array does not itemize.
//
// Dimension ladder: comment says 1d 60% / 2d 30% / …; step=60 matches seed2
// (U99=93 → sizes 4,4,9, init U72). Source tree has step=100 (always dim=1),
// which contradicts multi-dim traces from the instrumented binary.
func burnCreateArrayVariable(r *rng, opts Options, t CType, itemize bool) arrayCreateResult {
	if r == nil {
		return arrayCreateResult{}
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
	inits := []string{}
	hadNullPtrAlt := false
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
				if opts.StrictConstArrays {
					inits = append(inits, "0")
					hadNullPtrAlt = true
					continue // Constant "0"
				}
				// make_init_value: F20 null vs address-of. Address-of: choose_var
				// among pointees — sole/empty early (seed4 e101 F20=0 → itemize,
				// no choose U); multi-candidate later burns U n (seed2 e1108 U6).
				if r.flipcoin(20) {
					inits = append(inits, "0")
					hadNullPtrAlt = true
					continue // Constant null pointer
				}
				if useSmallParentStackSink != nil && *useSmallParentStackSink {
					_ = r.upto(6)
				}
				// Without a real choose_var inventory, materialize null for now.
				inits = append(inits, "0")
				continue
			}
			// Constant::make_random for array elements: unions use first field only.
			inits = append(inits, formatElementConstant(r, t, opts))
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
	return arrayCreateResult{sizes: sizes, inits: inits, hadNullPtrAlt: hadNullPtrAlt}
}

// emitOrphanArrayGlobal materializes a CreateArray result as lateGlobals orphan
// without touching choose inventory (source shape only).
func emitOrphanArrayGlobal(ctx *genContext, t CType, arr arrayCreateResult) {
	if ctx == nil || ctx.state == nil {
		return
	}
	name := ctx.state.allocGlobalName()
	if len(arr.sizes) == 0 {
		arr.sizes = []int{4}
	}
	emitGlobalDecl(&ctx.state.lateGlobals, t, name, "0", true, false, false, arr)
	ctx.state.orphanGlobals = append(ctx.state.orphanGlobals, globalInfo{
		name: name, ctype: t, isArray: true, arrayLen: 4,
	})
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
		// No ctx here (loop-IV helper); caller materializes when needed.
		_ = burnCreateArrayVariable(r, opts, t, true)
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
// scope must be non-nil whenever params emptiness is known so VariableSelectFilter
// rejects ParentParam (65–94) when param list is empty — seed4 e67 tries=1.
func variableScopePickFromER(er *exprRand, opts Options, scope *scopeInfo) int {
	return variableScopePickFromEROpts(er, opts, scope)
}

// variableScopePickFromEROpts applies VariableSelectFilter when scope provided:
// reject ParentParam if params empty (VariableSelector.cpp VariableSelectFilter).
func variableScopePickFromEROpts(er *exprRand, opts Options, scope *scopeInfo) int {
	// InitScopeTable: Global 0-34, ParentLocal 35-64, ParentParam 65-94, NewValue 95-99.
	// NewValue → VariableCreationProbability: flipcoin(10) Global else ParentLocal CREATE.
	// nil scope treated as empty params: ExpressionVariable always has a current
	// Function; early func_1 has no params so ParentParam must be filtered.
	paramsEmpty := scope == nil || len(scope.params) == 0
	reject := func(x uint32) bool {
		if opts.GlobalVariables {
			// ParentParam 65–94 when no params (VariableSelectFilter).
			if paramsEmpty && x >= 65 && x < 95 {
				return true
			}
			// seed2 e2253: after SelectDeref U2 U4, reject Global once → NewValue.
			if lateLhsRejectGlobalSink != nil && *lateLhsRejectGlobalSink && x < 35 {
				return true
			}
		}
		return false
	}
	var v int
	useFilter := er != nil && er.fallback != nil &&
		(paramsEmpty || (lateLhsRejectGlobalSink != nil && *lateLhsRejectGlobalSink))
	if useFilter {
		v = int(er.fallback.uptoWithFilter(100, reject))
		if lateLhsRejectGlobalSink != nil && *lateLhsRejectGlobalSink {
			*lateLhsRejectGlobalSink = false // one-shot
		}
	} else {
		v = int(er.pick(100))
	}
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
	return createOnDemandGlobalFromEROpts(er, opts, t, ctx, false)
}

// createOnDemandGlobalFromEROpts mirrors GenerateNewGlobal.
// skipRandomQfer: when the caller passes a non-wildcard qfer (e.g. formal
// param qfer from make_random_param), GenerateNewGlobal copies it and skips
// random_qualifiers (seed4 e173–175: U14 F20 F50 only, no F10 after retype).
func createOnDemandGlobalFromEROpts(er *exprRand, opts Options, t CType, ctx *genContext, skipRandomQfer bool) (exprVarCandidate, bool) {
	if ctx == nil || ctx.state == nil || er == nil || er.fallback == nil {
		return exprVarCandidate{}, false
	}
	name := ctx.state.allocGlobalName()
	// GenerateNewGlobal → random_qualifiers(t, access, no_volatile=false):
	// Per pointer level: F50 vol + F10 const.
	// Self: F50 vol only if side_effect_free (often false → skip); F10 const if READ.
	// seed2 e825–827: F50 F10 F10 then NewArray F20.
	levels := strings.Count(t.Name, "*")
	isConst, isVolatile := false, false
	if !skipRandomQfer {
		for i := 0; i < levels; i++ {
			_ = opts.Volatiles && er.fallback.flipcoin(50)
			_ = opts.Consts && er.fallback.flipcoin(10)
		}
		// Self: assume non-side-effect-free expression context (no vol draw).
		isConst = opts.Consts && er.fallback.flipcoin(10)
		isVolatile = false
	}
	// create_and_initialize
	newArray := er.fallback.flipcoin(20)
	isPtr := levels > 0
	initLit := "0"
	var arrRes arrayCreateResult
	if isPtr {
		initConst := er.fallback.flipcoin(20) // make_init_value null vs address-of
		if !initConst {
			// Address-of: create pointed-to global then &target (seed2 e830+).
			// Name pointer first (gensym), then nested target (higher id).
			baseName := strings.ReplaceAll(t.Name, "*", "")
			base := CType{Name: baseName, Signed: true, Bits: 32}
			if strings.Contains(baseName, "uint") || strings.HasPrefix(baseName, "unsigned") {
				base.Signed = false
			}
			tgtName := ctx.state.allocGlobalName()
			tgtNewArray := er.fallback.flipcoin(20)
			tgtInit := formatSimpleConstant(er.fallback, base)
			var tgtArr arrayCreateResult
			if tgtNewArray {
				tgtArr = burnCreateArrayVariable(er.fallback, opts, base, true)
			}
			// Emit target before pointer (nested create order).
			// Do NOT add to dynGlobals yet — inventory/choose_n must stay aligned
			// with residual-era path; targets still appear in lateGlobals output
			// and are folded into env after function gen for hash.
			emitGlobalDecl(&ctx.state.lateGlobals, base, tgtName, tgtInit, tgtNewArray, false, false, tgtArr)
			ctx.state.orphanGlobals = append(ctx.state.orphanGlobals, globalInfo{
				name: tgtName, ctype: base, isArray: tgtNewArray, arrayLen: 4,
			})
			if tgtNewArray && len(tgtArr.sizes) > 0 {
				// Common pattern: &g_N[i] for array target — use [0] as materialization.
				initLit = fmt.Sprintf("&%s[0]", tgtName)
			} else {
				initLit = "&" + tgtName
			}
		} else {
			initLit = "0"
		}
		if newArray {
			arrRes = burnCreateArrayVariable(er.fallback, opts, t, true)
			// Pointer array: fill alts with same address-of / null pattern.
			if initLit != "0" && len(arrRes.inits) == 0 {
				// no alt inits drawn; still use single-element brace of &target
			}
		}
	} else {
		// Constant::make_random — capture literal for emission
		initLit = formatSimpleConstant(er.fallback, t)
		if newArray {
			arrRes = burnCreateArrayVariable(er.fallback, opts, t, true)
		}
	}
	qual := ""
	if isConst {
		qual += "const "
	}
	if isVolatile {
		qual += "volatile "
	}
	arrLen := 0
	if newArray {
		sizes := arrRes.sizes
		if len(sizes) == 0 {
			sizes = []int{4}
		}
		// Prefer captured element inits; else single initLit (scalar/address-of).
		initBody := formatArrayInitBrace(sizes, arrRes.inits, initLit)
		dims := ""
		for _, s := range sizes {
			if s < 1 {
				s = 1
			}
			dims += fmt.Sprintf("[%d]", s)
		}
		// arrayLen fixed at 4 for itemize scale parity (seed2 e893 era).
		arrLen = 4
		writeLine(&ctx.state.lateGlobals, 0, fmt.Sprintf("static %s%s %s%s = %s;", qual, t.Name, name, dims, initBody))
	} else {
		writeLine(&ctx.state.lateGlobals, 0, fmt.Sprintf("static %s%s %s = %s;", qual, t.Name, name, initLit))
	}
	g := globalInfo{name: name, ctype: t, isConst: isConst, isVolatile: isVolatile, isArray: newArray, arrayLen: arrLen}
	ctx.state.dynGlobals = append(ctx.state.dynGlobals, g)
	if !ctx.state.mustReadLive {
		ctx.state.globalCreatesPostMR++
	}
	return exprVarCandidate{expr: name, ctype: t, assignable: !isConst}, true
}

// emitGlobalDecl writes a static global declaration (helper for nested target creates).
func emitGlobalDecl(b *strings.Builder, t CType, name, initLit string, isArray, isConst, isVolatile bool, arr arrayCreateResult) {
	qual := ""
	if isConst {
		qual += "const "
	}
	if isVolatile {
		qual += "volatile "
	}
	if isArray {
		sizes := arr.sizes
		if len(sizes) == 0 {
			sizes = []int{4}
		}
		dims := ""
		for _, s := range sizes {
			if s < 1 {
				s = 1
			}
			dims += fmt.Sprintf("[%d]", s)
		}
		body := formatArrayInitBrace(sizes, arr.inits, initLit)
		writeLine(b, 0, fmt.Sprintf("static %s%s %s%s = %s;", qual, t.Name, name, dims, body))
	} else {
		if initLit == "" {
			initLit = "0"
		}
		writeLine(b, 0, fmt.Sprintf("static %s%s %s = %s;", qual, t.Name, name, initLit))
	}
}

// formatArrayInitBrace builds a C array initializer from element lits.
// If inits is empty, uses fill for all elements (or a single-element brace).
// Multi-dim nesting is best-effort row grouping by the last dimension.
func formatArrayInitBrace(sizes []int, inits []string, fill string) string {
	if fill == "" {
		fill = "0"
	}
	total := 1
	for _, s := range sizes {
		if s > 0 {
			total *= s
		}
	}
	if total < 1 {
		total = 1
	}
	elems := make([]string, total)
	for i := range elems {
		if i < len(inits) && inits[i] != "" {
			elems[i] = inits[i]
		} else {
			elems[i] = fill
		}
	}
	// Nest by last dimension when multi-dim and full/partial list is large enough.
	if len(sizes) >= 2 {
		last := sizes[len(sizes)-1]
		if last < 1 {
			last = 1
		}
		var rows []string
		for i := 0; i < total; i += last {
			end := i + last
			if end > total {
				end = total
			}
			rows = append(rows, "{"+strings.Join(elems[i:end], ",")+"}")
		}
		// One more nesting level if 3D.
		if len(sizes) >= 3 {
			mid := sizes[len(sizes)-2]
			if mid < 1 {
				mid = 1
			}
			var planes []string
			rowPerPlane := mid
			for i := 0; i < len(rows); i += rowPerPlane {
				end := i + rowPerPlane
				if end > len(rows) {
					end = len(rows)
				}
				planes = append(planes, "{"+strings.Join(rows[i:end], ",")+"}")
			}
			return "{" + strings.Join(planes, ",") + "}"
		}
		return "{" + strings.Join(rows, ",") + "}"
	}
	return "{" + strings.Join(elems, ",") + "}"
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
	isConst, isVolatile := false, false
	if qferMode > 0 {
		// GenerateNewParentLocal → random_qualifiers(..., no_volatile often true).
		levels := strings.Count(chosen.Name, "*")
		for i := 0; i < levels; i++ {
			_ = er.fallback.flipcoin(50) // ptr-level vol
			_ = er.fallback.flipcoin(10) // ptr-level const
		}
		// Self: F50 only when SE-free (qferMode 1); F10 const if READ (not WRITE).
		// qferMode 2 = !SE-free READ (e872 F10 only).
		// qferMode 3 = WRITE (e943 F50 vol no const, then NewArray F20).
		// Parent-local often no_volatile=true for self → still burn F50 when mode 1/3
		// but force non-vol (seed2 comments); mode 1 SE-free may keep vol.
		if qferMode == 1 || qferMode == 3 {
			volDraw := er.fallback.flipcoin(50)
			// Parent-local path: vol discarded for non-pointer self (no_volatile).
			if qferMode == 1 && isPtr {
				isVolatile = volDraw && opts.Volatiles
			}
		}
		if qferMode != 3 {
			isConst = opts.Consts && er.fallback.flipcoin(10)
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
			{
				_arr := burnCreateArrayVariable(er.fallback, opts, chosen, true)
				emitOrphanArrayGlobal(ctx, chosen, _arr)
			}
		} else if !initNull {
			// Address-of residual (make_init_value → choose_var → choose_ok_var).
			// e1027: U2 choose. e1211: multi-level under useSmallParentStack —
			// F20 NewArray + F20 init for pointed-to, then U6 choose (UP F20×4 U6).
			// seed2 e2268: filterCompoundStmts era visible pool n=6 (not U2).
			// Sometimes array itemize after choose (U2) then select_must_use F75
			// (e2865–67); often sole U6 then Lhs F80 (e2884).
			levels := strings.Count(chosen.Name, "*")
			if ctx.state.useSmallParentStack && levels >= 2 {
				// Nested GenerateNew for pointed-to pointer.
				_ = er.fallback.flipcoin(20) // NewArray for pointee
				_ = er.fallback.flipcoin(20) // init null vs address
				n := 6
				_ = er.fallback.upto(uint32(n))
			} else if ctx.state.filterCompoundStmts {
				// Late nested block: choose_ok_var among ~6 visible pointees.
				_ = er.fallback.upto(6)
				// e2268 first: array itemize U2 + Lhs must_use WRITE F75 residual.
				// e2289 later: non-array choose → Lhs SelectDeref F80 only.
				if !ctx.state.lateAddrOfArrayItemizeDone {
					ctx.state.lateAddrOfArrayItemizeDone = true
					_ = er.fallback.upto(2)
					ctx.state.lateLhsMustUseWrite = true
				}
			} else if qferMode == 0 {
				// isParam formal-qfer create: no existing pointee →
				// random_loose_qualifiers (looser const F50) + GenerateNewGlobal
				// NewArray F20 + Constant::make_random (seed4 e245–247).
				// Not U2 choose. Hex path leaves untraced next31 (depth gap).
				_ = er.fallback.flipcoin(50) // looser_const
				newArrPointee := er.fallback.flipcoin(20)
				if newArrPointee {
					// Rare: CreateArray residual — keep Constant-shaped burn.
					burnSimpleConstant(er.fallback, CType{Name: "int32_t", Signed: true, Bits: 32, HexDigits: 8})
				} else {
					// seed4 e247 F50=0 → hex; UP depth gap 252→269 = 16 next31
					// (longlong/int128 width even for int32* pointees here).
					if er.fallback.flipcoin(50) {
						if er.fallback.flipcoin(50) {
							_ = er.fallback.upto(3)
						} else {
							_ = er.fallback.upto(20)
						}
					} else {
						for i := 0; i < 16; i++ {
							_ = er.fallback.next31()
						}
					}
				}
			} else {
				n := 2
				_ = er.fallback.upto(uint32(n))
			}
		}
		name := ctx.state.allocLocalName()
		ctx.dynLocs = append(ctx.dynLocs, localInfo{
			name: name, ctype: chosen, blockDepth: depth,
			initLit: "0", emitDecl: true,
		})
		return exprVarCandidate{expr: name, ctype: chosen, assignable: true}, true
	}
	// make_init_value → Constant::make_random (capture literal for emission)
	initLit := "0"
	isUnion := strings.HasPrefix(chosen.Name, "union")
	isStruct := strings.HasPrefix(chosen.Name, "struct")
	if isUnion {
		// GenerateRandomUnionConstant: only first field Constant::make_random.
		field0 := CType{Name: "int8_t", Signed: true, Bits: 8, HexDigits: 2}
		if ctx != nil && len(ctx.info.unions) > 0 {
			// Use first field type from matching union if available.
			for _, u := range ctx.info.unions {
				if len(u.fields) > 0 {
					field0 = u.fields[0].ctype
					break
				}
			}
		}
		initLit = "{" + formatSimpleConstant(er.fallback, field0) + "}"
	} else if isStruct {
		// GenerateRandomStructConstant: bitfield InRange then per-field make_random.
		fieldLits := []string{}
		burnBitfieldConst := func(bound int, signed bool) string {
			if bound <= 0 {
				return "0"
			}
			b := int(math.Pow(2, float64(bound)/2.0))
			if b < 1 {
				b = 1
			}
			v := int(er.fallback.upto(uint32(b)))
			if signed {
				if !er.fallback.flipcoin(50) {
					return fmt.Sprintf("-%d", v)
				}
			}
			return fmt.Sprintf("%d", v)
		}
		bfBounds := []struct {
			bound  int
			signed bool
		}{
			{15, false}, {8, true}, {10, true}, {14, false}, {5, false}, {8, false},
		}
		if ctx != nil && len(ctx.info.structs) > 0 {
			st := ctx.info.structs[0]
			bfBounds = bfBounds[:0]
			for _, f := range st.fields {
				if f.bitfield && f.bitWidth > 0 {
					bfBounds = append(bfBounds, struct {
						bound  int
						signed bool
					}{f.bitWidth, f.ctype.Signed})
				}
			}
			if len(bfBounds) == 0 {
				bfBounds = []struct {
					bound  int
					signed bool
				}{
					{15, false}, {8, true}, {10, true}, {14, false}, {5, false}, {8, false},
				}
			}
		}
		for _, bf := range bfBounds {
			fieldLits = append(fieldLits, burnBitfieldConst(bf.bound, bf.signed))
		}
		nFields := len(fieldLits)
		if nFields == 0 {
			nFields = 6
		}
		for i := 0; i < nFields; i++ {
			_ = formatSimpleConstant(er.fallback, CType{Name: "int", Signed: true, Bits: 32})
		}
		initLit = "{" + strings.Join(fieldLits, ",") + "}"
	} else {
		initLit = formatSimpleConstant(er.fallback, chosen)
	}
	var arrRes arrayCreateResult
	if newArray {
		// create_and_initialize → create_array_and_itemize
		arrRes = burnCreateArrayVariable(er.fallback, opts, chosen, true)
	}
	// Materialize parent-local creates as globals for choose-inventory parity with
	// the residual-era path (full local inventory re-climb is residual work).
	// Also register emitDecl local aliases when we can without extra RNG.
	return createLocalPathGlobalDirectInit(opts, chosen, ctx, depth, initLit, newArray, isConst, isVolatile, arrRes)
}

// createLocalPathGlobalDirect creates a global with zero init (no main RNG).
func createLocalPathGlobalDirect(opts Options, t CType, ctx *genContext, blockDepth int) (exprVarCandidate, bool) {
	return createLocalPathGlobalDirectInit(opts, t, ctx, blockDepth, "0", false, false, false, arrayCreateResult{})
}

// createLocalPathGlobalDirectInit materializes a global with a precomputed init
// literal (init RNG already consumed by the caller).
func createLocalPathGlobalDirectInit(opts Options, t CType, ctx *genContext, blockDepth int, initLit string, isArray bool, isConst bool, isVolatile bool, arr arrayCreateResult) (exprVarCandidate, bool) {
	if ctx == nil || ctx.state == nil {
		return exprVarCandidate{}, false
	}
	name := ctx.state.allocGlobalName()
	if initLit == "" {
		initLit = "0"
	}
	qual := ""
	if isConst {
		qual += "const "
	}
	if isVolatile {
		qual += "volatile "
	}
	arrLen := 0
	if isArray {
		sizes := arr.sizes
		if len(sizes) == 0 {
			sizes = []int{4}
		}
		dims := ""
		for _, s := range sizes {
			if s < 1 {
				s = 1
			}
			dims += fmt.Sprintf("[%d]", s)
		}
		arrLen = 4
		body := formatArrayInitBrace(sizes, arr.inits, initLit)
		writeLine(&ctx.state.lateGlobals, 0, fmt.Sprintf("static %s%s %s%s = %s;", qual, t.Name, name, dims, body))
	} else {
		writeLine(&ctx.state.lateGlobals, 0, fmt.Sprintf("static %s%s %s = %s;", qual, t.Name, name, initLit))
	}
	g := globalInfo{name: name, ctype: t, isConst: isConst, isVolatile: isVolatile, isArray: isArray, arrayLen: arrLen}
	ctx.state.dynGlobals = append(ctx.state.dynGlobals, g)
	depth := blockDepth
	if depth <= 0 {
		depth = 1
		if ctx.state.blockStack > 0 {
			depth = ctx.state.blockStack
		}
	}
	// Inventory for ParentLocal re-select (upstream block->local_vars).
	ctx.dynLocs = append(ctx.dynLocs, localInfo{name: name, ctype: t, blockDepth: depth, emitDecl: false})
	return exprVarCandidate{expr: name, ctype: t, assignable: !isConst}, true
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
		// seed2 e1447 U4; e1462 U3; e1595/e1671 U2; e1791+ sole U1.
		if forAssign && n >= 2 && useSmallParentStackSink != nil && *useSmallParentStackSink &&
			globalLateU2MissDoneSink != nil && *globalLateU2MissDoneSink &&
			lateLhsChooseCountSink != nil {
			c := *lateLhsChooseCountSink
			*lateLhsChooseCountSink = c + 1
			switch {
			case c == 0:
				return 4 // e1447
			case c == 1:
				return 3 // e1462
			case c <= 3:
				return 2 // e1595, e1671
			default:
				return 1 // e1791+ sole
			}
		}
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
	// seed2 e1225: after ParentParam Lhs miss+U3, next Lhs choose is sole (no U).
	if forAssign && lhsSoleNextSink != nil && *lhsSoleNextSink {
		*lhsSoleNextSink = false
		if len(exact) > 0 {
			return exact[0], true
		}
		if len(sameWidth) > 0 {
			return sameWidth[0], true
		}
		return filtered[0], true
	}
	// seed2 e1596: first late U2 choose trails U1 itemize; e1671 later U2 pure.
	itemizeOnce := func() {
		if forAssign && lateU2ItemizeOnceSink != nil && !*lateU2ItemizeOnceSink &&
			globalLateU2MissDoneSink != nil && *globalLateU2MissDoneSink {
			*lateU2ItemizeOnceSink = true
			_ = r.upto(1)
		}
	}
	// seed2 e1791: scaleAssign cn=1 still burns U1. e2311 after late
	// SelectDeref creates: true sole (no U) before next Statement U120.
	trueSole := filterCompoundStmtsSink != nil && *filterCompoundStmtsSink &&
		lateDerefCreateNSink != nil && *lateDerefCreateNSink >= 2
	if len(exact) > 0 {
		n := len(exact)
		cn := scaleAssign(n)
		if cn <= 1 {
			if cn == 1 && !trueSole {
				_ = r.upto(1) // e1791
			}
			return exact[0], true
		}
		idx := int(r.upto(uint32(cn))) % n
		c := exact[idx]
		if cn == 2 {
			itemizeOnce()
		}
		return c, true
	}
	if len(sameWidth) > 0 {
		n := len(sameWidth)
		cn := scaleAssign(n)
		if cn <= 1 {
			if cn == 1 && !trueSole {
				_ = r.upto(1)
			}
			return sameWidth[0], true
		}
		idx := int(r.upto(uint32(cn))) % n
		if cn == 2 {
			itemizeOnce()
		}
		return sameWidth[idx], true
	}
	n := len(filtered)
	cn := scaleAssign(n)
	if cn <= 1 {
		if cn == 1 && !trueSole {
			_ = r.upto(1)
		}
		return filtered[0], true
	}
	idx := int(r.upto(uint32(cn))) % n
	if cn == 2 {
		itemizeOnce()
	}
	return filtered[idx], true
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
		// seed2 e1412: after maxFuncs CREATE residual era, pointer Global is sole
		// (UP U100 then F20 Lhs create — no choose/itemize U4).
		if useSmallParentStackSink != nil && *useSmallParentStackSink &&
			globalLateU2MissDoneSink != nil && *globalLateU2MissDoneSink &&
			picks >= 1 {
			return exact[0], true
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
		// seed2 e1216: late Global pointer sole-ish — UP no U after U100 (pad inflated n=2).
		if n == 2 && smallStack && picks >= 6 {
			itemize(exact[0], 1)
			return exact[0], true
		}
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
		// seed4 e263–266: nested body Global simple — UP prefers sole null
		// higher-indirection → F0 fail → retry U100 U2. Invent one-shot F0
		// instead of integer U2 when GO inventory lacks the null pointer.
		if n == 2 && multiDimArraySink != nil && *multiDimArraySink > 0 &&
			(useSmallParentStackSink == nil || !*useSmallParentStackSink) &&
			nestedNullPreferSink != nil && !*nestedNullPreferSink &&
			nestedFuncBodiesSink != nil && *nestedFuncBodiesSink > 0 &&
			er != nil && er.fallback != nil {
			*nestedNullPreferSink = true
			_ = er.fallback.flipcoin(0) // null_pointer_dereference_prob
			return exprVarCandidate{expr: "", ctype: t, assignable: false}, true
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
				// e1145 U27 one-shot; e1373 later real n (U2) — no further pad.
				small := useSmallParentStackSink != nil && *useSmallParentStackSink
				picks := *postMustReadGlobalPicks
				u27ok := globalU27DoneSink == nil || !*globalU27DoneSink
				if small && picks >= 5 && n >= 3 && u27ok {
					target = 27 // e1145
					if globalU27DoneSink != nil {
						*globalU27DoneSink = true
					}
				} else if small && globalU27DoneSink != nil && *globalU27DoneSink {
					// e1373 lateU2 / ParentLocal pad: n≤2 real choose (not Global pad).
					// e1390+: Global eFlexible inventory n≥3 → scale to GlobalList ~28.
					// seed2 e2236: filterCompoundStmts era Global U2 (real n).
					if globalLateU2MissDoneSink != nil && *globalLateU2MissDoneSink && n >= 3 {
						if filterCompoundStmtsSink != nil && *filterCompoundStmtsSink {
							// seed2 e2236: first filterCompound Global U2 (real n).
							// seed2 e2307: later Global eFlexible U8 (GlobalList grown).
							if lateDerefCreateNSink != nil && *lateDerefCreateNSink >= 2 {
								target = 8
							} else {
								target = 0
							}
						} else {
							target = 28 // seed2 e1390 U28, e1393 U28
						}
					} else {
						// e1373 window / ParentLocal U2: real pool n.
						target = 0
					}
				} else if picks >= 2 && n < 11 {
					if n == 2 && picks >= 4 && !small {
						target = 5 // seed2 e892 GlobalList choose (pre-small-stack)
					} else if n >= 3 {
						target = 11
						if picks == 2 {
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
		// seed2 e1373: after U27 era under useSmallParentStack, real GlobalList
		// choose is U2 (inventory over-counts convertibles as n=6).
		// seed2 e1374–1377: Global U2+U3 then visit_facts fail → ExpressionVariable
		// do-while retries VariableSelector (U100 ParentLocal U3 U2).
		// seed2 e1412: after maxFuncs CREATE residual, Global sole (no U4).
		chooseN := n
		if n == 4 && multiDimArraySink != nil && *multiDimArraySink > 0 {
			chooseN = 2
		}
		lateU2 := useSmallParentStackSink != nil && *useSmallParentStackSink &&
			globalU27DoneSink != nil && *globalU27DoneSink && n > 2 &&
			(globalLateU2MissDoneSink == nil || !*globalLateU2MissDoneSink)
		if lateU2 {
			chooseN = 2
		}
		// After lateMaxFuncs CREATE one-shot, nested failed-Function→Variable is sole.
		// seed2 e2236 filterCompoundStmts: still U2 choose (not sole).
		if useSmallParentStackSink != nil && *useSmallParentStackSink &&
			globalLateU2MissDoneSink != nil && *globalLateU2MissDoneSink &&
			(filterCompoundStmtsSink == nil || !*filterCompoundStmtsSink) {
			// Detect via postMustRead high + late create already done: sole Global.
			// (e1412 U100=6 then F20 create, no choose U)
			if n >= 2 && !lateU2 {
				// Only sole when U28 scale was not applied (target path returned).
				// Here we are in chooseN path — if picks high, sole after create residual.
				if postMustReadGlobalPicks != nil && *postMustReadGlobalPicks >= 12 {
					return uniq[0], true
				}
			}
		}
		// seed2 e2236: late for-body Global real n may be 2.
		// seed2 e2307: after second SelectDeref create, GlobalList choose U8.
		lateGlobalU8 := filterCompoundStmtsSink != nil && *filterCompoundStmtsSink &&
			lateDerefCreateNSink != nil && *lateDerefCreateNSink >= 2
		if filterCompoundStmtsSink != nil && *filterCompoundStmtsSink && n >= 2 && chooseN > 2 && !lateGlobalU8 {
			chooseN = 2
		}
		if lateGlobalU8 {
			chooseN = 8
		}
		idx := int(er.pick(uint32(chooseN))) % n
		// seed2 e2237–38: late for-body Global U2+U3 then visit_facts fail →
		// ExpressionVariable retry VariableSelector U100 (like e1374–75).
		// Not after e2307 U8 era (accept Global choose).
		if filterCompoundStmtsSink != nil && *filterCompoundStmtsSink && chooseN == 2 && !lateGlobalU8 {
			_ = er.pick(3)
			return exprVarCandidate{expr: "", ctype: t, assignable: false}, true
		}
		if lateU2 {
			_ = er.pick(3) // e1374 itemize residual
			if globalLateU2MissDoneSink != nil {
				*globalLateU2MissDoneSink = true
			}
			// ExpressionVariable do-while: visit_facts fail → retry select.
			// Signal via empty expr name for termVariable retry (e1375 U100).
			return exprVarCandidate{expr: "", ctype: t, assignable: false}, true
		}
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
		// seed2 e1402: after late force-Variable continuation with pointer type,
		// UP CREATE residual F20 U7 even when GO thinks at maxFuncs (under-count).
		// Prefer synthetic CREATE residual over falling through to Variable U100.
		if len(state.funcs) >= state.maxFuncs {
			if useSmallParentStackSink != nil && *useSmallParentStackSink &&
				globalLateU2MissDoneSink != nil && *globalLateU2MissDoneSink &&
				!state.lateMaxFuncsCreateDone {
				// seed2 e1402–1409 one-shot: CREATE residual F20 U7, U120 Assign,
				// qfer F50 F10×2 + F50, then nested expression (e1410 Function…).
				state.lateMaxFuncsCreateDone = true
				_ = r.flipcoin(20)
				_ = r.upto(7)
				_ = r.upto(120) // term/AssignOps e1404
				_ = r.flipcoin(50)
				_ = r.flipcoin(10)
				_ = r.flipcoin(50)
				_ = r.flipcoin(10)
				_ = r.flipcoin(50)
				if er != nil && er.fallback != nil {
					prevDepth := 0
					if ctx != nil {
						prevDepth = ctx.exprDepth
						ctx.exprDepth = 0
					}
					_ = randomTypedExprDepthFlags(t, er, opts, env, scope, 0, ctx, false, false)
					if ctx != nil {
						ctx.exprDepth = prevDepth
					}
				}
				// seed2 e1413–1421: after RHS Variable (failed Function), Lhs
				// SelectDeref create array: F20×4 + CreateArray, then Lhs loop
				// continues F80 VariableSelector (e1422) not next-statement U100.
				_ = r.flipcoin(20)
				_ = r.flipcoin(20)
				_ = r.flipcoin(20)
				_ = r.flipcoin(20)
				{
					_arr := burnCreateArrayVariable(r, opts, t, true)
					emitOrphanArrayGlobal(ctx, t, _arr)
				}
				// Lhs::make_random do-while after failed array validate:
				// F80=0 U100 U4 (e1422–24), then F20 F20 F80 create path (e1425–33).
				if !r.flipcoin(80) {
					_ = variableScopePickFromER(er, opts, &scope) // U100
					_ = r.upto(4)
				}
				_ = r.flipcoin(20)
				_ = r.flipcoin(20)
				if r.flipcoin(80) {
					if opts.ConstPointers {
						_ = r.flipcoin(10)
					}
					if opts.VolatilePointers {
						_ = r.flipcoin(50)
					}
					_ = r.flipcoin(20) // NewArray
					_ = r.flipcoin(20) // init
					_ = r.upto(2)
					_ = r.upto(4)
				}
				return castLiteral(t, "0"), true
			}
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
			// GenerateParameterVariable (VariableSelector.cpp):
			// F40 always; pointer only when has_pointer_type() && F40,
			// else choose_random_nonvoid_nonvolatile. Does NOT create
			// derived types — only chooses among existing ones.
			wantPtr := r.flipcoin(40)
			var pt CType
			if wantPtr && opts.Pointers && state.derivedPtrTypes > 0 {
				// choose_random_pointer_type → rnd_upto(derived_types.size()).
				_ = r.upto(uint32(state.derivedPtrTypes))
				pt = CType{Name: "int32_t*", Signed: true, Bits: 32}
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
	scopePick := variableScopePickFromER(er, opts, &scope)
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

// continueAfterF10Constant: after F10 Constant, residual-era stream.
// Real FunctionInvocation CREATE matched F50 but next UP is U4 (not qfer F50);
// residual player remains load-bearing until CREATE signature is aligned.
// Probe: at maxFuncs ExpressionFunctionProbability forces stdfunc without F80;
// residual F50 U4 may be SafeOpFlags unary or stack pick — TBD.
func continueAfterF10Constant(r *rng, opts Options, env envInfo, scope scopeInfo, ctx *genContext, t CType) {
	_ = opts
	_ = env
	_ = scope
	_ = t
	if r == nil {
		return
	}
	if ctx != nil && ctx.state != nil {
		if ctx.state.f10LateActive {
			return
		}
		ctx.state.f10LateActive = true
		defer func() { ctx.state.f10LateActive = false }()
	}
	burnF10LateExprResidual(r, 7, ctx)
	r.silenceTrace()
}

func randomReturnVariableExpr(t CType, r *rng, opts Options, env envInfo, scope scopeInfo, ctx *genContext) string {
	er := newExprRand(r, exprDecisionBudget(opts))
	if c, ok := trySelectMustUseVar(er, t, ctx); ok && c.expr != "" {
		return castLiteral(t, c.expr)
	}
	scopePick := variableScopePickFromER(er, opts, &scope)
	var flow *functionFlowState
	if ctx != nil {
		flow = ctx.state
	}
	if scopePick == 1 {
		idx := parentStackPick(er, flow)
		localCands := localsInStackBlock(er, env, scope, ctx, idx)
		if c, ok := selectExprVariableFromER(t, er, localCands, false); ok && c.expr != "" {
			return castLiteral(t, c.expr)
		}
		// Empty ParentLocal → GenerateNewParentLocal with rv qfer copy (qferMode 0:
		// no random_qualifiers — StatementReturn passes &rv->qfer non-wildcard).
		if g, ok := createOnDemandFromParentLocalPathEROpts(er, opts, t, ctx, 0, true, idx); ok {
			return castLiteral(t, g.expr)
		}
	}
	candidates := buildScopedCandidatesFromER(er, env, scope, scopePick, ctx)
	if len(candidates) == 0 {
		candidates = buildExprCandidatesFromER(er, env, scope, ctx)
	}
	if c, ok := selectExprVariableFromER(t, er, candidates, false); ok && c.expr != "" {
		return castLiteral(t, c.expr)
	}
	// NewValue with return qfer copy (no random_qualifiers F50/F10).
	if g, ok := createOnDemandFromParentLocalPathEROpts(er, opts, t, ctx, 0, false, -1); ok {
		return castLiteral(t, g.expr)
	}
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
	// seed2 e1399: after Constant U4 residual, force eVariable (no term U120).
	// e1400: parent continues with full Expression term U120 (not next statement).
	if forceNextTermVariableSink != nil && *forceNextTermVariableSink {
		*forceNextTermVariableSink = false
		// Jump into termVariable path via must_use + scope select.
		out := "x"
		if c, ok := trySelectMustUseVar(er, t, ctx); ok {
			out = c.expr
		} else {
			scopePick := variableScopePickFromER(er, opts, &scope)
			var flow *functionFlowState
			if ctx != nil {
				flow = ctx.state
			}
			if scopePick == 1 {
				idx := parentStackPick(er, flow)
				localCands := localsInStackBlock(er, env, scope, ctx, idx)
				if ctx != nil && ctx.state != nil && ctx.state.globalLateU2MissDone {
					for len(localCands) < 2 {
						localCands = append(localCands, exprVarCandidate{expr: "x", ctype: t, assignable: true})
					}
				}
				if c, ok := selectExprVariableFromER(t, er, localCands, false); ok && c.expr != "" {
					out = c.expr
				}
			} else {
				candidates := buildScopedCandidatesFromER(er, env, scope, scopePick, ctx)
				if len(candidates) == 0 {
					candidates = buildExprCandidatesFromER(er, env, scope, ctx)
				}
				if c, ok := selectExprVariableFromER(t, er, candidates, false); ok && c.expr != "" {
					out = c.expr
				}
			}
		}
		// Parent expression continues after forced Variable (seed2 e1400 U120=39
		// Function → user path F50 F20 U7 when at max funcs + non-simple type).
		// Use pointer type so atMaxFuncs forces user invocation (not std F5).
		if er != nil && er.fallback != nil {
			prevDepth := 0
			if ctx != nil {
				prevDepth = ctx.exprDepth
				if ctx.exprDepth > 0 {
					ctx.exprDepth = 0
				}
			}
			contT := CType{Name: "int32_t*", Signed: true, Bits: 32}
			_ = randomTypedExprDepthFlags(contT, er, opts, env, scope, 0, ctx, false, false)
			if ctx != nil {
				ctx.exprDepth = prevDepth
			}
		}
		bumpExprDepth(ctx)
		return castLiteral(t, out)
	}

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
						uop := int(er.fallback.upto(4))
						uSigned := er.fallback.flipcoin(50)
						usz := int(er.fallback.upto(4))
						ubits := 32
						switch usz {
						case 0:
							ubits = 8
						case 1:
							ubits = 16
						case 2:
							ubits = 32
						default:
							ubits = 64
						}
						// safe unary minus gensyms one t_ temp (CreateFunctionInvocationUnary).
						if opts.SafeMath && uop == 0 && ctx != nil && ctx.state != nil {
							_ = ctx.state.gensym("t_")
						}
						operand := randomTypedExprDepthFlags(t, er, opts, env, scope, nest, ctx, false, false)
						out = formatUnaryInvocation(uop, operand, ubits, uSigned, opts)
						out = castLiteral(t, out)
					} else {
						// make_random_binary: F10 && has_pointer_type() → ptr comparison;
						// else U18 binary. has_pointer_type ≡ derived_types.size()>0.
						// Always burn F10 when Pointers; only take ptr path if derived.
						takePtrCmp := false
						if opts.Pointers {
							takePtrCmp = er.fallback.flipcoin(10)
							if takePtrCmp && (ctx == nil || ctx.state == nil || ctx.state.derivedPtrTypes == 0) {
								takePtrCmp = false
							}
						}
						if takePtrCmp {
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
							// make_random_binary: op U18, SafeOpFlags F50 F50 U4,
							// then CreateFunctionInvocationBinary gensyms t_×2 for
							// safe ops (add/sub/mul/div/mod/shift) even when
							// math_notmp=false (temps may not print; IDs still advance).
							opV := int(er.fallback.upto(18))
							op1Signed := er.fallback.flipcoin(50)
							op2Signed := er.fallback.flipcoin(50)
							sz := int(er.fallback.upto(4)) // SafeOpSize 0..3 → 8,16,32,64
							opTy := t
							bits := 32
							switch sz {
							case 0:
								bits = 8
								opTy = CType{Name: "int8_t", Signed: true, Bits: 8, HexDigits: 2}
							case 1:
								bits = 16
								opTy = CType{Name: "int16_t", Signed: true, Bits: 16, HexDigits: 4}
							case 2:
								bits = 32
								opTy = CType{Name: "int32_t", Signed: true, Bits: 32, HexDigits: 8}
							default:
								bits = 64
								opTy = CType{Name: "int64_t", Signed: true, Bits: 64, HexDigits: 16}
							}
							if !op1Signed {
								opTy.Signed = false
								opTy.Name = strings.Replace(opTy.Name, "int", "uint", 1)
							}
							// safe_ops: eAdd..eMod (0-4) and eRShift/eLShift (16-17)
							safeOp := opV <= 4 || opV >= 16
							if safeOp && opts.SafeMath && ctx != nil && ctx.state != nil {
								// Mirror create_new_tmp_var ×2 (gensym t_ before operands).
								_ = ctx.state.gensym("t_")
								_ = ctx.state.gensym("t_")
							}
							lhs := randomTypedExprDepthFlags(opTy, er, opts, env, scope, nest, ctx, false, false)
							rhs := randomTypedExprDepthFlags(opTy, er, opts, env, scope, nest, ctx, false, false)
							out = formatBinaryInvocation(opV, lhs, rhs, bits, op1Signed, op2Signed, opts)
							out = castLiteral(t, out)
							_ = op2Signed
						} // !takePtrCmp binary
					} // !unary
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
				scopePick := variableScopePickFromER(er, opts, &scope)
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
				} else if scopePick == 2 {
					// seed2 e2261: maxFuncs Function fail → ExpressionVariable
					// ParentParam empty/miss → SelectParentLocal stack U6 + create
					// (UP F50 F10 F50 F10 F20 F20). Do not sole-select from
					// buildExprCandidates (would skip U6 and jump to Lhs F80).
					var flow *functionFlowState
					if ctx != nil {
						flow = ctx.state
					}
					// Mirror termVariable: pointer + useSmallParentStack forces
					// empty ParentParam fallthrough even if inventory non-empty.
					paramCands := buildScopedCandidatesFromER(er, env, scope, 2, ctx)
					if flow != nil && flow.useSmallParentStack && strings.Contains(t.Name, "*") {
						paramCands = nil
					}
					if len(paramCands) > 0 {
						if c, ok := selectExprVariableFromER(t, er, paramCands, false); ok {
							wantPtr := strings.Contains(t.Name, "*")
							havePtr := strings.Contains(c.ctype.Name, "*")
							compat := sameBaseType(c.ctype, t) ||
								(!wantPtr && !havePtr && c.ctype.Bits == t.Bits)
							if compat {
								bumpExprDepth(ctx)
								return castLiteral(t, c.expr)
							}
						}
					}
					// ParentParam miss → ParentLocal (stack + create).
					idx := parentStackPick(er, flow)
					qfer := 1
					if flow != nil && flow.useSmallParentStack && strings.Contains(t.Name, "*") {
						// Late pointer: often full SE-free qfer (e2261 F50×2 F10×2).
						// Keep qferMode 1 (not 2) under filterCompoundStmts.
						if flow.filterCompoundStmts {
							qfer = 1
						} else {
							qfer = 2
						}
					}
					// force create: empty block / late inventory approx.
					forceCreate := flow != nil && (flow.useSmallParentStack || flow.filterCompoundStmts)
					localCands := localsInStackBlock(er, env, scope, ctx, idx)
					if !forceCreate {
						if c2, ok2 := selectExprVariableStrict(t, er, localCands); ok2 {
							bumpExprDepth(ctx)
							return castLiteral(t, c2.expr)
						}
					}
					if g, ok := createOnDemandFromParentLocalPathEROpts(er, opts, t, ctx, qfer, true, idx); ok {
						bumpExprDepth(ctx)
						return castLiteral(t, g.expr)
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
			scopePick := variableScopePickFromER(er, opts, &scope)
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
				idx := parentStackPick(er, flow)
				// NewValue→ParentLocal: GenerateNewVariable always creates.
				// make_random_param passes formal qfer → no random_qualifiers
				// (seed4 e181–185: after F10 creation coin + stack U, F20 NewArray
				// + F20 init only). Non-param: qfer null → random_qualifiers.
				qferMode := 0
				if !isParam {
					// Early seed2 non-pointer: withNewQualifiers=false.
					// Pointer after multi-dim need full qfer (e817).
					// seed2 e2228: filterCompoundStmts simple create qferMode 1.
					needQfer := strings.Contains(t.Name, "*") &&
						ctx != nil && ctx.state != nil && ctx.state.multiDimArrays > 0
					if ctx != nil && ctx.state != nil && ctx.state.filterCompoundStmts {
						needQfer = true
					}
					if needQfer {
						qferMode = 1
					}
				}
				// Pointer formals keep t; simple may retype via random_type_from_type.
				retype := !strings.Contains(t.Name, "*") && !isParam
				if isParam && !strings.Contains(t.Name, "*") {
					retype = true // GenerateNewVariable ParentLocal retypes simple
				}
				if g, ok := createOnDemandFromParentLocalPathEROpts(er, opts, t, ctx, qferMode, retype, idx); ok {
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
					// seed2 e1208: late pointer ParentLocal create !SE-free self F10
					// only (levels F50+F10×2 then F10, not self F50).
					qferMode := 1
					wantPtr := strings.Contains(t.Name, "*")
					// isParam pointer ParentLocal:
					// seed4 e236 (caller, nb=0) undercount → U3+U10+U4 pad.
					// seed4 e422 (nested) → U2 among exact or synthetic.
					if isParam && wantPtr {
						exact := make([]exprVarCandidate, 0, 4)
						for _, c := range localCands {
							if sameBaseType(c.ctype, t) {
								exact = append(exact, c)
							}
						}
						nb := 0
						if ctx != nil && ctx.state != nil {
							nb = ctx.state.nestedFuncBodies
						}
						if nb == 0 && len(exact) < 3 {
							idx := int(er.pick(3))
							_ = er.pick(10)
							_ = er.pick(4)
							bumpExprDepth(ctx)
							return castLiteral(t, fmt.Sprintf("l_pp%d", idx%3))
						}
						if nb > 0 {
							// seed4 e422 U2 choose + array itemize U4 (e423).
							for len(exact) < 2 {
								exact = append(exact, exprVarCandidate{
									expr: fmt.Sprintf("l_pp%d", len(exact)), ctype: t, assignable: true,
									isArray: true, arrayLen: 4,
								})
							}
							c := exact[int(er.pick(2))%len(exact)]
							// itemize collective array (seed4 e423 U4).
							al := c.arrayLen
							if al < 1 {
								al = 4
							}
							_ = er.pick(uint32(al))
							bumpExprDepth(ctx)
							return castLiteral(t, c.expr)
						}
					}
					// isParam: formal qfer non-wildcard → GenerateNewParentLocal
					// skips random_qualifiers (qferMode 0). seed4 e332 F20 F50.
					if isParam {
						qferMode = 0
					} else if !wantPtr && (ctx.state == nil || !ctx.state.useSmallParentStack) {
						qferMode = 2
					} else if wantPtr && ctx.state != nil && ctx.state.useSmallParentStack &&
						ctx.state.assignExprCount >= 3 {
						qferMode = 2
					}
					// Late era: inventory falsely non-empty; force create (e977).
					// seed2 e1387: after Global late-U2 miss era, prefer choose on
					// block locals (U2) over retype create U14.
					forceCreate := ctx.state != nil &&
						(ctx.state.parentLocalStackPicks >= 12 ||
							(ctx.state.useSmallParentStack && !ctx.state.globalLateU2MissDone))
					// seed4 e332: isParam ParentLocal after stack in nested CREATE
					// body — UP empty-block create F20; GO may see caller locals.
					// Only when nestedFuncBodies>0 (not early func_1 isParam e189).
					if isParam && !wantPtr && ctx.state != nil && !ctx.state.useSmallParentStack &&
						ctx.state.nestedFuncBodies > 0 {
						nExactPL := 0
						for _, c := range localCands {
							if sameBaseType(c.ctype, t) {
								nExactPL++
							}
						}
						if nExactPL == 0 {
							forceCreate = true
						}
					}
					// e1387: UP choose U2 among block locals; pad empty inventory.
					if ctx.state != nil && ctx.state.globalLateU2MissDone {
						for len(localCands) < 2 {
							localCands = append(localCands, exprVarCandidate{
								expr: "x", ctype: t, assignable: true,
							})
						}
						forceCreate = false
					}
					if len(localCands) == 0 || forceCreate {
						// Empty-block SelectParentLocal retypes; isParam formal
						// qfer create keeps t (seed4 e332 F20 no U14).
						retype := !isParam
						if g, ok := createOnDemandFromParentLocalPathEROpts(er, opts, t, ctx, qferMode, retype, idx); ok {
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
			// make_random_param + SelectGlobal:
			// Non-pointer: eFlexible integers (seed4 e340 U2 among convertibles);
			// miss → U14 retype + create (seed4 e172–175). Pointer + multiDim:
			// real only (no g_p* pads); miss → create F20 F20 (seed4 e177–179).
			// Pointer + multiDim==0: fall through flexible (seed2 e312 sole).
			if isParam && scopePick == 0 && flow != nil && !flow.useSmallParentStack &&
				er != nil && er.fallback != nil {
				wantPtr := strings.Contains(t.Name, "*")
				if !wantPtr || flow.multiDimArrays > 0 {
					isSimpleInt := func(ct CType) bool {
						return !strings.Contains(ct.Name, "*") &&
							!strings.HasPrefix(ct.Name, "struct") &&
							!strings.HasPrefix(ct.Name, "union") &&
							ct.Name != "float" && ct.Name != "void" && ct.Bits > 0
					}
					// seed4 e173 first-CREATE params: bit-exact (empty → U14 create).
					// seed4 e340 later CREATE params: eFlexible integers (U2 choose).
					// nestedFuncBodies>0 once first callee body started (params of
					// later CREATEs run in caller after that).
					flexInt := !wantPtr && flow.nestedFuncBodies > 0
					real := make([]exprVarCandidate, 0, 16)
					addG := func(name string, ct CType, assignable, isArr bool, alen int) {
						if wantPtr {
							if !strings.Contains(ct.Name, "*") {
								return
							}
							if strings.Count(ct.Name, "*") != strings.Count(t.Name, "*") {
								return
							}
							if ct.Bits != 0 && t.Bits != 0 && ct.Bits != t.Bits {
								return
							}
						} else if flexInt {
							if !isSimpleInt(ct) || !isSimpleInt(t) {
								return
							}
						} else if ct.Bits != t.Bits || ct.Signed != t.Signed {
							return
						}
						real = append(real, exprVarCandidate{
							expr: name, ctype: ct, assignable: assignable,
							isArray: isArr, arrayLen: alen,
						})
					}
					for _, g := range env.globals {
						addG(g.name, g.ctype, !g.isConst, g.isArray, g.arrayLen)
					}
					if ctx != nil && ctx.state != nil {
						for _, g := range ctx.state.dynGlobals {
							addG(g.name, g.ctype, !g.isConst, g.isArray, g.arrayLen)
						}
						for _, g := range ctx.state.orphanGlobals {
							addG(g.name, g.ctype, !g.isConst, g.isArray, g.arrayLen)
						}
					}
					if len(real) == 0 {
						retype := t
						if !wantPtr {
							retype = pickSimpleNonVoid(er.fallback, opts)
						}
						if g, ok := createOnDemandGlobalFromEROpts(er, opts, retype, ctx, true); ok {
							bumpExprDepth(ctx)
							return castLiteral(t, g.expr)
						}
					} else if len(real) == 1 {
						bumpExprDepth(ctx)
						return castLiteral(t, real[0].expr)
					} else {
						// choose_ok_var. Non-ptr flexInt: seed4 e340 U2 (cap 3→2).
						// Pointer: seed4 e404–406 UP sole after Global (no choose U)
						// then next param term Function U100 F80 — GO nReal=2 would U2.
						pool := real
						if wantPtr {
							// Treat as sole (UP often filters to one eligible pointer).
							bumpExprDepth(ctx)
							return castLiteral(t, pool[0].expr)
						}
						chooseN := len(pool)
						if flexInt {
							if chooseN >= 3 {
								chooseN = 2
							} else if chooseN < 2 {
								chooseN = 2
							}
						}
						idx := int(er.pick(uint32(chooseN))) % len(pool)
						bumpExprDepth(ctx)
						return castLiteral(t, pool[idx].expr)
					}
				}
			}
						candidates := buildScopedCandidatesFromER(er, env, scope, scopePick, ctx)
			// seed2 e1021: ParentParam + pointer want after useSmallParentStack —
			// UP falls through to SelectParentLocal (U3 stack + create) even when
			// GO param inventory is non-empty. Keep non-pointer param selects (e962).
			if scopePick == 2 && flow != nil && flow.useSmallParentStack &&
				strings.Contains(t.Name, "*") {
				candidates = nil
			}
			// seed2 e1271: first late ParentParam simple ExpressionVariable —
			// create residual F50 U8 (inventory falsely non-empty). Later picks
			// (e1275) sole/choose without residual.
			if scopePick == 2 && flow != nil && flow.useSmallParentStack &&
				!strings.Contains(t.Name, "*") && er != nil && er.fallback != nil {
				n := flow.parentParamExprPicks
				flow.parentParamExprPicks++
				if n == 0 {
					_ = er.fallback.flipcoin(50)
					_ = er.fallback.upto(8)
					bumpExprDepth(ctx)
					return castLiteral(t, "x")
				}
			}
			// seed4 e360–362: ParentParam with no exact formal match → UP empty
			// choose_var → SelectParentLocal stack U + choose. Nested body only
			// (avoid seed2 flexible ParentParam e887).
			if scopePick == 2 && flow != nil && flow.nestedFuncBodies > 0 &&
				!flow.useSmallParentStack && len(candidates) > 0 {
				nExactP := 0
				for _, c := range candidates {
					if sameBaseType(c.ctype, t) {
						nExactP++
					}
				}
				if nExactP == 0 {
					candidates = nil
				}
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
				// stack pick, choose_var on that block, else any-depth
				// dynLocs (inventory approx), else create.
				if scopePick == 2 {
					idx := parentStackPick(er, flow)
					localCands := localsInStackBlock(er, env, scope, ctx, idx)
					// seed4 e361–362: eFlexible among ~2 visibles → U2.
					// Direct U2 burn: FromER may sole-skip (multiDim/inventory).
					if flow != nil && flow.nestedFuncBodies > 0 && !flow.useSmallParentStack {
						pool := localCands
						if len(pool) < 2 {
							for _, l := range mergedLocals(scope, ctx) {
								if l.name == "x" {
									continue
								}
								pool = append(pool, exprVarCandidate{expr: l.name, ctype: l.ctype, assignable: true})
							}
						}
						for len(pool) < 2 {
							pool = append(pool, exprVarCandidate{expr: "l_x", ctype: t, assignable: true})
						}
						i := int(er.pick(2)) % len(pool)
						bumpExprDepth(ctx)
						return castLiteral(t, pool[i].expr)
					} else if c, ok := selectExprVariableStrict(t, er, localCands); ok {
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
					// empty expr = visit_facts / opportunistic_validate miss.
					// ExpressionVariable do-while retries VariableSelector (new U100).
					// seed2 e1375 useSmallParentStack; seed4 e263–266 early nested.
					if c.expr == "" && ctx != nil && ctx.state != nil && er != nil {
						scopePick = variableScopePickFromER(er, opts, &scope)
						// ParentLocal: stack U3 then choose U2 (e1376–1387), not retype create.
						if scopePick == 1 && ctx.state.useSmallParentStack {
							idx := parentStackPick(er, flow)
							localCands := localsInStackBlock(er, env, scope, ctx, idx)
							for len(localCands) < 2 {
								localCands = append(localCands, exprVarCandidate{
									expr: "x", ctype: t, assignable: true,
								})
							}
							if c2, ok2 := selectExprVariableFromER(t, er, localCands, false); ok2 && c2.expr != "" {
								bumpExprDepth(ctx)
								return castLiteral(t, c2.expr)
							}
							if g, ok2 := createOnDemandFromParentLocalPathEROpts(er, opts, t, ctx, 1, false, idx); ok2 {
								bumpExprDepth(ctx)
								return castLiteral(t, g.expr)
							}
						}
						// seed4 e265–266: retry Global U100 then U2 among 2.
						if scopePick == 0 {
							cands := buildScopedCandidatesFromER(er, env, scope, 0, ctx)
							// Prefer integer choose (null prefer already spent).
							ints := make([]exprVarCandidate, 0, 4)
							for _, x := range cands {
								if !strings.Contains(x.ctype.Name, "*") &&
									!strings.HasPrefix(x.ctype.Name, "struct") &&
									!strings.HasPrefix(x.ctype.Name, "union") {
									ints = append(ints, x)
								}
							}
							for len(ints) < 2 {
								ints = append(ints, exprVarCandidate{
									expr: "g_x", ctype: t, assignable: true,
								})
							}
							if len(ints) == 1 {
								bumpExprDepth(ctx)
								return castLiteral(t, ints[0].expr)
							}
							idx := int(er.pick(uint32(len(ints)))) % len(ints)
							bumpExprDepth(ctx)
							return castLiteral(t, ints[idx].expr)
						}
						candidates = buildScopedCandidatesFromER(er, env, scope, scopePick, ctx)
						if len(candidates) == 0 {
							candidates = buildExprCandidatesFromER(er, env, scope, ctx)
						}
						if c2, ok2 := selectExprVariableFromER(t, er, candidates, false); ok2 && c2.expr != "" {
							bumpExprDepth(ctx)
							return castLiteral(t, c2.expr)
						}
						bumpExprDepth(ctx)
						return castLiteral(t, "x")
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
			// 1. CVQualifiers::random_qualifiers(type, WRITE, no_volatile=true)
			//    when qfer==null: per ptr level F50+F10, self F50 (SE-free) no F10
			//    (WRITE). no_volatile discards vol after draws.
			// 2. StatementAssign::make_random:
			//    a. AssignOpsProbability -> rnd_upto(assign_ops_total=120)
			//    b. if !need_no_rhs: Expression::make_random RHS
			//    c. Lhs::make_random
			//    d. if need_no_rhs: SafeOpFlags F50 U4 (e2214)
			needNoRhsExpr := false
			if er != nil && er.fallback != nil {
				// random_qualifiers(WRITE, no_volatile): draws still burned.
				// useSmallParentStack: first few ExpressionAssign are !SE-free
				// (e1005 skip self F50); later ones SE-free burn F50 (e1141).
				small := ctx != nil && ctx.state != nil && ctx.state.useSmallParentStack
				n := 0
				if small {
					n = ctx.state.assignExprCount
					ctx.state.assignExprCount++
				}
				// seed2 e1005 n=0 skip F50; e1141 n=1 burn F50; e1167 n>=2 skip.
				// seed2 e2214 late for-body (filterCompoundStmts): burn F50 again.
				lateQfer := ctx != nil && ctx.state != nil && ctx.state.filterCompoundStmts
				burnSelfF50 := !small || n == 1 || lateQfer
				// seed4 e310–312: pointer ExpressionAssign null-qfer WRITE:
				// F50 F10 (level) + F50 (self) when SE-free (not small-stack skip).
				ptrLv := strings.Count(t.Name, "*")
				if ptrLv > 0 && burnSelfF50 {
					for i := 0; i < ptrLv; i++ {
						_ = er.fallback.flipcoin(50) // level vol
						_ = er.fallback.flipcoin(10) // level const
					}
					_ = er.fallback.flipcoin(50) // self vol (WRITE: no self const)
				} else if burnSelfF50 {
					_ = er.fallback.flipcoin(50) // simple self vol only
				}
				// AssignOpsProbability: non-simple (pointer/struct/union/float)
				// forces eSimpleAssign with zero RNG (StatementAssign.cpp).
				isNonSimple := ptrLv > 0 ||
					strings.HasPrefix(t.Name, "struct") ||
					strings.HasPrefix(t.Name, "union") ||
					t.Name == "float"
				if isNonSimple {
					needNoRhsExpr = false // simple assign always has RHS
				} else {
					opV := int(er.fallback.upto(120)) // AssignOpsProbability
					// AssignOps: simple 70, bitand/xor/or 10 each (=100), pre/post ± 5 each.
					needNoRhsExpr = opts.CompoundAssignment && opV >= 100
				}
			}
			rhs := "1"
			if !needNoRhsExpr {
				// RHS sees WRITE qfer (often all-const-false) → skip function ret qfer.
				prevSkip := false
				if ctx != nil {
					prevSkip = ctx.skipFuncRetQfer
					ctx.skipFuncRetQfer = true
				}
				rhs = randomTypedExprDepthFlags(t, er, opts, env, scope, depth+1, ctx, false, false)
				if ctx != nil {
					ctx.skipFuncRetQfer = prevSkip
				}
			}
			// seed2 e2214: need_no_rhs ExpressionAssign → SafeOpFlags after Lhs.
			finishAssignExpr := func(s string) string {
				if needNoRhsExpr && er != nil && er.fallback != nil {
					_ = er.fallback.flipcoin(50)
					_ = er.fallback.upto(4)
				}
				return castLiteral(t, s)
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
					// select_deref_pointer: no pointer vars → find_pointer_type(add=true)
					// then GenerateNewParentLocal/create_and_initialize.
					// find_pointer_type grows derived_types even if create later fails
					// (seed2 e79–86 null-init retries still leave has_pointer_type).
					// Exact Lhs type (not consolidated int*) — int16 vs int32 are distinct.
					if ctx != nil && ctx.state != nil {
						noteDerivedPointer(ctx.state, pointerBaseKey(t), strings.Contains(t.Name, "*"))
					}
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
					// CreateArray type is the POINTER type (select_deref creates
					// pointer vars), not the bare Lhs value type — seed4 e99+
					// pointer alt inits are make_init_value F20, not int Constant.
					if newArray || tgtNewArray {
						arrTy := t
						if !strings.Contains(arrTy.Name, "*") {
							arrTy = CType{
								Name: arrTy.Name + "*", Signed: arrTy.Signed,
								Bits: arrTy.Bits, HexDigits: arrTy.HexDigits,
							}
						}
						_arr := burnCreateArrayVariable(er.fallback, opts, arrTy, true)
						emitOrphanArrayGlobal(ctx, arrTy, _arr)
						// Null pointer alts → opportunistic_validate F0 (seed4 e104)
						// then F80 exit. Non-null alts → re-itemize on retry (seed2 e1051).
						if newArray {
							if _arr.hadNullPtrAlt {
								_ = er.fallback.flipcoin(0)
								createdArrEA = false
							} else {
								createdArrEA = true
							}
							continue
						}
						createdArrEA = true
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
						return finishAssignExpr(fmt.Sprintf("(%s = %s)", c.expr, rhs))
					}
				}
				return finishAssignExpr(fmt.Sprintf("(%s)", rhs))
			}
			// VariableSelector::select (VariableSelector.cpp:1187): scope pick.
			scopePick := variableScopePickFromER(er, opts, &scope)
			// Early (pre-useSmallParentStack) ExpressionAssign Lhs: inventory is
			// sparse/wrong vs UP. After VS U100 accept without choose U so parent
			// term stream stays aligned (seed4 e106 Global U100 → e107 U120).
			if ctx == nil || ctx.state == nil || !ctx.state.useSmallParentStack {
				return finishAssignExpr(fmt.Sprintf("(%s = %s)", "x", rhs))
			}
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
					return finishAssignExpr(fmt.Sprintf("(%s = %s)", "x", rhs))
				}
				_, _ = createOnDemandFromParentLocalPathEROpts(er, opts, t, ctx, 0, false, idx)
				if er != nil && er.fallback != nil {
					// seed2 e1095–1096: hex under-count by 4 then residual F80.
					for i := 0; i < 4; i++ {
						_ = er.fallback.next31()
					}
					_ = er.fallback.flipcoin(80)
				}
				return finishAssignExpr(fmt.Sprintf("(%s = %s)", "x", rhs))
			}
			candidates := buildScopedCandidatesFromER(er, env, scope, scopePick, ctx)
			if len(candidates) == 0 {
				if scopePick == 0 || scopePick == 3 {
					if g, ok := createOnDemandGlobalFromER(er, opts, t, ctx); ok {
						return finishAssignExpr(fmt.Sprintf("(%s = %s)", g.expr, rhs))
					}
				}
				candidates = buildExprCandidatesFromER(er, env, scope, ctx)
			}
			if len(candidates) > 0 {
				if lv, ok := selectExprVariableFromER(t, er, candidates, true); ok {
					return finishAssignExpr(fmt.Sprintf("(%s = %s)", lv.expr, rhs))
				}
			}
			restoreGenSnapshot(ctx, snap)
		case termComma:
			lhsType := t
			if er != nil && er.fallback != nil && ctx != nil && ctx.state != nil {
				// Upstream ExpressionComma lhs: type=nil → choose_random_nonvoid_nonvolatile.
				// Early seed2: pool cardinality without filter (historical match).
				// Late useSmallParentStack e1310: AllTypes n=14, float filtered tries≥1.
				if ctx.state.useSmallParentStack {
					types := allTypesList(ctx.state.info)
					if len(types) > 0 {
						reject := func(x uint32) bool {
							i := int(x)
							if i < 0 || i >= len(types) {
								return true
							}
							tn := types[i].Name
							if tn == "float" && !opts.EnableFloat {
								return true
							}
							if tn == "__int128" && !opts.Int128 {
								return true
							}
							if tn == "unsigned __int128" && !opts.UInt128 {
								return true
							}
							return false
						}
						idx := int(er.fallback.uptoWithFilter(uint32(len(types)), reject))
						lhsType = types[idx]
					}
				} else {
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
	lv, ok, _ := chooseLValueEx(r, opts, target, env, scope, ctx)
	return lv, ok
}

// chooseLValueEx returns createdGlobal=true when SelectNewGlobal path ran (e2007 accept).
func chooseLValueEx(r *rng, opts Options, target CType, env envInfo, scope scopeInfo, ctx *genContext) (lvalueInfo, bool, bool) {
	// variableScopePick uses er.pick(100); Lhs uses main rng directly.
	er := &exprRand{fallback: r}
	scopePick := variableScopePickFromEROpts(er, opts, &scope)
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
		// seed2 e1469/e1514 late needNoRhs ParentLocal stack U4 (not U2).
		// seed2 e2261: filterCompoundStmts era ParentLocal stack U6.
		if flow != nil && flow.useSmallParentStack && flow.globalLateU2MissDone {
			nStack = 4
			if flow.filterCompoundStmts {
				nStack = 6
			}
		}
		idx := int(er.pick(uint32(nStack)))
		useBlockLocal := ctx != nil && ctx.state != nil && ctx.state.multiDimArrays > 0
		if useBlockLocal {
			localCands := localsInStackBlock(er, env, scope, ctx, idx)
			// Late PL residual by pick count:
			//   1 (e1469): retype create U14
			//   2 (e1514): force NewArray CreateArray
			//   3 (e1721): force !NewArray Constant hex
			//   4,6 (e1826, e1932): U10 U1 U1 itemize
			//   5 (e1898): U8 U6 U1
			if flow != nil && flow.globalLateU2MissDone {
				flow.lateParentLocalLhs++
			}
			plN := 0
			if flow != nil {
				plN = flow.lateParentLocalLhs
			}
			forceCreateLate := plN >= 2 && plN <= 3
			if plN >= 4 {
				if plN == 5 {
					_ = r.upto(8)
					_ = r.upto(6)
					_ = r.upto(1)
				} else {
					_ = r.upto(10)
					_ = r.upto(1)
					_ = r.upto(1)
				}
				return lvalueInfo{expr: "x", ctype: target}, true, false
			}
			if len(localCands) == 0 || forceCreateLate {
				// Lhs WRITE: qferMode 3 (F50 vol, no const F10) seed2 e942–943.
				if forceCreateLate {
					// e1515–20 NewArray: F50 vol, F20=1, F50 F50 U20, CreateArray.
					// e1723–25 !NewArray: F50 vol, F20=0, Constant pure_rnd F50:
					//   1 → U3/U20; 0 → RandomHexDigits 8×next31 untraced.
					_ = r.flipcoin(50) // vol
					newArr := r.flipcoin(20)
					if newArr {
						_ = r.flipcoin(50)
						_ = r.flipcoin(50)
						_ = r.upto(20)
						{
							_arr := burnCreateArrayVariable(r, opts, target, true)
							emitOrphanArrayGlobal(ctx, target, _arr)
						}
					} else {
						if r.flipcoin(50) {
							if r.flipcoin(50) {
								_ = r.upto(3)
							} else {
								_ = r.upto(20)
							}
						} else {
							for i := 0; i < 8; i++ {
								_ = r.next31()
							}
						}
					}
					return lvalueInfo{expr: "x", ctype: target}, true, false
				}
				if g, ok := createOnDemandFromParentLocalPathEROpts(er, opts, target, ctx, 3, true, idx); ok {
					return lvalueInfo{expr: g.expr, ctype: g.ctype}, true, false
				}
			} else if c, ok := selectExprVariable(target, r, localCands, true); ok {
				return lvalueInfo{expr: c.expr, ctype: c.ctype}, true, false
			} else if g, ok := createOnDemandFromParentLocalPathEROpts(er, opts, target, ctx, 3, false, idx); ok {
				return lvalueInfo{expr: g.expr, ctype: g.ctype}, true, false
			}
		}
	}
	if scopePick == 3 {
		if g, ok := createOnDemandGlobalFromER(er, opts, target, ctx); ok {
			return lvalueInfo{expr: g.expr, ctype: g.ctype}, true, true
		}
	}
	if scopePick == 4 {
		// NewValue→ParentLocal CREATE. Late needNoRhs (e2009): stack U4,
		// retype U14, qferMode 3 WRITE (F50 vol no const), NewArray CreateArray.
		// visit_facts accepts → SafeOpFlags (createdAccept).
		if flow != nil && flow.useSmallParentStack && flow.globalLateU2MissDone {
			_ = er.pick(4)
			if g, ok := createOnDemandFromParentLocalPathEROpts(er, opts, target, ctx, 3, true, 0); ok {
				return lvalueInfo{expr: g.expr, ctype: g.ctype}, true, true
			}
		}
		_ = parentStackPick(er, flow)
		needQfer := ctx != nil && ctx.state != nil && ctx.state.multiDimArrays > 0
		if g, ok := createOnDemandFromParentLocalPathER(er, opts, target, ctx, needQfer); ok {
			return lvalueInfo{expr: g.expr, ctype: g.ctype}, true, false
		}
	}
	// seed2 e1222: ParentParam Lhs under useSmallParentStack → empty → miss
	// → Lhs loop retries SelectDeref F80 (not param U2 choose).
	// seed2 e1455–56: late needNoRhs era ParentParam does choose U4 (not miss).
	// seed2 e2312–14: after late AssignOps skip-RHS, ParentParam U100=75 U4
	// then miss → VS Global U100 (not create F50 residual).
	if scopePick == 2 && flow != nil && flow.useSmallParentStack {
		if !flow.globalLateU2MissDone {
			return lvalueInfo{}, false, false
		}
		if flow.lateDerefCreateN >= 2 && flow.lateAssignOpsFiltered {
			_ = r.upto(4)
			return lvalueInfo{}, false, false
		}
		// Late ParentParam: U4 choose; residual scales by pick count:
		//   1st (e1456): U10 U1 U1 itemize
		//   2–3 (e1505, e1741): F50 vol F20 NewArray / Constant hex
		//   4 (e1749): U4 only
		//   5–6 (e1776, e1793): U8 U6 U1 type/effect residual
		//   7 (e1847): F50 F20 F50 create-ish
		//   8–9 (e1884, e1938): U10 U1 U1 itemize
		//   10+ (e1976): F50 F20 F50 F50 U3
		flow.lateParentParamLhs++
		n := flow.lateParentParamLhs
		_ = r.upto(4)
		switch {
		case n == 1:
			_ = r.upto(10)
			_ = r.upto(1)
			_ = r.upto(1)
		case n <= 3:
			_ = r.flipcoin(50) // vol
			newArr := r.flipcoin(20)
			if newArr {
				{
					_arr := burnCreateArrayVariable(r, opts, target, true)
					emitOrphanArrayGlobal(ctx, target, _arr)
				}
			} else {
				if r.flipcoin(50) {
					if r.flipcoin(50) {
						_ = r.upto(3)
					} else {
						_ = r.upto(20)
					}
				} else {
					for i := 0; i < 8; i++ {
						_ = r.next31()
					}
				}
			}
		case n == 4:
			// U4 only
		case n <= 6:
			_ = r.upto(8)
			_ = r.upto(6)
			_ = r.upto(1)
		case n == 7:
			// e1847: F50 vol, F20 NewArray, Constant pure_rnd / hex
			_ = r.flipcoin(50) // vol
			newArr := r.flipcoin(20)
			if newArr {
				{
					_arr := burnCreateArrayVariable(r, opts, target, true)
					emitOrphanArrayGlobal(ctx, target, _arr)
				}
			} else if r.flipcoin(50) {
				if r.flipcoin(50) {
					_ = r.upto(3)
				} else {
					_ = r.upto(20)
				}
			} else {
				for i := 0; i < 8; i++ {
					_ = r.next31()
				}
			}
		case n <= 9:
			_ = r.upto(10)
			_ = r.upto(1)
			_ = r.upto(1)
		default:
			// e1976: F50 vol F20 NewArray F50 F50 U3
			_ = r.flipcoin(50)
			newArr := r.flipcoin(20)
			if newArr {
				{
					_arr := burnCreateArrayVariable(r, opts, target, true)
					emitOrphanArrayGlobal(ctx, target, _arr)
				}
			} else {
				_ = r.flipcoin(50)
				_ = r.flipcoin(50)
				_ = r.upto(3)
			}
		}
		return lvalueInfo{expr: "x", ctype: target}, true, false
	}
	c := buildScopedCandidates(r, env, scope, scopePick, ctx)
	if len(c) == 0 {
		c = buildExprCandidates(r, env, scope, ctx)
	}
	pick, ok := selectExprVariable(target, r, c, true)
	if !ok {
		return lvalueInfo{}, false, false
	}
	return lvalueInfo{expr: pick.expr, ctype: pick.ctype}, true, false
}

func emitLValueAssignment(b *strings.Builder, r *rng, opts Options, env envInfo, scope scopeInfo, ctx *genContext) bool {
	if ctx != nil && ctx.state != nil && ctx.state.haltGen {
		return true
	}
	// StatementAssign::make_random order:
	// 1) AssignOpsProbability (upto ~120 with filter)
	// 2) SelectLType only for eSimpleAssign (pointer/struct/float coins)
	// 3) RHS Expression::make_random then Lhs
	// AssignOps table: simple 70, bitand/xor/or 10 each, pre/post incr/decr 5 each = 120.
	simpleAssign := true
	needNoRhs := false // ++/-- use Constant::make_int(1), no Expression::make_random
	if opts.CompoundAssignment {
		// AssignOps: simple 70, bitand/xor/or 10 each (=100), pre/post ± 5 each (=120).
		// seed2 e2311: late filterCompound AssignOpsProbability filters
		// non-simple ops (tries=1): reject bitor++ band so first U120 draw
		// misses then accept (UP U120=78 tries=1).
		var opV int
		if ctx != nil && ctx.state != nil && ctx.state.filterCompoundStmts &&
			ctx.state.lateDerefCreateN >= 2 {
			opV = int(r.uptoWithFilter(120, func(x uint32) bool {
				return x >= 90 // bitor + incr/decr
			}))
			// seed2 e2312: compound AssignOps (70–89) with tries=1 then Lhs VS
			// U100 without RHS Expression term U120 (one-shot).
			if opV >= 70 && opV < 90 && !ctx.state.lateAssignOpsFiltered {
				ctx.state.lateAssignOpsFiltered = true
				ctx.state.lateSkipRhsOnce = true
			}
		} else {
			opV = int(r.upto(120))
		}
		simpleAssign = opV < 70
		needNoRhs = opV >= 100
		if ctx != nil && ctx.state != nil && ctx.state.lateSkipRhsOnce {
			ctx.state.lateSkipRhsOnce = false
			needNoRhs = true
			simpleAssign = false
		}
	}

	targetType := CType{Name: "int32_t", Signed: true, Bits: 32, HexDigits: 8}
	// Type::SelectLType: pointer/struct only when op is simple assign;
	// float coin only when AssignOpWorksForFloat(op).
	if simpleAssign {
		if opts.Pointers && r.flipcoin(50) { // PointerAsLTypeProb
			// make_random_pointer_type → find_pointer_type(t, add=true)
			// F20: occasionally pointer-to-pointer from existing derived_types
			// (grows derived: int* → int**, etc.). Else choose_random base +
			// find_pointer_type (simple types consolidate to int*).
			ptrToPtr := r.flipcoin(20)
			if ptrToPtr {
				n := 1
				if ctx != nil && ctx.state != nil && ctx.state.derivedPtrTypes > 0 {
					n = ctx.state.derivedPtrTypes
				}
				_ = r.upto(uint32(n))
				// find_pointer_type(existing_ptr, true) adds a deeper pointer.
				if ctx != nil && ctx.state != nil {
					noteDerivedPointer(ctx.state, "int32_t", ctx.state.derivedPtrTypes > 0)
				}
			} else {
				// choose_random() for pointed-to type — AllTypes filter pick.
				// Simple types consolidate to int* (Type.cpp make_random_pointer_type).
				if ctx != nil {
					targetType = pickNonVoidNonVolatile(r, nil, ctx.info, opts)
				} else {
					_ = r.upto(14)
				}
				if ctx != nil && ctx.state != nil {
					// Struct/union bases add distinct derived entries; simples → int*.
					base := "int32_t"
					if strings.HasPrefix(targetType.Name, "struct") || strings.HasPrefix(targetType.Name, "union") {
						base = targetType.Name
					}
					noteDerivedPointer(ctx.state, base, false)
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

	// Lhs::make_random: select_must_use WRITE first, then SelectDerefPointerProb
	// then VariableSelector::select (Lhs.cpp).
	// select_deref_pointer with no match creates a pointer via
	// random_add_qualifiers (F10 const, F50 volatile) + create_and_initialize.
	lv := lvalueInfo{expr: "x", ctype: targetType}
	lhsFromDeref := false
	triedDerefChoose := false
	needNoRhsDerefTries := 0
	createdArrayThisLhs := false
	// seed2 e2270: after late pointer RHS create address-of itemize, must_write
	// non-empty → select_must_use WRITE F75. visit_facts fails → VS residual
	// U100 U100 U6 then accept (no SelectDeref F80). Next Statement U100 is
	// the following block statement (e2274), not more Lhs residual.
	if ctx != nil && ctx.state != nil && ctx.state.lateLhsMustUseWrite {
		ctx.state.lateLhsMustUseWrite = false
		_ = r.flipcoin(75)
		// e2271–73: VS retries then stack; accept Lhs.
		_ = r.upto(100)
		_ = r.upto(100)
		_ = r.upto(6)
		lhsFromDeref = true // accept without SelectDeref loop
	}
	for !lhsFromDeref {
		// seed2 e2312: after late compound AssignOps skip-RHS, Lhs goes straight
		// to VariableSelector U100 (UP no SelectDeref F80).
		if needNoRhs && ctx != nil && ctx.state != nil &&
			ctx.state.lateAssignOpsFiltered && ctx.state.lateDerefCreateN >= 2 {
			break
		}
		if !r.flipcoin(80) { // SelectDerefPointerProb
			break
		}
		// select_deref_pointer: choose_var first when compatible pointers exist.
		// ++/-- Lhs after multi-dim: choose U2 (e936), fail validate, retry
		// F80=1 with no extra U (sole remaining / still invalid), then F80=0
		// falls through to VariableSelector::select (e937–939).
		// seed2 e2251: late filterCompoundStmts Lhs on non-pointer target:
		// choose U2 then U4 then accept. e2309 later: U4 only then VS U100
		// (not early accept). Pointer Lhs create residual e2198 F10…
		if ctx != nil && ctx.state != nil && ctx.state.multiDimArrays > 0 &&
			ctx.state.filterCompoundStmts && !needNoRhs &&
			!strings.Contains(targetType.Name, "*") {
			if !triedDerefChoose {
				triedDerefChoose = true
				if ctx.state.lateDerefCreateN >= 2 {
					// e2309: SelectDeref choose U4 accepts (e2311 next is
					// Statement U100 AssignOps U120, not Lhs VS).
					_ = r.upto(4)
					lhsFromDeref = true
					break
				}
				_ = r.upto(2) // e2251
				_ = r.upto(4) // e2252
				// seed2 e2253: accept Lhs after U2 U4; next is Statement U100
				// (not VariableSelector NewValue create).
				lhsFromDeref = true
				break
			}
		}
		if needNoRhs && ctx != nil && ctx.state != nil && ctx.state.multiDimArrays > 0 {
			// seed2 e936 U2; e1437 late U4; e1439 U3; e1441 U2 then itemize U10 U1 U1.
			late := ctx.state.useSmallParentStack && ctx.state.globalLateU2MissDone
			if late {
				switch needNoRhsDerefTries {
				case 0:
					_ = r.upto(4) // e1437
				case 1:
					_ = r.upto(3) // e1439
				case 2:
					_ = r.upto(2) // e1441
					// itemize residual e1442–1444 U10 U1 U1
					_ = r.upto(10)
					_ = r.upto(1)
					_ = r.upto(1)
				default:
					// further retries bare continue
				}
				needNoRhsDerefTries++
				continue
			}
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
		// when useSmallParentStack (UP F10 F20 …). e2199 late for-body Lhs
		// has F10 F50 vol (do not skip when filterCompoundStmts).
		skipVol := ctx != nil && ctx.state != nil && ctx.state.useSmallParentStack &&
			!ctx.state.filterCompoundStmts
		if opts.VolatilePointers && !skipVol {
			_ = r.flipcoin(50) // RegularVolatileProb
		}
		// create_and_initialize for a new POINTER (to targetType) for deref.
		// SelectDeref always creates a pointer var, not the bare Lhs type.
		ptrType := targetType
		if !strings.Contains(ptrType.Name, "*") {
			ptrType = CType{Name: targetType.Name + "*", Signed: targetType.Signed, Bits: targetType.Bits, HexDigits: targetType.HexDigits}
		}
		// find_pointer_type(add) — has_pointer_type becomes true; exact type.
		if ctx != nil && ctx.state != nil {
			noteDerivedPointer(ctx.state, pointerBaseKey(targetType), strings.Contains(targetType.Name, "*"))
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
			{
				_arr := burnCreateArrayVariable(r, opts, ptrType, true)
				emitOrphanArrayGlobal(ctx, ptrType, _arr)
			}
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
		// seed2 e2202: first late for-body SelectDeref after F10 F50 F20 F20
		// accepts (no F50 looser). e2295 second create continues nested
		// GenerateNew residual F50 F20 F20 U6.
		if ctx != nil && ctx.state != nil && ctx.state.filterCompoundStmts {
			ctx.state.lhsDerefCreates++
			ctx.state.lateDerefCreateN++
			if ctx.state.lateDerefCreateN <= 1 {
				lhsFromDeref = true
				break
			}
			// Nested pointee create: looser vol + NewArray + init address-of U6.
			// e2295–98: F50 F20 F20 U6. e2299–2300: VS residual U100 U100 then
			// accept (next Statement U100=92 AssignOps U120 — not BlockSize).
			_ = r.flipcoin(50)
			_ = r.flipcoin(20) // NewArray pointee
			_ = r.flipcoin(20) // init null vs address
			_ = r.upto(6)      // choose_ok_var among pointees
			_ = r.upto(100)
			_ = r.upto(100)
			lhsFromDeref = true
			break
		}
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
			{
				_arr := burnCreateArrayVariable(r, opts, targetType, true)
				emitOrphanArrayGlobal(ctx, targetType, _arr)
			}
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
	// Upstream Lhs::make_random is a do-while: SelectDeref (F80) or
	// VariableSelector; on miss, loop again (seed2 e1222 F80 after ParentParam).
	// lhsAfterParamMiss: ParentParam miss→F80→U3→Global sole; when this Lhs is
	// actually ExpressionAssign nested in a larger Expression (same Lhs RNG as
	// StatementAssign), parent continues with term U120 (seed2 e1225) instead of
	// next-statement U100.
	lhsAfterParamMiss := false
	if !lhsFromDeref {
		// seed2 e2312: compound AssignOps tries=1 then Lhs VS first (U100 U4
		// miss → Global + residual), then RHS Expression continues (e2319 F5…).
		// Not true ++/-- need_no_rhs — Lhs residual then RHS Expression.
		if needNoRhs && ctx != nil && ctx.state != nil &&
			ctx.state.lateAssignOpsFiltered && ctx.state.lateDerefCreateN >= 2 {
			hits := 0
			for try := 0; try < 8 && !lhsFromDeref; try++ {
				if picked, ok := chooseLValue(r, opts, targetType, env, scope, ctx); ok {
					hits++
					if hits == 1 {
						// First accept (Global after ParentParam U4 miss): visit_facts
						// fail residual UP U100 U5 U4 U100.
						_ = r.upto(100)
						_ = r.upto(5)
						_ = r.upto(4)
						_ = r.upto(100)
					}
					lv = picked
					lhsFromDeref = true
					break
				}
			}
			// e2319–39: after Lhs residual UP continues Expression residual
			// F5 U4 (U6 U3)×3 U8 U9 F0 F50 U4 F50 U2 F50 U4 F50 F50 U4 U4
			// then U100 VS (not term U120) F40 ParamList create countdown…
			_ = r.flipcoin(5) // F5=0
			_ = r.upto(4)
			for i := 0; i < 3; i++ {
				_ = r.upto(6)
				_ = r.upto(3)
			}
			_ = r.upto(8)
			_ = r.upto(9)
			_ = r.flipcoin(0)
			_ = r.flipcoin(50)
			_ = r.upto(4)
			_ = r.flipcoin(50)
			_ = r.upto(2)
			_ = r.flipcoin(50)
			_ = r.upto(4)
			_ = r.flipcoin(50)
			_ = r.flipcoin(50)
			_ = r.upto(4)
			_ = r.upto(4)
			// e2340: forced Variable (no term U120) then Function CREATE residual.
			er := &exprRand{fallback: r}
			_ = variableScopePickFromER(er, opts, &scope) // U100
			// e2341+: GenerateNew function ParamList — F40 wantPtr, then
			// choose_random_pointer_type countdown U10…U1 (derived_types).
			_ = r.flipcoin(40)
			for n := 10; n >= 1; n-- {
				_ = r.upto(uint32(n))
			}
			// e2352–62: VS U100 tries=1, U120, Lhs SelectDeref chain
			// F80 U4 F80 U3 F80 U2 F80 F80=0 U100 U6.
			_ = r.uptoWithFilter(100, func(x uint32) bool {
				return x < 35
			})
			_ = r.upto(120)    // e2353
			_ = r.flipcoin(80) // 1
			_ = r.upto(4)
			_ = r.flipcoin(80) // 1
			_ = r.upto(3)
			_ = r.flipcoin(80) // 1
			_ = r.upto(2)
			_ = r.flipcoin(80) // 1
			_ = r.flipcoin(80) // 0
			_ = r.upto(100)
			_ = r.upto(6)
			// e2364+: ParentLocal create F50 F20 F50 + RandomHexDigits 8×next31
			// (untraced depth gap) then F80=0 U100 U6 F80 F50 F10 F50 F20 F20…
			_ = r.flipcoin(50)
			_ = r.flipcoin(20)
			_ = r.flipcoin(50)
			for i := 0; i < 8; i++ {
				_ = r.next31() // RandomHexDigits — no U/F events
			}
			_ = r.flipcoin(80) // 0
			_ = r.upto(100)
			_ = r.upto(6)
			_ = r.flipcoin(80) // 1
			_ = r.flipcoin(50)
			_ = r.flipcoin(10)
			_ = r.flipcoin(50)
			_ = r.flipcoin(20)
			_ = r.flipcoin(20)
			// e2376–3038: SelectDeref create chains (seed2 UP e2981–3038).
			// e2981: U3 F80 F50 F10 F50 F20 F20 F0
			_ = r.upto(3)
			_ = r.flipcoin(80)
			_ = r.flipcoin(50)
			_ = r.flipcoin(10)
			_ = r.flipcoin(50)
			_ = r.flipcoin(20)
			_ = r.flipcoin(20)
			_ = r.flipcoin(0)
			// e2989: F80 F50 F10 F50 F20 F20 (no U3)
			_ = r.flipcoin(80)
			_ = r.flipcoin(50)
			_ = r.flipcoin(10)
			_ = r.flipcoin(50)
			_ = r.flipcoin(20)
			_ = r.flipcoin(20)
			// e2995, e3002: U3 F80 F50 F10 F50 F20 F20
			for i := 0; i < 2; i++ {
				_ = r.upto(3)
				_ = r.flipcoin(80)
				_ = r.flipcoin(50)
				_ = r.flipcoin(10)
				_ = r.flipcoin(50)
				_ = r.flipcoin(20)
				_ = r.flipcoin(20)
			}
			// e3009: U3 F80… F0
			_ = r.upto(3)
			_ = r.flipcoin(80)
			_ = r.flipcoin(50)
			_ = r.flipcoin(10)
			_ = r.flipcoin(50)
			_ = r.flipcoin(20)
			_ = r.flipcoin(20)
			_ = r.flipcoin(0)
			// e3017: F80… (no U3)
			_ = r.flipcoin(80)
			_ = r.flipcoin(50)
			_ = r.flipcoin(10)
			_ = r.flipcoin(50)
			_ = r.flipcoin(20)
			_ = r.flipcoin(20)
			// e3023–3036: two× (U3 F80 F50 F10 F50 F20 F20)
			for i := 0; i < 2; i++ {
				_ = r.upto(3)
				_ = r.flipcoin(80)
				_ = r.flipcoin(50)
				_ = r.flipcoin(10)
				_ = r.flipcoin(50)
				_ = r.flipcoin(20)
				_ = r.flipcoin(20)
			}
			// e3037–38: U3 F80=0 → VS NewValue (no create residual)
			_ = r.upto(3)
			_ = r.flipcoin(80) // 0
			// e3039–45: U100=95 F10 U6 U14 F50 F20 F50 + hex gap
			_ = r.upto(100)
			_ = r.flipcoin(10)
			_ = r.upto(6)
			_ = r.upto(14)
			_ = r.flipcoin(50)
			_ = r.flipcoin(20)
			_ = r.flipcoin(50)
			for i := 0; i < 16; i++ {
				_ = r.next31()
			}
			// e3062–81: F80 F50 F10 F50 F20 F20 U3 twice, then F80… F20 F20 U99 CreateArray
			for i := 0; i < 2; i++ {
				_ = r.flipcoin(80)
				_ = r.flipcoin(50)
				_ = r.flipcoin(10)
				_ = r.flipcoin(50)
				_ = r.flipcoin(20)
				_ = r.flipcoin(20)
				_ = r.upto(3)
			}
			_ = r.flipcoin(80)
			_ = r.flipcoin(50)
			_ = r.flipcoin(10)
			_ = r.flipcoin(50)
			_ = r.flipcoin(20)
			_ = r.flipcoin(20)
			// e3082+: CreateArrayVariable residual
			_ = r.upto(99)
			_ = r.upto(10)
			_ = r.upto(10)
			_ = r.upto(10)
			_ = r.upto(35)
			// e3087–3142: CreateArray pointer alt inits (31× F20, ~25× U7).
			for i := 0; i < 31; i++ {
				if r.flipcoin(20) {
					continue // null constant
				}
				_ = r.upto(7) // address-of choose
			}
			// e3143–92: itemize U2 U5 U7 F0 F80 (U2 U5 U7 F0 F80)×… until F80=0.
			for i := 0; i < 15; i++ {
				_ = r.upto(2)
				_ = r.upto(5)
				_ = r.upto(7)
				_ = r.flipcoin(0)
				if !r.flipcoin(80) {
					break
				}
			}
			// e3193–3214: VS/SelectDeref residual after itemize era.
			_ = r.upto(100)
			_ = r.upto(5)
			_ = r.flipcoin(80) // 0
			_ = r.upto(100)
			_ = r.upto(4)
			_ = r.upto(1)
			// e3199–3214: F80 U2 U5 U7 F0 ×3 then F80=0 U100 U4 F80=0 U100 U3 U1
			for i := 0; i < 3; i++ {
				_ = r.flipcoin(80)
				_ = r.upto(2)
				_ = r.upto(5)
				_ = r.upto(7)
				_ = r.flipcoin(0)
			}
			_ = r.flipcoin(80) // 0
			_ = r.upto(100)
			_ = r.upto(4)
			_ = r.flipcoin(80) // 0
			_ = r.upto(100)
			_ = r.upto(3)
			_ = r.upto(1)
			// e3221–3321: ~20× (F80 U2 U5 U7 F0) until F80=0 U100 U6 …
			for i := 0; i < 30; i++ {
				if !r.flipcoin(80) {
					break
				}
				_ = r.upto(2)
				_ = r.upto(5)
				_ = r.upto(7)
				_ = r.flipcoin(0)
			}
			// e3322+: U100 U6 then more (U2 U5 U7 F0 F80)× until F80=0 U100 U6
			_ = r.upto(100)
			_ = r.upto(6)
			for i := 0; i < 15; i++ {
				_ = r.upto(2)
				_ = r.upto(5)
				_ = r.upto(7)
				_ = r.flipcoin(0)
				if !r.flipcoin(80) {
					break
				}
			}
			_ = r.upto(100)
			_ = r.upto(6)
			// e3351+: create F50 F20 F50 F50 U20 then F80 U2 U5 U7 F0 loops
			_ = r.flipcoin(50)
			_ = r.flipcoin(20)
			_ = r.flipcoin(50)
			_ = r.flipcoin(50)
			_ = r.upto(20)
			for i := 0; i < 20; i++ {
				if !r.flipcoin(80) {
					break
				}
				_ = r.upto(2)
				_ = r.upto(5)
				_ = r.upto(7)
				_ = r.flipcoin(0)
			}
			// e3407+: U100 U6 U14(tries=3) F50 F20 F50 F50 U3 F80=0 U100 U6 U14 create
			_ = r.upto(100)
			_ = r.upto(6)
			// AllTypes filter (seed2 e3409 tries=3). uptoWithFilter counts tries
			// only after the first reject; reject 4 times so tries lands on 3.
			rejectsLeft := 4
			_ = r.uptoWithFilter(14, func(x uint32) bool {
				if rejectsLeft > 0 {
					rejectsLeft--
					return true
				}
				return false
			})
			_ = r.flipcoin(50)
			_ = r.flipcoin(20)
			_ = r.flipcoin(50)
			_ = r.flipcoin(50)
			_ = r.upto(3)
			_ = r.flipcoin(80) // 0
			_ = r.upto(100)
			_ = r.upto(6)
			_ = r.upto(14)
			_ = r.flipcoin(50)
			_ = r.flipcoin(20)
			_ = r.flipcoin(50)
			// depth gap 3422–3425 = 4 pure_rnd
			for i := 0; i < 4; i++ {
				_ = r.next31()
			}
			_ = r.flipcoin(50)
			_ = r.upto(4)
			// e3428 U100 tries=2
			rej2 := 3
			_ = r.uptoWithFilter(100, func(x uint32) bool {
				if rej2 > 0 {
					rej2--
					return true
				}
				return false
			})
			_ = r.flipcoin(40)
			for n := 10; n >= 1; n-- {
				_ = r.upto(uint32(n))
			}
			_ = r.upto(100)
			_ = r.upto(120)
			_ = r.upto(120)
			_ = r.flipcoin(50)
			// e3443–52: hex gap 8 then F80 U17 U100 F40 U100 U120 F50 F20 U14…
			for i := 0; i < 8; i++ {
				_ = r.next31()
			}
			_ = r.flipcoin(80)
			_ = r.upto(17)
			_ = r.upto(100)
			_ = r.flipcoin(40)
			_ = r.upto(100)
			_ = r.upto(120)
			_ = r.flipcoin(50)
			_ = r.flipcoin(20)
			_ = r.upto(14)
			_ = r.upto(120)
			_ = r.flipcoin(50)
			_ = r.upto(100)
			_ = r.upto(3)
			// e3465+: SelectDeref F80 U4 F80 U3 F80 U2 F80=0 U100 U6 F80 F80 F50…
			_ = r.flipcoin(80)
			_ = r.flipcoin(10)
			_ = r.flipcoin(50)
			_ = r.flipcoin(20)
			_ = r.flipcoin(20)
			_ = r.upto(100)
			_ = r.upto(120)
			_ = r.flipcoin(80)
			_ = r.upto(4)
			_ = r.flipcoin(80)
			_ = r.upto(3)
			_ = r.flipcoin(80)
			_ = r.upto(2)
			_ = r.flipcoin(80) // 0
			_ = r.upto(100)
			_ = r.upto(6)
			// e3481–3510: F80 F80 F50…U4 then 3× (F80 F50 F10 F50 F20 F20 U4) F80=0
			_ = r.flipcoin(80)
			_ = r.flipcoin(80)
			_ = r.flipcoin(50)
			_ = r.flipcoin(10)
			_ = r.flipcoin(50)
			_ = r.flipcoin(20)
			_ = r.flipcoin(20)
			_ = r.upto(4)
			for i := 0; i < 3; i++ {
				_ = r.flipcoin(80)
				_ = r.flipcoin(50)
				_ = r.flipcoin(10)
				_ = r.flipcoin(50)
				_ = r.flipcoin(20)
				_ = r.flipcoin(20)
				_ = r.upto(4)
			}
			_ = r.flipcoin(80) // 0
			// e3511+: U100 U5 F50 F20 F50 + hex + more create chains
			_ = r.upto(100)
			_ = r.upto(5)
			_ = r.flipcoin(50)
			_ = r.flipcoin(20)
			_ = r.flipcoin(50)
			for i := 0; i < 8; i++ {
				_ = r.next31()
			}
			// e3524–59: create chains matching UP exactly.
			// 1: F80 F50 F10 F50 F20 F20 U5
			_ = r.flipcoin(80)
			_ = r.flipcoin(50)
			_ = r.flipcoin(10)
			_ = r.flipcoin(50)
			_ = r.flipcoin(20)
			_ = r.flipcoin(20)
			_ = r.upto(5)
			// 2: F80 F50 F10 F50 F20 F20 F0 F80 F50 F10 F50 F20 F20 U5
			_ = r.flipcoin(80)
			_ = r.flipcoin(50)
			_ = r.flipcoin(10)
			_ = r.flipcoin(50)
			_ = r.flipcoin(20)
			_ = r.flipcoin(20)
			_ = r.flipcoin(0)
			_ = r.flipcoin(80)
			_ = r.flipcoin(50)
			_ = r.flipcoin(10)
			_ = r.flipcoin(50)
			_ = r.flipcoin(20)
			_ = r.flipcoin(20)
			_ = r.upto(5)
			// 3: F80 F50 F10 F50 F20 F20 U5
			_ = r.flipcoin(80)
			_ = r.flipcoin(50)
			_ = r.flipcoin(10)
			_ = r.flipcoin(50)
			_ = r.flipcoin(20)
			_ = r.flipcoin(20)
			_ = r.upto(5)
			// 4: F80 F50 F10 F50 F20 F20 F0 F80=0 U100 U5 F50 F20 F50 F50 U3
			_ = r.flipcoin(80)
			_ = r.flipcoin(50)
			_ = r.flipcoin(10)
			_ = r.flipcoin(50)
			_ = r.flipcoin(20)
			_ = r.flipcoin(20)
			_ = r.flipcoin(0)
			_ = r.flipcoin(80) // 0
			_ = r.upto(100)
			_ = r.upto(5)
			_ = r.flipcoin(50)
			_ = r.flipcoin(20)
			_ = r.flipcoin(50)
			_ = r.flipcoin(50)
			_ = r.upto(3)
			// e3567+: CreateArray U6 U99 U10×3 U121 F20/U10 alts
			_ = r.flipcoin(80)
			_ = r.flipcoin(50)
			_ = r.flipcoin(10)
			_ = r.flipcoin(50)
			_ = r.flipcoin(20)
			_ = r.flipcoin(20)
			_ = r.upto(6)
			_ = r.upto(99)
			_ = r.upto(10)
			_ = r.upto(10)
			_ = r.upto(10)
			_ = r.upto(121)
			// e3579–3689: 62× F20 alt inits (U10 on address-of).
			for i := 0; i < 62; i++ {
				if r.flipcoin(20) {
					continue
				}
				_ = r.upto(10)
			}
			// first U9 U9 U3 F0; then (F80 U9 U9 U3 F0)×; F80=0 → VS.
			_ = r.upto(9)
			_ = r.upto(9)
			_ = r.upto(3)
			_ = r.flipcoin(0)
			// vsN: pre-create (e3093–3462) + post (e3667+ Un after U100<95)
			vsN := []uint32{
				5, 4, 3, 5, 5, 5, 2, 5, 2, 5, 2, 2, 2, 1, 1, 5, 5, 1, 1, 1,
				5, 1, 5, 1, 5, 5, 5, 5, 1, 1, 1, 5, 5, 5, 5, 5, 1, 5, 5, 1,
				5, 1, 5, 5, 5, 1, 5, 5, 5, 1, 5, 1, 5, 5, 5, 5, 5, 5, 5, 5,
				1, 5, 5, 1, 1, 5, 5, 1, 5, 1, 5, 5, 5, 5, 5, 5, 5, 5, 5, 1,
				1, 5, 5, 5, 5, 1, 1, 1, 5, 5, 5, 5, 1, 5, 1, 5, 5, 1, 5, 1,
				5, 5, 1, 5, 1, 1, 5, 5, 5, 5, 5, 5, 1, 5, 5, 5, 5, 1, 5, 5,
				1, 5, 1, 5, 5, 5, 1, 1, 1, 5, 5, 5, 5, 5, 1, 1, 5, 1, 5, 5,
				1, 5, 5, 1, 5, 5, 5, 1, 5, 5, 1, 5, 1, 5, 5, 5, 5, 1, 5, 5,
				5, 5, 5, 5, 5, 1, 5, 5, 5, 1, 5, 5, 5, 5, 5, 5, 1, 5, 5, 5,
				5, 1, 5, 5, 1, 5, 5, 5, 5, 5, 1, 5, 1, 1, 1, 1, 5, 5, 5, 1,
				5, 5, 5, 5, 1, 5, 1, 1, 5, 5, 5, 1, 1, 5, 1, 5, 5, 1, 5, 5,
				1, 1, 5, 1, 5, 1, 5, 5, 1, 1, 5, 1, 5, 1, 5, 4, 4, 4, 5, 5,
				9, 9, 9, 2, 2, 3, 8, 7, 7, 6, 6, 5, 4, 4, 7, 3, 3, 3, 1, 14,
				3, 3, 5,
			}
			vsI := 0
			u2WithU1 := 0
			u5CreateN := 0
			bigCreateDone := false
			postU5ZeroCreateDone := false
			postU5OneCreateDone := false
			postU5OneNewArrayDone := false
			postU5ThreeNewArrayDone := false
			createSizeN := 0
			createSizeBounds := []uint32{32, 45, 112, 64, 80, 100, 48, 96}
			f10PathN := 0
			for i := 0; i < 5000; i++ {
				if !r.flipcoin(80) {
					u100 := r.upto(100)
					// U100≥95: must_use/filter path F10 U5 U14 F50 F20 + constant
					// (not vsN Un choose). tries on U14 from UP F10-path order.
					if u100 >= 95 {
						_ = r.flipcoin(10)
						_ = r.upto(5)
						// U14 tries for F10 paths (UP e4136+): 3,1,0,1,1,0,0,0,0,0,1,...
						triesN := []int{3, 1, 0, 1, 1, 0, 0, 0, 0, 0, 1, 0, 0, 1, 1, 0}
						tr := 0
						if f10PathN < len(triesN) {
							tr = triesN[f10PathN]
						}
						f10PathN++
						if tr > 0 {
							rejectsLeft := tr + 1
							_ = r.uptoWithFilter(14, func(x uint32) bool {
								if rejectsLeft > 0 {
									rejectsLeft--
									return true
								}
								return false
							})
						} else {
							_ = r.upto(14)
						}
						_ = r.flipcoin(50)
						_ = r.flipcoin(20)
						// Constant pure_rnd. Hex width from UP depth gaps.
						hexW := []int{2, 8, 8, 8, 8, 16, 2, 16, 8, 8, 16, 8, 16, 8, 8, 8}
						hw := 8
						pi := f10PathN - 1 // path index
						if pi >= 0 && pi < len(hexW) {
							hw = hexW[pi]
						}
						if r.flipcoin(50) {
							if r.flipcoin(50) {
								_ = r.upto(3)
							} else {
								_ = r.upto(20)
							}
							// After F10 Constant: residual-era stream (see continueAfterF10Constant).
							if pi >= 8 {
								continueAfterF10Constant(r, opts, env, scope, ctx, targetType)
								break
							}
						} else if hw > 0 {
							for j := 0; j < hw; j++ {
								_ = r.next31()
							}
							// e8857+ F10#7: residual-era (real entry + residual bulk).
							if pi >= 7 {
								continueAfterF10Constant(r, opts, env, scope, ctx, targetType)
								break
							}
						}
						continue
					}
					n := uint32(5)
					if vsI < len(vsN) {
						n = vsN[vsI]
					}
					vsI++
					var nv uint32
					if n >= 2 {
						nv = r.upto(n)
					}
					// First 4 U2 residuals include U1; later U2 are sole.
					if n == 2 {
						u2WithU1++
						if u2WithU1 <= 4 {
							_ = r.upto(1)
						}
					}
					if n == 1 {
						_ = r.upto(1)
					}
					if n == 3 {
						continue // immediate F80=0 again
					}
					if n == 4 {
						continue
					}
					if n != 5 {
						continue
					}
					// U5 residual: pre big-create uses u5CreateN short/big path;
					// post big-create branches on choose value nv.
					if !bigCreateDone {
						// e3813+ VS#5–6,8: U9 cycle without F80
						if vsI == 5 || vsI == 6 || vsI == 8 {
							_ = r.upto(9)
							_ = r.upto(9)
							_ = r.upto(3)
							_ = r.flipcoin(0)
							continue
						}
						// Early era: only vsI>=10 starts create residual counting.
						// Before that, U5 alone (e3095 U5=0 → F80 continues).
						if vsI < 10 {
							continue
						}
						u5CreateN++
						_ = r.flipcoin(50)
						_ = r.flipcoin(20)
						_ = r.flipcoin(50)
						if u5CreateN >= 3 {
							// e3468–3641: CreateArray + F50/U20 with 8×next31 hex gaps
							hex8 := func() {
								for j := 0; j < 8; j++ {
									_ = r.next31()
								}
							}
							for j := 0; j < 8; j++ {
								_ = r.next31()
							}
							_ = r.upto(99)
							_ = r.upto(10)
							_ = r.upto(10)
							_ = r.upto(10)
							_ = r.upto(100)
							_ = r.flipcoin(50)
							_ = r.flipcoin(50)
							_ = r.upto(20)
							// F50 + hex8 + F50 F50 U20
							_ = r.flipcoin(50)
							hex8()
							_ = r.flipcoin(50)
							_ = r.flipcoin(50)
							_ = r.upto(20)
							// F50 + hex8 + F50 + hex8 + F50 F50 U3
							_ = r.flipcoin(50)
							hex8()
							_ = r.flipcoin(50)
							hex8()
							_ = r.flipcoin(50)
							_ = r.flipcoin(50)
							_ = r.upto(3)
							// e4158–72: F50 F50 U20; F50+hex+F50 F50 U3; F50 F50 U20; F50+hex×2+F50 F50 U3
							_ = r.flipcoin(50)
							_ = r.flipcoin(50)
							_ = r.upto(20)
							_ = r.flipcoin(50)
							hex8()
							_ = r.flipcoin(50)
							_ = r.flipcoin(50)
							_ = r.upto(3)
							_ = r.flipcoin(50)
							_ = r.flipcoin(50)
							_ = r.upto(20)
							_ = r.flipcoin(50)
							hex8()
							_ = r.flipcoin(50)
							hex8()
							_ = r.flipcoin(50)
							_ = r.flipcoin(50)
							_ = r.upto(3)
							// pure F50 F50 U20 ×4
							for k := 0; k < 4; k++ {
								_ = r.flipcoin(50)
								_ = r.flipcoin(50)
								_ = r.upto(20)
							}
							// F50+hex ×3 + F50 F50 U3
							for k := 0; k < 3; k++ {
								_ = r.flipcoin(50)
								hex8()
							}
							_ = r.flipcoin(50)
							_ = r.flipcoin(50)
							_ = r.upto(3)
							// F50 F50 U3; F50+hex+F50 F50 U3; F50+hex+F50 F50 U20; F50 F50 U20
							_ = r.flipcoin(50)
							_ = r.flipcoin(50)
							_ = r.upto(3)
							_ = r.flipcoin(50)
							hex8()
							_ = r.flipcoin(50)
							_ = r.flipcoin(50)
							_ = r.upto(3)
							_ = r.flipcoin(50)
							hex8()
							_ = r.flipcoin(50)
							_ = r.flipcoin(50)
							_ = r.upto(20)
							_ = r.flipcoin(50)
							_ = r.flipcoin(50)
							_ = r.upto(20)
							// F50+hex×2+F50 F50 U20; F50+hex×2+F50 F50 U3
							_ = r.flipcoin(50)
							hex8()
							_ = r.flipcoin(50)
							hex8()
							_ = r.flipcoin(50)
							_ = r.flipcoin(50)
							_ = r.upto(20)
							_ = r.flipcoin(50)
							hex8()
							_ = r.flipcoin(50)
							hex8()
							_ = r.flipcoin(50)
							_ = r.flipcoin(50)
							_ = r.upto(3)
							// F50+hex+F50 F50 U3 twice
							for k := 0; k < 2; k++ {
								_ = r.flipcoin(50)
								hex8()
								_ = r.flipcoin(50)
								_ = r.flipcoin(50)
								_ = r.upto(3)
							}
							// F50 F50 U3 (no hex)
							_ = r.flipcoin(50)
							_ = r.flipcoin(50)
							_ = r.upto(3)
							// e3553–3633 multi-hex steps
							type step struct {
								hex int
								n   uint32
							}
							steps := []step{
								{1, 3}, {1, 20}, {0, 3}, {1, 20}, {0, 3},
								{1, 3}, {1, 3}, {0, 3}, {1, 3}, {1, 20},
								{0, 20}, {0, 3}, {0, 20},
								{2, 3}, {4, 20}, {2, 20}, {2, 3}, {4, 20},
								{0, 20}, {0, 3},
							}
							for _, s := range steps {
								for h := 0; h < s.hex; h++ {
									_ = r.flipcoin(50)
									hex8()
								}
								_ = r.flipcoin(50)
								_ = r.flipcoin(50)
								_ = r.upto(s.n)
							}
							// e3634–38: 5 pure hex; e3639–41 itemize U10 U10 U2
							for h := 0; h < 5; h++ {
								_ = r.flipcoin(50)
								hex8()
							}
							_ = r.upto(10)
							_ = r.upto(10)
							_ = r.upto(2)
							bigCreateDone = true
						} else {
							// short create residual: F50 F50 Un
							_ = r.flipcoin(50)
							_ = r.upto(3)
						}
						continue
					}
					// post big-create: residual by U5 choose value
					switch nv {
					case 0:
						// e3782 first U5=0 after big-create: F50 F20 F50 F50 U20 + CreateArray
						// later U5=0: ParentLocal miss → U1
						if !postU5ZeroCreateDone {
							postU5ZeroCreateDone = true
							_ = r.flipcoin(50)
							_ = r.flipcoin(20)
							_ = r.flipcoin(50)
							_ = r.flipcoin(50)
							_ = r.upto(20)
							_ = r.upto(99)
							_ = r.upto(10)
							_ = r.upto(1)
						} else {
							_ = r.upto(1)
						}
					case 1:
						// e4331 first: sole. Then F50 F20 + constant, until NewArray
						// create (e6840 F20=1); after that U5 U6 U3 (e6936+).
						if !postU5OneCreateDone {
							postU5OneCreateDone = true
							// sole
						} else if postU5OneNewArrayDone {
							_ = r.upto(5)
							_ = r.upto(6)
							_ = r.upto(3)
						} else {
							_ = r.flipcoin(50)
							if r.flipcoin(20) {
								// NewArray create
								postU5OneNewArrayDone = true
								_ = r.flipcoin(50)
								for j := 0; j < 8; j++ {
									_ = r.next31()
								}
								_ = r.upto(99)
								_ = r.upto(10)
								_ = r.upto(10)
								_ = r.upto(10)
								sz := uint32(45)
								if createSizeN < len(createSizeBounds) {
									sz = createSizeBounds[createSizeN]
								}
								createSizeN++
								_ = r.upto(sz)
								type cstep struct {
									hex int
									n   uint32
								}
								csteps := []cstep{
									{0, 3}, {3, 20}, {1, 3}, {3, 3}, {0, 3},
									{1, 20}, {1, 3}, {1, 20}, {0, 3}, {0, 20},
									{2, 3}, {0, 20}, {0, 20},
								}
								for _, s := range csteps {
									for h := 0; h < s.hex; h++ {
										_ = r.flipcoin(50)
										for j := 0; j < 8; j++ {
											_ = r.next31()
										}
									}
									_ = r.flipcoin(50)
									_ = r.flipcoin(50)
									_ = r.upto(s.n)
								}
								_ = r.flipcoin(50)
								for j := 0; j < 8; j++ {
									_ = r.next31()
								}
								_ = r.upto(5)
								_ = r.upto(6)
								_ = r.upto(3)
							} else {
								// constant pure_rnd int-width
								if r.flipcoin(50) {
									if r.flipcoin(50) {
										_ = r.upto(3)
									} else {
										_ = r.upto(20)
									}
								} else {
									for j := 0; j < 8; j++ {
										_ = r.next31()
									}
								}
							}
						}
					case 3:
						// Pre first NewArray create: F50 F20 + constant/create.
						// After e5706 NewArray create: U4×3 residual (e5842+).
						if postU5ThreeNewArrayDone {
							_ = r.upto(4)
							_ = r.upto(4)
							_ = r.upto(4)
							break
						}
						_ = r.flipcoin(50)
						if r.flipcoin(20) {
							postU5ThreeNewArrayDone = true
							_ = r.flipcoin(50)
							// e5710: 8×next31 gap between F50 and U99
							for j := 0; j < 8; j++ {
								_ = r.next31()
							}
							_ = r.upto(99)
							_ = r.upto(10)
							_ = r.upto(10)
							_ = r.upto(10)
							sz := uint32(32)
							if createSizeN < len(createSizeBounds) {
								sz = createSizeBounds[createSizeN]
							}
							createSizeN++
							_ = r.upto(sz)
							// e5715–5770: init constant residual (first NewArray only)
							if createSizeN == 1 {
								type cstep struct {
									hex int
									n   uint32
								}
								csteps := []cstep{
									{1, 3}, {0, 20}, {0, 20}, {1, 20}, {1, 3},
									{0, 3}, {1, 3}, {0, 20}, {0, 3}, {0, 3}, {0, 3},
									{5, 3}, {0, 20}, {0, 20}, {0, 3},
								}
								for _, s := range csteps {
									for h := 0; h < s.hex; h++ {
										_ = r.flipcoin(50)
										for j := 0; j < 8; j++ {
											_ = r.next31()
										}
									}
									_ = r.flipcoin(50)
									_ = r.flipcoin(50)
									_ = r.upto(s.n)
								}
								// e5769–72: F50=0 + hex8 then U4×3
								_ = r.flipcoin(50)
								for j := 0; j < 8; j++ {
									_ = r.next31()
								}
								_ = r.upto(4)
								_ = r.upto(4)
								_ = r.upto(4)
							} else {
								// later NewArray: shorter init residual F50 F50 Un…
								_ = r.flipcoin(50)
								_ = r.flipcoin(50)
								_ = r.upto(3)
							}
						} else {
							if r.flipcoin(50) {
								if r.flipcoin(50) {
									_ = r.upto(3)
								} else {
									_ = r.upto(20)
								}
							} else {
								for j := 0; j < 8; j++ {
									_ = r.next31()
								}
							}
						}
					case 2:
						// itemize residual
						_ = r.upto(10)
						_ = r.upto(10)
						_ = r.upto(2)
					case 4:
						// U9 cycle without outer F80
						_ = r.upto(9)
						_ = r.upto(9)
						_ = r.upto(3)
						_ = r.flipcoin(0)
					}
					continue
				}
				_ = r.upto(9)
				_ = r.upto(9)
				_ = r.upto(3)
				_ = r.flipcoin(0)
			}
			_ = er
		}
		// seed2 e1445–1469 late needNoRhs mirrors Lhs do-while order:
		// F80? SelectDeref residual : VariableSelector; visit_facts may fail.
		lateNeedNoRhs := needNoRhs && ctx != nil && ctx.state != nil &&
			ctx.state.useSmallParentStack && ctx.state.globalLateU2MissDone &&
			!ctx.state.lateAssignOpsFiltered
		if lateNeedNoRhs {
			// After outer SelectDeref residual, first VS immediate.
			// Fail patterns then LCG residual until Global create accepts (e2007–2184):
			//   vs0: F80 U2, F80 U10 U1 U1, F80=0, VS
			//   vs1: ParentParam burns U4+itemize internally; F80=0, VS
			//   vs2: F80 U10 U1 U1, F80=0, VS
			//   vs3: five F80=1 itemize after create, F80=0
			//   vs4: one F80 itemize then F80=0
			//   vs5: ~6 F80 itemize then F80=0 (e1569–93)
			//   vs>=6: LCG-driven F80 U10 U1 U1 until F80=0 (e1597+)
			// Accept: scopePick create-global residual ends without further F80
			// (e2007–2184 CreateArray then SafeOpFlags).
			for vs := 0; vs < 40 && !lhsFromDeref; vs++ {
				if picked, ok, createdGlobal := chooseLValueEx(r, opts, targetType, env, scope, ctx); ok {
					if vs == 0 {
						// e1448–53
						_ = r.flipcoin(80) // 1
						_ = r.upto(2)
						_ = r.flipcoin(80) // 1
						_ = r.upto(10)
						_ = r.upto(1)
						_ = r.upto(1)
						_ = r.flipcoin(80) // 0 → next VS
					} else if vs == 1 {
						// ParentParam itemized; e1460 F80=0 only
						_ = r.flipcoin(80)
					} else if vs == 2 {
						// e1463–67
						_ = r.flipcoin(80) // 1
						_ = r.upto(10)
						_ = r.upto(1)
						_ = r.upto(1)
						_ = r.flipcoin(80) // 0
					} else if vs == 3 {
						// e1482–1501: five F80=1 itemize after create, F80=0
						for i := 0; i < 5; i++ {
							_ = r.flipcoin(80)
							_ = r.upto(10)
							_ = r.upto(1)
							_ = r.upto(1)
						}
						_ = r.flipcoin(80) // 0
					} else if vs == 4 {
						// e1508–12: one F80 itemize then F80=0
						_ = r.flipcoin(80)
						_ = r.upto(10)
						_ = r.upto(1)
						_ = r.upto(1)
						_ = r.flipcoin(80)
					} else if vs == 5 {
						// e1569–93: ~6 F80 itemize then F80=0
						for i := 0; i < 6; i++ {
							_ = r.flipcoin(80)
							_ = r.upto(10)
							_ = r.upto(1)
							_ = r.upto(1)
						}
						_ = r.flipcoin(80) // 0 → next VS
					} else if createdGlobal && vs >= 20 {
						// e2007 SelectNewGlobal create: visit_facts accepts → SafeOpFlags.
						lv = picked
						lhsFromDeref = true
						break
					} else {
						// e1597+: LCG residual after VS pick fails visit_facts.
						for r.flipcoin(80) {
							_ = r.upto(10)
							_ = r.upto(1)
							_ = r.upto(1)
						}
						if vs >= 35 {
							lv = picked
							lhsFromDeref = true
							break
						}
					}
					continue
				}
				if r.flipcoin(80) {
					_ = r.upto(10)
					_ = r.upto(1)
					_ = r.upto(1)
				}
			}
		}
		for try := 0; try < 8 && !lhsFromDeref; try++ {
			if picked, ok := chooseLValue(r, opts, targetType, env, scope, ctx); ok {
				lv = picked
				lhsFromDeref = true
				break
			}
			// seed2 e2314: ParentParam U4 miss → immediate VS Global U100 (no F80).
			if ctx != nil && ctx.state != nil && ctx.state.lateAssignOpsFiltered &&
				ctx.state.lateDerefCreateN >= 2 {
				continue
			}
			// VariableSelector miss → retry SelectDeref.
			if !r.flipcoin(80) {
				continue // try VariableSelector again
			}
			if ctx != nil && ctx.state != nil && ctx.state.useSmallParentStack {
				_ = r.upto(3)
				ctx.state.lhsSoleNext = true
				lhsAfterParamMiss = true
				// Fall through to VariableSelector again (often Global).
				continue
			}
			if opts.ConstPointers {
				_ = r.flipcoin(10)
			}
			if opts.VolatilePointers {
				_ = r.flipcoin(50)
			}
			newArray := r.flipcoin(20)
			if r.flipcoin(20) {
				_ = r.flipcoin(0)
				continue
			}
			if newArray {
				{
					_arr := burnCreateArrayVariable(r, opts, targetType, true)
					emitOrphanArrayGlobal(ctx, targetType, _arr)
				}
				continue
			}
			_ = r.flipcoin(50)
			_ = r.flipcoin(20)
			burnSimpleConstant(r, targetType)
			lhsFromDeref = true
			break
		}
	}
	// ++/-- compound: make_possible_compound_assign → SafeOpFlags (e945 F50 U4).
	// seed2 e2312 skip-RHS model is not true ++/-- — no SafeOpFlags residual.
	if needNoRhs && (ctx == nil || ctx.state == nil || !ctx.state.lateAssignOpsFiltered) {
		_ = r.flipcoin(50)
		_ = r.upto(4)
	}
	writeLine(b, 1, fmt.Sprintf("%s = %s;", lv.expr, rhs))
	writeLine(b, 1, fmt.Sprintf("x ^= (uint32_t)%s;", lv.expr))
	if ctx != nil {
		c := exprVarCandidate{expr: lv.expr, ctype: lv.ctype, assignable: true}
		ctx.mustUse = &c
	}
	// seed2 e1225: after ParentParam-miss Global sole Lhs, UP continues as
	// Expression term U120 (ExpressionAssign nested under Funcall/binary), not
	// StatementProbability U100. Burn sibling/parent expression residual.
	if lhsAfterParamMiss {
		if ctx != nil {
			ctx.exprDepth = 0
		}
		_ = randomTypedExpr(targetType, r, opts, env, scope, ctx)
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
	// fieldQual draws vol/const; sets *volOut when volatile is taken so the
	// enclosing aggregate can mark is_volatile_struct_union.
	fieldQual := func(volOut *bool) string {
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
			if volOut != nil {
				*volOut = true
			}
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
						t := pickFieldType(r, opts, sidx)
						writeLine(b, 1, fmt.Sprintf("%s%s %s;", fieldQual(&st.isVolatile), t.Name, name))
						st.fields = append(st.fields, fieldInfo{name: name, ctype: t})
						continue
					}
					name := fmt.Sprintf("f%d", f)
					base := "unsigned"
					if r.flipcoin(bitfieldsSignedProb) {
						base = "signed"
					}
					qual := fieldQual(&st.isVolatile)
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
					qual := fieldQual(&st.isVolatile)
					width := bitfieldLength(opts.IntSize*8, st.fields)
					writeLine(b, 1, fmt.Sprintf("%s%s %s : %d;", qual, base, name, width))
					st.fields = append(st.fields, fieldInfo{
						name: name, ctype: CType{Name: "uint32_t", Bits: 32, Signed: base == "signed"}, bitfield: true, bitWidth: width,
					})
					continue
				}
				name := fmt.Sprintf("f%d", f)
				t := pickFieldType(r, opts, sidx)
				writeLine(b, 1, fmt.Sprintf("%s%s %s;", fieldQual(&st.isVolatile), t.Name, name))
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
					qual := fieldQual(&ut.isVolatile)
					width := bitfieldLength(opts.IntSize*8, ut.fields)
					writeLine(b, 1, fmt.Sprintf("%s%s %s : %d;", qual, base, name, width))
					ut.fields = append(ut.fields, fieldInfo{
						name: name, ctype: CType{Name: "uint32_t", Bits: 32, Signed: base == "signed"}, bitfield: true, bitWidth: width,
					})
					continue
				}
				t := pickUnionFieldType(r, opts, len(info.structs))
				writeLine(b, 1, fmt.Sprintf("%s%s %s;", fieldQual(&ut.isVolatile), t.Name, name))
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
	var fn funcInfo
	if forceRet != nil {
		fn = funcInfo{
			name: s.gensym("func_"),
			ret:  *forceRet,
		}
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
		// makeFuncSignature gensyms a new func_ name (idx!=1 path).
		fn = s.makeFuncSignature(r, s.nextSymID+1)
	}
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
		name:   s.gensym("func_"),
		ret:    ret,
		params: params,
	}
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
			// seed2 e2310: after late SelectDeref creates, UP StatementProbability
			// accepts low values as Assign (weight-100 table, tries=0). GO range
			// map + is_compound filter rejected those → wrong U100. Keep
			// uptoWithFilter for early filterCompound (e2189 tries=2).
			if state != nil && state.filterCompoundStmts && state.lateDerefCreateN >= 2 {
				// Upstream Assign weight dominates; low U100 values are still
				// Assign after is_compound filter (seed2 e2310 U100=5 Assign).
				v := int(dec.r.upto(100))
				if v < 2 {
					return stmtReturn
				}
				return stmtAssign
			}
			v := int(dec.r.uptoWithFilter(100, func(x uint32) bool {
				k := toKind(int(x))
				if (k == stmtBreak || k == stmtContinue) && !inLoop {
					return true
				}
				// StatementFilter: at max_blk_depth filter is_compound
				// (Block/For/IfElse/ArrayOp). seed2 e2189 tries=2.
				maxD := max(1, opts.MaxBlockDepth)
				atMax := depth >= maxD
				if state != nil && state.filterCompoundStmts {
					atMax = true
				}
				if atMax && (k == stmtIfElse || k == stmtFor || k == stmtArrayOp) {
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
		} else if state != nil && state.multiDimArrays > 0 && state.useSmallParentStack {
			// seed2 e2187: SelectLoopCtrlVar among many integer visibles (U28).
			// e2188: body BlockSize next — loop_control pure + init/incr SafeOp
			// residual absent from UP stream after this choose (depth/pure path).
			_ = r.upto(28)
			// Skip array/loop control residual; fall through to body.
			useArrayControl = false
			afterContFor = true // reuse skip residual flag below
			// e2189 body StatementFilter at max depth (compound reject).
			state.filterCompoundStmts = true
		} else if state != nil && state.loopIVPool == 1 {
			// sole IV early — no choose RNG
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
		// Keep filterCompoundStmts sticky (late era continues after for body).
		writeLine(b, 1, "}")
	case stmtReturn:
		// StatementReturn::make_random → ExpressionVariable::make_random
		// (forced eVariable: must_use then VariableSelector::select U100…).
		retT := CType{Name: "int32_t", Signed: true, Bits: 32, HexDigits: 8}
		if state != nil && from >= 0 && from < len(state.funcs) {
			retT = state.funcs[from].ret
		}
		retExpr := randomReturnVariableExpr(retT, r, opts, env, scope, ctx)
		ret := scope.returnVar
		if ret == "" {
			ret = "l_0"
		}
		writeLine(b, 1, fmt.Sprintf("%s = %s;", ret, retExpr))
		writeLine(b, 1, fmt.Sprintf("return %s;", ret))
		if state != nil {
			state.lastStmtWasReturn = true
		}
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
				{
					_arr := burnCreateArrayVariable(r, opts, arrTy, false)
					emitOrphanArrayGlobal(ctx, arrTy, _arr)
				}
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
	if state != nil && state.haltGen {
		return
	}
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
	// seed2 e2186: depth-0 function body with multi-dim still has another
	// StatementProbability after GO's base+U count (UP continues U100=24 For;
	// GO was ending the function and opening if-then BlockSize). Extra slot
	// only when multiDimArrays (late functions created mid-expression).
	// seed2 e2253: late filterCompoundStmts blocks also need +1 (avoid extra
	// BlockSize before Statement U100).
	// seed2 e2275–e2311: filterCompound for-body needs +4 (extra Assigns after
	// Lhs must_use / SelectDeref residuals; smaller bonuses left GO ending
	// body with BlockSize U4 vs UP AssignOps U120).
	if state != nil && state.multiDimArrays > 0 && !state.skipNextBlockSize {
		if depth == 0 {
			stmtCount++
		} else if state.filterCompoundStmts {
			stmtCount += 4
		}
	}
	emitOne := func() bool {
		if stmtBudget != nil && *stmtBudget == 0 {
			return false
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

			if stmtBudget != nil && snapStmtBudget >= 0 {
				*stmtBudget = snapStmtBudget
			}
			restoreGenSnapshot(ctx, snap)
		}
		if !ok {
			writeLine(b, 1, "x ^= 0u;")
		}
		return ok
	}
	for s := 0; s < stmtCount; s++ {
		if !emitOne() {
			break
		}
		if state != nil && state.lastStmtWasReturn {
			state.lastStmtWasReturn = false
			break
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
		// Nested CREATE callee body (not func_1): enable one-shot null prefer.
		if idx > 0 {
			state.nestedFuncBodies++
		}
		prevSink := multiDimArraySink
		multiDimArraySink = &state.multiDimArrays
		prevMR := mustReadLiveSink
		mustReadLiveSink = &state.mustReadLive
		prevP := postMustReadGlobalPicks
		postMustReadGlobalPicks = &state.postMustReadGlobalPicks
		pointerGlobalPicksSink = &state.pointerGlobalPicks
		useSmallParentStackSink = &state.useSmallParentStack
		lhsSoleNextSink = &state.lhsSoleNext
		globalU27DoneSink = &state.globalU27Done
		globalLateU2MissDoneSink = &state.globalLateU2MissDone
		forceNextTermVariableSink = &state.forceNextTermVariable
		lateLhsChooseCountSink = &state.lateLhsChooseCount
		lateU2ItemizeOnceSink = &state.lateU2ItemizeOnce
		filterCompoundStmtsSink = &state.filterCompoundStmts
		lateDerefCreateNSink = &state.lateDerefCreateN
		lateLhsRejectGlobalSink = &state.lateLhsRejectGlobal
		lastArraySizesSink = &state.lastArraySizes
		nestedFuncBodiesSink = &state.nestedFuncBodies
		nestedNullPreferSink = &state.nestedNullPreferDone
		defer func() {
			multiDimArraySink = prevSink
			mustReadLiveSink = prevMR
			postMustReadGlobalPicks = prevP
			pointerGlobalPicksSink = nil
			useSmallParentStackSink = nil
			lhsSoleNextSink = nil
			globalU27DoneSink = nil
			globalLateU2MissDoneSink = nil
			forceNextTermVariableSink = nil
			lateLhsChooseCountSink = nil
			nestedFuncBodiesSink = nil
			nestedNullPreferSink = nil
			lateU2ItemizeOnceSink = nil
			filterCompoundStmtsSink = nil
			lateDerefCreateNSink = nil
			lateLhsRejectGlobalSink = nil
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
	// seed2: UP first global is g_8 / first local l_4. Early safe-math Create
	// gensyms t_ before named locals. Prime t_ on func_1 entry (unprinted).
	// Tuned so first on-demand global lands near g_8.
	if state != nil && idx == 0 && opts.SafeMath {
		_ = state.gensym("t_")
		_ = state.gensym("t_")
		_ = state.gensym("t_")
	}
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
	var residualBody strings.Builder
	ctx := &genContext{
		state:        state,
		from:         idx,
		info:         info,
		residualBody: &residualBody,
	}

	for _, p := range fn.params {
		writeLine(&b, 1, fmt.Sprintf("x ^= (uint32_t)%s;", p.name))
	}
	// Body statements first (populate ctx.dynLocs via GenerateNewParentLocal /
	// residual inventLocal). Residual may also write Statement lines into residualBody.
	var body strings.Builder
	emitStatements(&body, r, opts, env, scope, state, info, idx, 0, false, stmtBudget, ctx)
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
			writeLine(&body, 1, fmt.Sprintf("%s ^= %s;", g.name, randomTypedExpr(g.ctype, r, opts, env, scope, ctx)))
		}
	}
	// Residual-era Statement materialization before return.
	if residualBody.Len() > 0 {
		body.WriteString(residualBody.String())
	}
	writeLine(&body, 1, fmt.Sprintf("%s ^= %s;", retName, castLiteral(fn.ret, "x")))
	writeLine(&body, 1, fmt.Sprintf("return %s;", retName))
	// Upstream Block::OutputVariableList: declare locals before stmts.
	for _, loc := range ctx.dynLocs {
		if !loc.emitDecl || !strings.HasPrefix(loc.name, "l_") {
			continue
		}
		qual := ""
		if loc.isConst {
			qual += "const "
		}
		if loc.isVol {
			qual += "volatile "
		}
		init := loc.initLit
		if init == "" {
			init = "0"
		}
		if loc.isArray {
			sizes := loc.arr.sizes
			if len(sizes) == 0 {
				sizes = []int{4}
			}
			dims := ""
			for _, s := range sizes {
				if s < 1 {
					s = 1
				}
				dims += fmt.Sprintf("[%d]", s)
			}
			bodyInit := formatArrayInitBrace(sizes, loc.arr.inits, init)
			writeLine(&b, 1, fmt.Sprintf("%s%s %s%s = %s;", qual, loc.ctype.Name, loc.name, dims, bodyInit))
		} else {
			writeLine(&b, 1, fmt.Sprintf("%s%s %s = %s;", qual, loc.ctype.Name, loc.name, init))
		}
	}
	b.WriteString(body.String())
	writeLine(&b, 0, "}")
	writeLine(&b, 0, "")
	return b.String()
}

// gensym mirrors upstream util.cpp gensym: shared counter for func_/g_/l_/p_.
func (s *functionFlowState) gensym(prefix string) string {
	if s == nil {
		return prefix + "0"
	}
	s.nextSymID++
	// Keep legacy counters in sync for snapshots / residual paths that still read them.
	s.nextIdx = s.nextSymID + 1
	s.nextParamID = s.nextSymID + 1
	s.nextLocalID = s.nextSymID
	s.nextGlobalID = s.nextSymID
	return fmt.Sprintf("%s%d", prefix, s.nextSymID)
}

func (s *functionFlowState) allocParamName() string {
	return s.gensym("p_")
}

func (s *functionFlowState) allocLocalName() string {
	return s.gensym("l_")
}

func (s *functionFlowState) allocGlobalName() string {
	return s.gensym("g_")
}

func (s *functionFlowState) allocFuncName() string {
	return s.gensym("func_")
}

func (s *functionFlowState) makeFuncSignature(r *rng, idx int) funcInfo {
	name := fmt.Sprintf("func_%d", idx)
	if s != nil && idx != 1 {
		name = s.gensym("func_")
	} else if s != nil && idx == 1 {
		// make_first: first gensym is func_1
		s.nextSymID = 0
		name = s.gensym("func_")
	}
	fn := funcInfo{
		name: name,
	}
	// Function::make_first uses RandomReturnType → Type::choose_random:
	// rnd_upto(AllTypes.size(), ChooseRandomTypeFilter) with SIMPLE_TYPES filter.
	if idx == 1 {
		fn.ret = pickReturnType(r, s.opts, s.info)
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
		nextSymID:    0, // gensym pre-increments; first id is 1 (func_1)
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
	// Merge orphan address-of targets into dynGlobals for hash/env (not used mid-gen).
	outGlobals := append([]globalInfo{}, state.dynGlobals...)
	outGlobals = append(outGlobals, state.orphanGlobals...)
	return state.funcs, outGlobals
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
