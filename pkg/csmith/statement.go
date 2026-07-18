// Upstream: Statement.h / Statement.cpp (eStatementType, number_to_type, StatementProbability).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// StatementType mirrors eStatementType.
type StatementType int

const (
	StmtAssign StatementType = iota
	StmtBlock
	StmtFor
	StmtIfElse
	StmtInvoke
	StmtReturn
	StmtContinue
	StmtBreak
	StmtGoto
	StmtArrayOp
)

// MaxStatementType mirrors MAX_STATEMENT_TYPE.
const MaxStatementType StatementType = StmtArrayOp + 1

// NewStatementThresholdTable builds ProbabilityTable for pStatementProb
// from set_default_statement_prob thresholds (unequal group).
// Probabilities.cpp:748–774; keys are cumulative cutoffs, values are statement kinds.
// Note: pname_to_type maps ProbName → eStatementType; we store StatementType directly.
func NewStatementThresholdTable(opts Options) *ThresholdTable {
	t := &ThresholdTable{}
	// Block weight 0 → not inserted
	// IfElse 15, For 30, Return 35, Continue 40, Break 45
	t.Add(15, int(StmtIfElse))
	t.Add(30, int(StmtFor))
	t.Add(35, int(StmtReturn))
	t.Add(40, int(StmtContinue))
	t.Add(45, int(StmtBreak))
	if opts.Jumps && opts.Arrays {
		t.Add(50, int(StmtGoto))
		t.Add(60, int(StmtArrayOp))
	} else if opts.Jumps && !opts.Arrays {
		t.Add(50, int(StmtGoto))
		// ArrayOp 0 → skip
	} else if !opts.Jumps && opts.Arrays {
		t.Add(55, int(StmtArrayOp))
	}
	t.Add(100, int(StmtAssign))
	return t
}

// NumberToType mirrors Statement::number_to_type(value) for value in [0,100).
// Statement.cpp:141–147.
func NumberToType(table *ThresholdTable, value uint32) StatementType {
	if table == nil || value >= 100 {
		return StmtAssign
	}
	v := table.GetValue(int(value))
	if v < 0 {
		return StmtAssign
	}
	return StatementType(v)
}

// StatementProbability mirrors StatementProbability without StatementFilter.
// Statement.cpp:230–235 — rnd_upto(100); number_to_type.
// Callers that need filter pass reject via RndUptoFilter.
func StatementProbability(r *Rng, table *ThresholdTable) StatementType {
	if r == nil {
		return StmtAssign
	}
	v := r.RndUpto(100)
	return NumberToType(table, v)
}

// StatementProbabilityFilter mirrors StatementProbability with a Filter
// (e.g. reject compound when at max depth — filter implemented by caller).
func StatementProbabilityFilter(r *Rng, table *ThresholdTable, f Filter) StatementType {
	if r == nil {
		return StmtAssign
	}
	v := r.RndUptoFilter(100, f)
	return NumberToType(table, v)
}

// IsCompound mirrors Statement::is_compound.
func IsCompound(t StatementType) bool {
	return t == StmtBlock || t == StmtFor || t == StmtIfElse || t == StmtArrayOp
}
