// Upstream: Probabilities.h / Probabilities.cpp
// (initialize_single_probs, set_default_simple_types_prob, ProbabilityFilter for equal groups).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// ProbName identifies a probability knob (subset used by the fair spine so far).
// Full enum in Probabilities.h — extend when a ported path needs a name.
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

	// Group: simple types (equal weights 0/1)
	PSimpleTypesProb
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
}

// NewProbabilities builds tables from opts like Probabilities::initialize
// after CGOptions::set_default_settings (and option flags).
func NewProbabilities(opts Options) *Probabilities {
	p := &Probabilities{
		single: make(map[ProbName]int),
	}
	p.initSingle(opts)
	p.initSimpleTypes(opts)
	p.initBinaryOps(opts)
	p.initUnaryOps(opts)
	p.initSafeOpsSize(opts)
	p.initStatementProbs(opts)
	return p
}

// StatementThresholdTable returns Statement::stmtTable_ built from pStatementProb.
// Incomplete Probabilities sticky nil (no invent empty table soft-skip past hole).
func (p *Probabilities) StatementThresholdTable() *ThresholdTable {
	// Probabilities always live in C++ singleton; sticky incomplete no invent nil table
	if p == nil {
		SetError(ErrGeneric)
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
		SetError(ErrGeneric)
		return 0
	}
	return p.single[name]
}

// SimpleTypeWeight returns equal-group weight (0 or 1) for eSimpleType index.
// Incomplete Probabilities sticky 0 (no invent weight 1 soft-skip past hole).
func (p *Probabilities) SimpleTypeWeight(simpleIdx int) int {
	// Probabilities always live; sticky incomplete no invent weight 0 soft-skip
	if p == nil {
		SetError(ErrGeneric)
		return 0
	}
	if simpleIdx < 0 || simpleIdx >= len(p.simpleTypeWeight) {
		return 0
	}
	return p.simpleTypeWeight[simpleIdx]
}

// SimpleTypesFilter rejects eSimpleType indices with weight 0.
// ProbabilityFilter for pSimpleTypesProb (equal group): filter(v) when weight==0.
func (p *Probabilities) SimpleTypesFilter() Filter {
	return filterFunc(func(v uint32) bool {
		return p.SimpleTypeWeight(int(v)) == 0
	})
}

// BinaryOpWeight returns equal-group weight for eBinaryOps index.
// Incomplete Probabilities sticky 0 (no invent weight soft-skip past hole).
func (p *Probabilities) BinaryOpWeight(opIdx int) int {
	// Probabilities always live; sticky incomplete no invent weight 0 soft-skip
	if p == nil {
		SetError(ErrGeneric)
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
	return filterFunc(func(v uint32) bool {
		return p.BinaryOpWeight(int(v)) == 0
	})
}

// UnaryOpWeight returns equal-group weight for eUnaryOps index.
// Incomplete Probabilities sticky 0 (no invent weight soft-skip past hole).
func (p *Probabilities) UnaryOpWeight(opIdx int) int {
	// Probabilities always live; sticky incomplete no invent weight 0 soft-skip
	if p == nil {
		SetError(ErrGeneric)
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
	return filterFunc(func(v uint32) bool {
		return p.UnaryOpWeight(int(v)) == 0
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
func (p *Probabilities) SafeOpsSizeWeight(sizeIdx int) int {
	if p == nil || sizeIdx < 0 || sizeIdx >= len(p.safeOpsSizeWeight) {
		return 0
	}
	return p.safeOpsSizeWeight[sizeIdx]
}

// SafeOpsSizeFilter rejects SafeOpSize indices with weight 0 (SAFE_OPS_SIZE_PROB_FILTER).
// Probabilities.cpp set_default_safe_ops_size_prob + set_prob_filter.
func (p *Probabilities) SafeOpsSizeFilter() Filter {
	return filterFunc(func(v uint32) bool {
		return p.SafeOpsSizeWeight(int(v)) == 0
	})
}
