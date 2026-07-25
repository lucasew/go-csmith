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

// StmtLabel is a Go-only labeled no-op marker for forward goto targets (not in Csmith enum).
const StmtLabel StatementType = 100

// buildStatementThresholdTable builds ProbabilityTable entries for pStatementProb
// from set_default_statement_prob thresholds (unequal group).
// Probabilities.cpp:748–774; keys are cumulative cutoffs, values are statement kinds
// (C++ stores ProbName then pname_to_type → eStatementType; we store StatementType).
func buildStatementThresholdTable(s *Session, opts Options) *ThresholdTable {
	t := &ThresholdTable{}
	// Block weight 0 → not inserted
	// IfElse 15, For 30, Return 35, Continue 40, Break 45
	t.AddSess(s, 15, int(StmtIfElse))
	t.AddSess(s, 30, int(StmtFor))
	t.AddSess(s, 35, int(StmtReturn))
	t.AddSess(s, 40, int(StmtContinue))
	t.AddSess(s, 45, int(StmtBreak))
	if opts.Jumps && opts.Arrays {
		t.AddSess(s, 50, int(StmtGoto))
		t.AddSess(s, 60, int(StmtArrayOp))
	} else if opts.Jumps && !opts.Arrays {
		t.AddSess(s, 50, int(StmtGoto))
		// ArrayOp 0 → skip
	} else if !opts.Jumps && opts.Arrays {
		t.AddSess(s, 55, int(StmtArrayOp))
	}
	t.AddSess(s, 100, int(StmtAssign))
	return t
}

// NewStatementThresholdTable returns the session statement table when process
// Probabilities is live (Statement::stmtTable_ from pStatementProb); otherwise
// builds a library one-off from opts (tests that pass an explicit table).
// Generation should prefer ProcessStmtTab / probs.StatementThresholdTableSess(s).
func NewStatementThresholdTable(opts Options) *ThresholdTable {
	return NewStatementThresholdTableSess(testAmbientSession, opts)
}

// NewStatementThresholdTableSess prefers session Probabilities statement table.
func NewStatementThresholdTableSess(s *Session, opts Options) *ThresholdTable {
	if p := sessProbs(s); p != nil {
		if t := p.StatementThresholdTableSess(s); t != nil {
			return t
		}
	}
	// no invent from process when unset — library path builds from opts only
	return buildStatementThresholdTable(s, opts)
}

// NumberToType mirrors Statement::number_to_type(value) for value in [0,100).
// Statement.cpp:141–147.
func NumberToType(table *ThresholdTable, value uint32) StatementType {
	return NumberToTypeSess(testAmbientSession, table, value)
}

// NumberToTypeSess is NumberToType with explicit session residual sticky.
func NumberToTypeSess(s *Session, table *ThresholdTable, value uint32) StatementType {
	// Statement.cpp:141–147 — assert(stmtTable_); assert(value < 100)
	// sticky fail closed MaxStatementType (invalid) — no invent eAssign / soft re-pick
	if table == nil || value >= 100 {
		sessNoteError(s, ErrGeneric)
		return MaxStatementType
	}
	v := table.GetValueSess(s, int(value))
	if v < 0 {
		return MaxStatementType
	}
	return StatementType(v)
}

// StatementProbability mirrors StatementProbability without StatementFilter.
// Statement.cpp:230–235 — rnd_upto(100); number_to_type.
// Callers that need filter pass reject via RndUptoFilter.
func StatementProbability(r *Rng, table *ThresholdTable) StatementType {
	return StatementProbabilitySess(testAmbientSession, r, table)
}

// StatementProbabilitySess is StatementProbability with explicit session residual sticky.
func StatementProbabilitySess(s *Session, r *Rng, table *ThresholdTable) StatementType {
	// Statement.cpp:233–234 — assert(value != -1); assert(0..99)
	// ERROR_GUARD(MAX_STATEMENT_TYPE); sticky no soft invent eAssign
	if r == nil || table == nil {
		sessNoteError(s, ErrGeneric)
		return MaxStatementType
	}
	v := r.RndUptoSess(s, 100)
	return NumberToTypeSess(s, table, v)
}

// StatementProbabilityFilter mirrors StatementProbability with a Filter
// (e.g. reject compound when at max depth — filter implemented by caller).
func StatementProbabilityFilter(r *Rng, table *ThresholdTable, f Filter) StatementType {
	return StatementProbabilityFilterSess(testAmbientSession, r, table, f)
}

// StatementProbabilityFilterSess is StatementProbabilityFilter with explicit session residual sticky.
func StatementProbabilityFilterSess(s *Session, r *Rng, table *ThresholdTable, f Filter) StatementType {
	// Statement.cpp ERROR_GUARD(MAX_STATEMENT_TYPE); sticky no soft invent eAssign
	if r == nil || table == nil {
		sessNoteError(s, ErrGeneric)
		return MaxStatementType
	}
	v := r.RndUptoFilterSess(s, 100, f)
	// filter rejection may yield -1 from RndUptoFilter — non-sticky MAX (soft re-pick kinds)
	if int32(v) < 0 {
		return MaxStatementType
	}
	return NumberToTypeSess(s, table, v)
}

// IsCompound mirrors Statement::is_compound.
func IsCompound(t StatementType) bool {
	return t == StmtBlock || t == StmtFor || t == StmtIfElse || t == StmtArrayOp
}

// GetType mirrors Statement::get_type — returns the eStatementType kind.
// Incomplete Stmt sticky MaxStatementType.
func (st *Stmt) GetType() StatementType {
	return st.GetTypeSess(testAmbientSession)
}

// GetTypeSess is GetType with explicit session residual sticky.
func (st *Stmt) GetTypeSess(s *Session) StatementType {
	if st == nil {
		sessNoteError(s, ErrGeneric)
		return MaxStatementType
	}
	return st.Kind
}

// GetCurrentSID mirrors Statement::get_current_sid process counter.
// Statement uses Session.NextStmID (ambient bag when no Sess).
func GetCurrentSID() int {
	return GetCurrentSIDSess(testAmbientSession)
}

// GetCurrentSIDSess returns NextStmID on an explicit session bag.
func GetCurrentSIDSess(s *Session) int {
	return sessOrAmbient(s).NextStmID
}
