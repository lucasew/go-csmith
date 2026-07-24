// Upstream: Probabilities.h / Probabilities.cpp
// (initialize_single_probs, set_default_simple_types_prob, ProbabilityFilter for equal groups).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// ProbName identifies a probability knob (subset used by the fair spine so far).
// Full enum in Probabilities.h — extend when a ported path needs a name.
// Order of *single* names is independent of C++ enum values; group roots used
// by ProbabilityFilter must match the names set_prob_filter registers.
type ProbName int

const (
	// Single probs — Probabilities::initialize_single_probs
	PMoreStructUnionProb ProbName = iota
	PBitFieldsCreationProb
	PBitFieldInNormalStructProb
	PScalarFieldInFullBitFieldsProb
	PExhaustiveBitFieldsProb
	PBitFieldsSignedProb
	PSafeOpsSignedProb
	PFuncAttrProb
	PTypeAttrProb
	PLabelAttrProb
	PVarAttrProb
	PBinaryConstProb
	PRegularVolatileProb
	PRegularConstProb
	PStricterConstProb
	PLooserConstProb
	PFieldVolatileProb
	PFieldConstProb
	PStdUnaryFuncProb
	PShiftByNonConstantProb
	PStructAsLTypeProb
	PUnionAsLTypeProb
	PFloatAsLTypeProb
	PNewArrayVariableProb
	PPointerAsLTypeProb
	PSelectDerefPointerProb
	PAccessOnceVariableProb
	PInlineFunctionProb
	PBuiltinFunctionProb
	PArrayOOBProb

	// Equal groups (set_default_* + set_prob_filter)
	PStatementProb
	PAssignOpsProb
	PUnaryOpsProb
	PBinaryOpsProb
	PSimpleTypesProb
	PSafeOpsSizeProb
)

// Probabilities holds default probability tables for the current Options.
// Mirrors Probabilities singleton after initialize() for the fields we port.
type Probabilities struct {
	single map[ProbName]int
	// simpleTypeWeight[i] is weight for eSimpleType(i) in equal group pSimpleTypesProb.
	// Probabilities::set_default_simple_types_prob — weight 0 or 1.
	simpleTypeWeight []int
	// binaryOpWeight mirrors pBinaryOpsProb equal group (set_default_binary_ops_prob).
	binaryOpWeight []int
	// unaryOpWeight mirrors pUnaryOpsProb equal group (set_default_unary_ops_prob).
	unaryOpWeight []int
	// safeOpsSizeWeight mirrors pSafeOpsSizeProb equal group (set_default_safe_ops_size_prob).
	// Indices: SafeInt8..SafeInt64 (float excluded from draw).
	safeOpsSizeWeight []int
	// statementTable mirrors Statement::stmtTable_ after initialize(pStatementProb).
	// Probabilities.cpp set_default_statement_prob (unequal group cutoffs → eStatementType).
	statementTable *ThresholdTable

	// probFilters mirrors Probabilities::prob_filters_ (ProbabilityFilter per equal group).
	probFilters map[ProbName]*ProbabilityFilter
	// extraFilters mirrors Probabilities::extra_filters_ (register_extra_filter).
	extraFilters map[ProbName]Filter
}

// ProbabilityFilter mirrors ProbabilityFilter — equal-group weight filter.
// Probabilities.cpp:55–82 — extra filter first, then weight==0 rejects.
type ProbabilityFilter struct {
	pname ProbName
}

// NewProbabilityFilter mirrors ProbabilityFilter(ProbName).
func NewProbabilityFilter(pname ProbName) *ProbabilityFilter {
	return &ProbabilityFilter{pname: pname}
}

// Filter implements Filter — true means reject candidate v.
// Probabilities.cpp:59–81.
func (f *ProbabilityFilter) Filter(v uint32) bool {
	if f == nil {
		sessNoteError(nil, ErrGeneric)
		return true
	}
	p := ProcessProbabilities()
	if p == nil {
		// C++ GetInstance always live after init; nil is library fail-closed.
		sessNoteError(nil, ErrGeneric)
		return true
	}
	if p.CheckExtraFilter(f.pname, int(v)) {
		return true
	}
	w := p.equalGroupWeight(f.pname, int(v))
	if sessHasError(nil) {
		return true
	}
	return w == 0
}

// NewProbabilities builds tables from opts like Probabilities::initialize
// after CGOptions::set_default_settings (and option flags).
func NewProbabilities(opts Options) *Probabilities {
	p := &Probabilities{
		single:       make(map[ProbName]int),
		probFilters:  make(map[ProbName]*ProbabilityFilter),
		extraFilters: make(map[ProbName]Filter),
	}
	p.initSingle(opts)
	p.initSimpleTypes(opts)
	p.initBinaryOps(opts)
	p.initUnaryOps(opts)
	p.initSafeOpsSize(opts)
	p.initStatementProbs(opts)
	// set_prob_filter for each equal group after set_default_*
	p.setProbFilter(PSimpleTypesProb)
	p.setProbFilter(PBinaryOpsProb)
	p.setProbFilter(PUnaryOpsProb)
	p.setProbFilter(PSafeOpsSizeProb)
	return p
}

// equalGroupWeight returns weight for equal-group root pname at index v.
// ProbabilityFilter walks GroupProbElem; Go weight slices are pre-indexed.
func (p *Probabilities) equalGroupWeight(pname ProbName, v int) int {
	if p == nil {
		sessNoteError(nil, ErrGeneric)
		return 0
	}
	switch pname {
	case PSimpleTypesProb:
		return p.SimpleTypeWeight(v)
	case PBinaryOpsProb:
		return p.BinaryOpWeight(v)
	case PUnaryOpsProb:
		return p.UnaryOpWeight(v)
	case PSafeOpsSizeProb:
		return p.SafeOpsSizeWeight(v)
	default:
		// Not an equal-group filter root; C++ would assert on dynamic_cast.
		sessNoteError(nil, ErrGeneric)
		return 0
	}
}

// setProbFilter mirrors Probabilities::set_prob_filter.
// Probabilities.cpp:787–789.
func (p *Probabilities) setProbFilter(pname ProbName) {
	if p == nil {
		sessNoteError(nil, ErrGeneric)
		return
	}
	p.probFilters[pname] = NewProbabilityFilter(pname)
}

// GetProbFilter mirrors Probabilities::get_prob_filter (static via process).
// Probabilities.cpp:777–785 — prob_filters_ then extra_filters_; missing → sticky nil.
func GetProbFilter(pname ProbName) Filter {
	p := ProcessProbabilities()
	if p == nil {
		sessNoteError(nil, ErrGeneric)
		return filterFunc(func(uint32) bool { return true })
	}
	if f, ok := p.probFilters[pname]; ok && f != nil {
		return f
	}
	if f, ok := p.extraFilters[pname]; ok && f != nil {
		return f
	}
	// C++ asserts filter non-null; fail closed reject-all.
	sessNoteError(nil, ErrGeneric)
	return filterFunc(func(uint32) bool { return true })
}

// RegisterExtraFilter mirrors Probabilities::register_extra_filter.
// Probabilities.cpp:791–796.
func RegisterExtraFilter(pname ProbName, filter Filter) {
	p := ProcessProbabilities()
	if p == nil || filter == nil {
		sessNoteError(nil, ErrGeneric)
		return
	}
	p.extraFilters[pname] = filter
}

// UnregisterExtraFilter mirrors Probabilities::unregister_extra_filter.
// Probabilities.cpp:798–804 — pointer identity; Go requires comparable Filter
// values (pointer/struct receivers). Function-typed Filters are not comparable.
func UnregisterExtraFilter(pname ProbName, filter Filter) {
	p := ProcessProbabilities()
	if p == nil || filter == nil {
		sessNoteError(nil, ErrGeneric)
		return
	}
	cur, ok := p.extraFilters[pname]
	if !ok || cur == nil || !filterPtrEqual(cur, filter) {
		sessNoteError(nil, ErrGeneric)
		return
	}
	p.extraFilters[pname] = nil
}

// filterPtrEqual is C++ Filter* equality without panicking on func Filters.
func filterPtrEqual(a, b Filter) (eq bool) {
	defer func() {
		if recover() != nil {
			eq = false
		}
	}()
	return a == b
}

// CheckExtraFilter mirrors Probabilities::check_extra_filter.
// Probabilities.cpp:806–813 — true when extra filter rejects v.
func (p *Probabilities) CheckExtraFilter(pname ProbName, v int) bool {
	if p == nil {
		sessNoteError(nil, ErrGeneric)
		return true
	}
	if v < 0 {
		return false
	}
	f, ok := p.extraFilters[pname]
	if !ok || f == nil {
		return false
	}
	return f.Filter(uint32(v))
}

// ClearFilters mirrors Probabilities::clear_filter on both maps (destructor path).
func (p *Probabilities) ClearFilters() {
	if p == nil {
		sessNoteError(nil, ErrGeneric)
		return
	}
	p.probFilters = make(map[ProbName]*ProbabilityFilter)
	p.extraFilters = make(map[ProbName]Filter)
}

// StatementThresholdTable returns Statement::stmtTable_ built from pStatementProb.
// Incomplete Probabilities sticky nil (no invent empty table soft-skip past hole).
func (p *Probabilities) StatementThresholdTable() *ThresholdTable {
	// Probabilities always live in C++ singleton; sticky incomplete no invent nil table
	if p == nil {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	return p.statementTable
}

// Single returns a single probability in [0,100].
// SingleProbElem::get_prob_direct.
// Incomplete Probabilities sticky 0 (no invent default 50 / soft re-pick past hole).
func (p *Probabilities) Single(name ProbName) int {
	// Probabilities always live; sticky incomplete no invent 0% without table
	if p == nil {
		sessNoteError(nil, ErrGeneric)
		return 0
	}
	return p.single[name]
}

// SimpleTypeWeight returns equal-group weight (0 or 1) for eSimpleType index.
// Incomplete Probabilities sticky 0 (no invent weight 1 soft-skip past hole).
func (p *Probabilities) SimpleTypeWeight(simpleIdx int) int {
	// Probabilities always live; sticky incomplete no invent weight 0 soft-skip
	if p == nil {
		sessNoteError(nil, ErrGeneric)
		return 0
	}
	if simpleIdx < 0 || simpleIdx >= len(p.simpleTypeWeight) {
		return 0
	}
	return p.simpleTypeWeight[simpleIdx]
}

// SimpleTypesFilter rejects eSimpleType indices with weight 0.
// ProbabilityFilter for pSimpleTypesProb (equal group): filter(v) when weight==0.
// Uses live ProbabilityFilter when p is the process singleton; otherwise a
// bound filter against this *Probabilities (library/test paths without SetProcess).
func (p *Probabilities) SimpleTypesFilter() Filter {
	if p == nil {
		sessNoteError(nil, ErrGeneric)
		return filterFunc(func(uint32) bool { return true })
	}
	if ProcessProbabilities() == p {
		if f, ok := p.probFilters[PSimpleTypesProb]; ok && f != nil {
			return f
		}
	}
	return filterFunc(func(v uint32) bool {
		w := p.SimpleTypeWeight(int(v))
		if sessHasError(nil) {
			return true
		}
		return w == 0
	})
}

// BinaryOpWeight returns equal-group weight for eBinaryOps index.
// Incomplete Probabilities sticky 0 (no invent weight soft-skip past hole).
func (p *Probabilities) BinaryOpWeight(opIdx int) int {
	// Probabilities always live; sticky incomplete no invent weight 0 soft-skip
	if p == nil {
		sessNoteError(nil, ErrGeneric)
		return 0
	}
	if opIdx < 0 || opIdx >= len(p.binaryOpWeight) {
		return 0
	}
	return p.binaryOpWeight[opIdx]
}

// BinaryOpsFilter rejects eBinaryOps with weight 0 (BINARY_OPS_PROB_FILTER).
// Probabilities.cpp set_default_binary_ops_prob + set_prob_filter.
func (p *Probabilities) BinaryOpsFilter() Filter {
	if p == nil {
		sessNoteError(nil, ErrGeneric)
		return filterFunc(func(uint32) bool { return true })
	}
	if ProcessProbabilities() == p {
		if f, ok := p.probFilters[PBinaryOpsProb]; ok && f != nil {
			return f
		}
	}
	return filterFunc(func(v uint32) bool {
		w := p.BinaryOpWeight(int(v))
		if sessHasError(nil) {
			return true
		}
		return w == 0
	})
}

// UnaryOpWeight returns equal-group weight for eUnaryOps index.
// Incomplete Probabilities sticky 0 (no invent weight soft-skip past hole).
func (p *Probabilities) UnaryOpWeight(opIdx int) int {
	// Probabilities always live; sticky incomplete no invent weight 0 soft-skip
	if p == nil {
		sessNoteError(nil, ErrGeneric)
		return 0
	}
	if opIdx < 0 || opIdx >= len(p.unaryOpWeight) {
		return 0
	}
	return p.unaryOpWeight[opIdx]
}

// UnaryOpsFilter rejects eUnaryOps with weight 0 (UNARY_OPS_PROB_FILTER).
// Probabilities.cpp set_default_unary_ops_prob + set_prob_filter.
func (p *Probabilities) UnaryOpsFilter() Filter {
	if p == nil {
		sessNoteError(nil, ErrGeneric)
		return filterFunc(func(uint32) bool { return true })
	}
	if ProcessProbabilities() == p {
		if f, ok := p.probFilters[PUnaryOpsProb]; ok && f != nil {
			return f
		}
	}
	return filterFunc(func(v uint32) bool {
		w := p.UnaryOpWeight(int(v))
		if sessHasError(nil) {
			return true
		}
		return w == 0
	})
}

// initSingle — Probabilities::initialize_single_probs
func (p *Probabilities) initSingle(opts Options) {
	m := p.single
	m[PMoreStructUnionProb] = 50
	m[PBitFieldsCreationProb] = 50
	m[PBitFieldInNormalStructProb] = 10
	m[PScalarFieldInFullBitFieldsProb] = 10
	m[PExhaustiveBitFieldsProb] = 10
	m[PBitFieldsSignedProb] = 50
	m[PSafeOpsSignedProb] = 50
	m[PFuncAttrProb] = 30
	m[PTypeAttrProb] = 50
	m[PLabelAttrProb] = 30
	m[PVarAttrProb] = 30
	m[PBinaryConstProb] = 3

	if opts.Volatiles {
		m[PRegularVolatileProb] = 50
	} else {
		m[PRegularVolatileProb] = 0
	}
	if opts.Consts {
		m[PRegularConstProb] = 10
		m[PStricterConstProb] = 50
		m[PLooserConstProb] = 50
	} else {
		m[PRegularConstProb] = 0
		m[PStricterConstProb] = 0
		m[PLooserConstProb] = 0
	}
	if opts.Volatiles && opts.VolStructUnionFields && opts.GlobalVariables {
		m[PFieldVolatileProb] = 30
	} else {
		m[PFieldVolatileProb] = 0
	}
	if opts.Consts && opts.ConstStructUnionFields {
		m[PFieldConstProb] = 20
	} else {
		m[PFieldConstProb] = 0
	}

	m[PStdUnaryFuncProb] = 5
	m[PShiftByNonConstantProb] = 50
	m[PStructAsLTypeProb] = 30
	m[PUnionAsLTypeProb] = 25
	if opts.EnableFloat {
		m[PFloatAsLTypeProb] = 40
	} else {
		m[PFloatAsLTypeProb] = 0
	}
	if opts.Arrays {
		m[PNewArrayVariableProb] = 20
	} else {
		m[PNewArrayVariableProb] = 0
	}
	if opts.Pointers {
		m[PPointerAsLTypeProb] = 50
		m[PSelectDerefPointerProb] = 80
	} else {
		m[PPointerAsLTypeProb] = 0
		m[PSelectDerefPointerProb] = 0
	}
	m[PAccessOnceVariableProb] = 20
	m[PInlineFunctionProb] = opts.InlineFunctionProb
	m[PBuiltinFunctionProb] = opts.BuiltinFunctionProb
	m[PArrayOOBProb] = opts.ArrayOOBProb
}

// initSimpleTypes — Probabilities::set_default_simple_types_prob (equal group).
func (p *Probabilities) initSimpleTypes(opts Options) {
	// Type.h eSimpleType order; weight 0/1.
	w := make([]int, MaxSimpleTypes)
	// Void always 0 for non-parameter choose paths.
	w[int(EVoid)] = 0
	if opts.Int8 {
		w[int(EChar)] = 1
	}
	w[int(EInt)] = 1
	w[int(EShort)] = 1
	if !opts.CComp {
		w[int(ELong)] = 1
		w[int(EULong)] = 1
	}
	if opts.AllowInt64() {
		w[int(ELongLong)] = 1
		w[int(EULongLong)] = 1
	}
	if opts.UInt8 {
		w[int(EUChar)] = 1
	}
	w[int(EUInt)] = 1
	w[int(EUShort)] = 1
	if opts.EnableFloat {
		w[int(EFloat)] = 1
	}
	if opts.Int128 {
		w[int(EInt128)] = 1
	}
	if opts.UInt128 {
		w[int(EUInt128)] = 1
	}
	p.simpleTypeWeight = w
}

// initBinaryOps — Probabilities::set_default_binary_ops_prob (equal group weight 0/1).
// Probabilities.cpp:704–737 — muls/divs gated by CGOptions; all other ops weight 1.
func (p *Probabilities) initBinaryOps(opts Options) {
	w := make([]int, MaxBinaryOp)
	for i := range w {
		w[i] = 1
	}
	if !opts.Muls {
		w[int(BinMul)] = 0
	}
	if !opts.Divs {
		w[int(BinDiv)] = 0
	}
	p.binaryOpWeight = w
}

// initUnaryOps — Probabilities::set_default_unary_ops_prob (equal group weight 0/1).
// Probabilities.cpp:680+ — unary_plus gated by CGOptions::unary_plus_operator.
func (p *Probabilities) initUnaryOps(opts Options) {
	w := make([]int, MaxUnaryOp)
	if opts.UnaryPlusOperator {
		w[int(UnPlus)] = 1
	}
	w[int(UnMinus)] = 1
	w[int(UnNot)] = 1
	w[int(UnBitNot)] = 1
	p.unaryOpWeight = w
}

// initSafeOpsSize — Probabilities::set_default_safe_ops_size_prob (equal group).
// Probabilities.cpp:588–609 — Int8 if int8&uint8; Int16/32 always; Int64 if allow_int64.
func (p *Probabilities) initSafeOpsSize(opts Options) {
	w := make([]int, MaxSafeOpSizeNonFloat)
	if opts.Int8 && opts.UInt8 {
		w[int(SafeInt8)] = 1
	}
	w[int(SafeInt16)] = 1
	w[int(SafeInt32)] = 1
	if opts.AllowInt64() {
		w[int(SafeInt64)] = 1
	}
	p.safeOpsSizeWeight = w
}

// initStatementProbs — Probabilities::set_default_statement_prob (unequal group).
// Probabilities.cpp:748–774 — cumulative cutoffs; Statement::InitProbabilityTable
// installs stmtTable_ from pStatementProb (pname_to_type → eStatementType).
func (p *Probabilities) initStatementProbs(opts Options) {
	p.statementTable = buildStatementThresholdTable(opts)
}

// SafeOpsSizeWeight returns equal-group weight for SafeOpSize index (int sizes only).
// Probabilities always live at weight query; sticky 0 (no invent zero-weight soft-skip past hole).
// OOB sizeIdx is complete miss weight 0 (not incomplete IR).
func (p *Probabilities) SafeOpsSizeWeight(sizeIdx int) int {
	if p == nil {
		sessNoteError(nil, ErrGeneric)
		return 0
	}
	if sizeIdx < 0 || sizeIdx >= len(p.safeOpsSizeWeight) {
		return 0
	}
	return p.safeOpsSizeWeight[sizeIdx]
}

// SafeOpsSizeFilter rejects SafeOpSize indices with weight 0 (SAFE_OPS_SIZE_PROB_FILTER).
// Probabilities.cpp set_default_safe_ops_size_prob + set_prob_filter.
func (p *Probabilities) SafeOpsSizeFilter() Filter {
	if p == nil {
		sessNoteError(nil, ErrGeneric)
		return filterFunc(func(uint32) bool { return true })
	}
	if ProcessProbabilities() == p {
		if f, ok := p.probFilters[PSafeOpsSizeProb]; ok && f != nil {
			return f
		}
	}
	return filterFunc(func(v uint32) bool {
		w := p.SafeOpsSizeWeight(int(v))
		if sessHasError(nil) {
			return true
		}
		return w == 0
	})
}
