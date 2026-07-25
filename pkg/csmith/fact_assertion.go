// Upstream: Fact::OutputAssertion; FactPointTo::Output / is_assertable / has_invisible;
// FactMgr::output_assertions; Statement::post_output.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"strings"
)

// IsTop mirrors FactPointTo::is_top — empty points-to set.
// FactPointTo.h:93.
// Incomplete Fact shell sticky false (no invent TOP / soft re-pick past hole).
func (f *FactPointTo) IsTop() bool {
	return f.IsTopSess(testAmbientSession)
}

// IsTopSess is IsTop with explicit session residual sticky.
func (f *FactPointTo) IsTopSess(s *Session) bool {
	// Fact always live; sticky incomplete no invent empty-complete TOP
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	return len(f.PointTo) == 0
}

// IsBottom mirrors FactPointTo::is_bottom — always false (no bottom lattice).
// FactPointTo.h:94–96.
func (f *FactPointTo) IsBottom() bool {
	return f.IsBottomSess(testAmbientSession)
}

// IsBottomSess is IsBottom with explicit session residual sticky.
func (f *FactPointTo) IsBottomSess(s *Session) bool {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	return false
}

// SetTop mirrors FactPointTo::set_top — clear points-to set.
// FactPointTo.h:97.
func (f *FactPointTo) SetTop() {
	f.SetTopSess(testAmbientSession)
}

// SetTopSess is SetTop with explicit session residual sticky.
func (f *FactPointTo) SetTopSess(s *Session) {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	f.PointTo = nil
}

// SetBottom mirrors FactPointTo::set_bottom — no-op.
// FactPointTo.h:98.
func (f *FactPointTo) SetBottom() {
	f.SetBottomSess(testAmbientSession)
}

// SetBottomSess is SetBottom with explicit session residual sticky.
func (f *FactPointTo) SetBottomSess(s *Session) {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
}

// GetVar mirrors Fact::get_var / FactPointTo::get_var.
// FactPointTo.h:64.
func (f *FactPointTo) GetVar() *Variable {
	return f.GetVarSess(testAmbientSession)
}

// GetVarSess is GetVar with explicit session residual sticky.
func (f *FactPointTo) GetVarSess(s *Session) *Variable {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	return f.Var
}

// Output mirrors FactPointTo::Output — pointee set diagnostic.
// FactPointTo.cpp Output — subject and pointees by name.
func (f *FactPointTo) Output() string {
	return f.OutputSess(testAmbientSession)
}

// OutputSess is Output with explicit session residual sticky.
func (f *FactPointTo) OutputSess(s *Session) string {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	if f.Var == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	var b strings.Builder
	b.WriteString(f.Var.Name)
	b.WriteString(" => {")
	for i, p := range f.PointTo {
		if p == nil {
			sessNoteError(s, ErrGeneric)
			return ""
		}
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p.Name)
	}
	b.WriteString("}")
	return b.String()
}

// HasInvisible mirrors FactPointTo::has_invisible.
// FactPointTo.cpp:87–99 — subject or pointee not visible at stm parent.
// Incomplete Param/LocalVars / PointTo holes fail closed sticky as invisible
// (no invent "all visible" / soft re-pick past holes).
func (f *FactPointTo) HasInvisible(stParent *Block) bool {
	return f.HasInvisibleSess(testAmbientSession, stParent)
}

func (f *FactPointTo) HasInvisibleSess(s *Session, stParent *Block) bool {
	if f == nil || f.Var == nil {
		sessNoteError(s, ErrGeneric)
		return true
	}
	if stParent != nil && !stParent.StackScanComplete() {
		sessNoteError(s, ErrGeneric)
		return true
	}
	if !f.Var.IsVisibleSess(s, stParent) {
		// residual ERROR sticky — no invent invisible true past IsVisible hole
		if sessHasError(s) {
			return true
		}
		return true
	}
	// residual ERROR sticky — no invent soft-continue visible scan past IsVisible residual false path
	if sessHasError(s) {
		return true
	}
	for _, p := range f.PointTo {
		// Variable* always live in PointTo; nil hole sticky as invisible
		if p == nil {
			sessNoteError(s, ErrGeneric)
			return true
		}
		if IsSpecialPtr(p) {
			continue
		}
		if !p.IsVisibleSess(s, stParent) {
			// residual ERROR sticky — no invent invisible true past pointee IsVisible hole
			if sessHasError(s) {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue visible past pointee residual false path
		if sessHasError(s) {
			return true
		}
	}
	return false
}

// IsAssertable mirrors FactPointTo::is_assertable.
// FactPointTo.cpp:661–666 — no array subject; no garbage/tbd; no invisible.
// Incomplete PointTo fails closed sticky not-assertable (no invent skip GarbagePtr
// / soft re-pick past hole via IsVariableInSet false membership).}

func (f *FactPointTo) IsAssertable(stParent *Block) bool {
	return f.IsAssertableSess(testAmbientSession, stParent)
}

// IsAssertableSess is IsAssertable with explicit session residual sticky.
func (f *FactPointTo) IsAssertableSess(s *Session, stParent *Block) bool {
	if f == nil || f.Var == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	// incomplete fact sticky (no invent assertable via partial PointTo scan)
	if !FactsComplete([]*FactPointTo{f}) {
		sessNoteError(s, ErrGeneric)
		return false
	}
	// C++ isArray always ArrayVariable*; missing AsArray sticky not-assertable
	// (no invent complete not-array assertable past broken shell)
	if f.Var.IsArray && f.Var.AsArray == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	// get_array != null → not assertable
	if f.Var.IsArray || f.Var.AsArray != nil {
		return false
	}
	if IsVariableInSet(f.PointTo, GarbagePtr) || IsVariableInSet(f.PointTo, TBDPtr) {
		return false
	}
	inv := f.HasInvisibleSess(s, stParent)
	// residual ERROR sticky — no invent assertable true past HasInvisible residual hole
	if sessHasError(s) {
		return false
	}
	return !inv
}

// OutputCondition mirrors FactPointTo::Output — C expression for the fact.
// FactPointTo.cpp:627–658.
func (f *FactPointTo) OutputCondition() string {
	return f.OutputConditionSess(testAmbientSession)
}

func (f *FactPointTo) OutputConditionSess(s *Session) string {
	// Fact subject always live; sticky no invent bare pointee compare without var
	if f == nil || f.Var == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	lhs := outputFactVar(f.Var)
	// residual ERROR sticky — no invent soft-empty condition past outputFactVar residual
	if sessHasError(s) {
		return ""
	}
	// subject always live Output; sticky no invent " == 0" / " >= &" without lhs
	if lhs == "" {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	var parts []string
	for _, pointee := range f.PointTo {
		// FactPointTo.cpp: point_to_vars[i] always live; sticky no invent skip nil holes
		if pointee == nil {
			sessNoteError(s, ErrGeneric)
			return ""
		}
		if pointee.IsArray || (pointee.AsArray != nil) {
			// C++ isArray always ArrayVariable*; missing AsArray sticky
			// (no invent bare-name bounds range form past broken array shell)
			if pointee.IsArray && pointee.AsArray == nil {
				sessNoteError(s, ErrGeneric)
				return ""
			}
			// range form: (p >= &lo && p <= &hi)
			// OutputLower/UpperBound always live; sticky no invent "(p >= & && p <= &)"
			lo := pointee.OutputLowerBoundSess(s, false)
			// residual ERROR sticky — no invent soft-continue hi past LowerBound residual
			if sessHasError(s) {
				return ""
			}
			hi := pointee.OutputUpperBoundSess(s, false)
			// residual ERROR sticky — no invent soft-continue range past UpperBound residual
			if sessHasError(s) {
				return ""
			}
			if lo == "" || hi == "" {
				sessNoteError(s, ErrGeneric)
				return ""
			}
			parts = append(parts, "("+lhs+" >= &"+lo+" && "+lhs+" <= &"+hi+")")
			continue
		}
		rhs := ""
		switch {
		case pointee == GarbagePtr:
			rhs = "dangling"
		case pointee == TBDPtr:
			rhs = "tbd"
		case pointee == NullPtr:
			rhs = "0"
		default:
			// pointee->Output always live name; sticky no invent bare "&"
			nm := pointee.GetActualNameSess(s, false)
			// residual ERROR sticky — no invent soft-empty & past GetActualName residual
			if sessHasError(s) {
				return ""
			}
			if nm == "" {
				sessNoteError(s, ErrGeneric)
				return ""
			}
			rhs = "&" + nm
		}
		parts = append(parts, lhs+" == "+rhs)
	}
	return strings.Join(parts, " || ")
}

func outputFactVar(v *Variable) string {
	return outputFactVarSess(testAmbientSession, v)
}

func outputFactVarSess(s *Session, v *Variable) string {
	// Variable always live at fact emit; sticky no invent empty lhs token
	if v == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	name := v.GetActualNameSess(s, false)
	// residual ERROR sticky — no invent soft-empty fact-var past GetActualName residual
	if sessHasError(s) {
		return ""
	}
	// sticky no invent "[0]" indices without identifier
	if name == "" {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	// FactPointTo.cpp:612–621 — output_var: for array, [0] per get_dimension()
	// no soft invent dim=1 when sizes empty
	// C++ isArray always ArrayVariable*; missing AsArray sticky empty
	// (no invent [0] indices from ArraySizes alone past broken shell)
	if v.IsArray && v.AsArray == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	if v.IsArray || v.AsArray != nil {
		dim := 0
		if v.AsArray != nil {
			dim = len(v.AsArray.Sizes)
		}
		for i := 0; i < dim; i++ {
			name += "[0]"
		}
	}
	return name
}

// OutputAssertion mirrors Fact::OutputAssertion.
// Fact.cpp:64–73 — assert(cond); comment-out if not assertable.}

func (f *FactPointTo) OutputAssertion(stParent *Block, indent string) string {
	return f.OutputAssertionSess(testAmbientSession, stParent, indent)
}

func (f *FactPointTo) OutputAssertionSess(s *Session, stParent *Block, indent string) string {
	// Fact* always live at assert emit; sticky no invent empty assert without it
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	// TOP fact: no assert condition (complete empty success)
	isTop := f.IsTopSess(s)
	// residual ERROR sticky — no invent soft-empty assert past IsTop residual
	if sessHasError(s) {
		return ""
	}
	if isTop {
		return ""
	}
	cond := f.OutputConditionSess(s)
	if cond == "" {
		// incomplete condition IR sticky (OutputCondition may already SetError)
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return ""
	}
	prefix := ""
	if !f.IsAssertableSess(s, stParent) {
		// residual ERROR sticky — no invent assert line past IsAssertable residual hole
		if sessHasError(s) {
			return ""
		}
		prefix = "//"
	} else if sessHasError(s) {
		// residual ERROR sticky — no invent live assert past IsAssertable residual true
		return ""
	}
	return indent + prefix + "assert (" + cond + ");\n"
}

// OutputAssertions mirrors FactMgr::output_assertions.
// FactMgr.cpp:614–649 — post_condition uses updated final facts; filter unused globals.}

func (fm *FactMgr) OutputAssertions(st *Stmt, stParent *Block, indent string, postCondition bool) string {
	// FactMgr + Statement always live for assertion emit; sticky no invent section without them
	if fm == nil || st == nil {
		noteErrFM(fm, ErrGeneric)
		return ""
	}
	// Statement::stm_id always live; StmID 0 sticky (no invent empty assertion section)
	if StmIDUnset(st.StmID) {
		noteErrFM(fm, ErrGeneric)
		return ""
	}
	var facts []*FactPointTo
	if !postCondition {
		facts = fm.GetMapFactsInFinal(st.StmID)
	} else {
		facts = fm.FindUpdatedFinalFacts(st.StmID)
		// fall back to non-final updated if final empty (complete empty only)
		if FactsComplete(facts) && len(facts) == 0 {
			facts = fm.FindUpdatedFacts(st.StmID)
		}
	}
	// incomplete maps fail closed sticky (no invent empty assertion block / soft re-pick)
	if !FactsComplete(facts) {
		if !hasErrFM(fm) {
			noteErrFM(fm, ErrGeneric)
		}
		return ""
	}
	if len(facts) == 0 {
		return ""
	}
	// emit assertions first; no invent comment-only shell when all facts filtered
	eff := EmptyEffect()
	if fm.Func != nil {
		eff = fm.Func.FEffect
	}
	var body strings.Builder
	for _, f := range facts {
		// Fact* always live in fact maps; nil hole sticky (no invent skip holes)
		if f == nil || f.Var == nil {
			noteErrFM(fm, ErrGeneric)
			return ""
		}
		// skip globals neither read nor written in this function
		isG := f.Var.IsGlobalSess(sessFromFM(fm))
		// residual ERROR sticky — no invent soft-skip then partial assert emit past IsGlobal residual
		if hasErrFM(fm) {
			return ""
		}
		if isG {
			rd := eff.IsReadSess(sessFromFM(fm), f.Var)
			// residual ERROR sticky — no invent soft-skip assert past IsRead residual
			if hasErrFM(fm) {
				return ""
			}
			wr := eff.IsWrittenSess(sessFromFM(fm), f.Var)
			// residual ERROR sticky — no invent soft-skip assert past IsWritten residual
			if hasErrFM(fm) {
				return ""
			}
			if !rd && !wr {
				continue
			}
		}
		// IsTop / empty OutputAssertion intentionally silent; non-nil fact still live
		body.WriteString(f.OutputAssertion(stParent, indent))
		// residual ERROR sticky — no invent partial assertion section past hard IR hole
		if hasErrFM(fm) {
			return ""
		}
	}
	if body.Len() == 0 {
		return ""
	}
	var b strings.Builder
	// comments for compound / simple (FactMgr.cpp:625–635)
	switch st.Kind {
	case StmtFor:
		b.WriteString(indent + "/* facts after for loop */\n")
	case StmtIfElse:
		b.WriteString(indent + "/* facts after branching */\n")
	case StmtAssign, StmtInvoke, StmtReturn:
		b.WriteString(indent + "/* statement id: " + itoa(st.StmID) + " */\n")
	}
	b.WriteString(body.String())
	return b.String()
}

// PreOutput mirrors Statement::pre_output.
// Statement.cpp:905–917 — if goto target emit "label:" [attrs]; else output_hash.
// isGotoTarget true means step_hash was not emitted (C++ returns 1 after label).
//
// Label resolution: Statement.cpp:908–914 — find_jump_sources only (gotos[0]->label).
// SourceLabel is generation-side dest mirror used when FactMgr is absent (no CFG).
func PreOutput(st *Stmt, fm *FactMgr, emitStepHash, emitLabelAttrs bool, attrRng *Rng, indent string) (out string, isGotoTarget bool) {
	return PreOutputSess(testAmbientSession, st, fm, emitStepHash, emitLabelAttrs, attrRng, indent)
}

func PreOutputSess(s *Session, st *Stmt, fm *FactMgr, emitStepHash, emitLabelAttrs bool, attrRng *Rng, indent string) (out string, isGotoTarget bool) {
	// Statement always live at pre_output; sticky incomplete no invent empty pre shell
	if st == nil {
		sessNoteError(s, ErrGeneric)
		return "", false
	}
	label := ""
	// Statement.cpp:908–914 — find_jump_sources → first goto label
	if fm != nil {
		// Statement::stm_id always live under FM; StmID 0 sticky fail closed
		// (no invent SourceLabel / step_hash soft-fallback for incomplete id)
		if StmIDUnset(st.StmID) {
			sessNoteError(s, ErrGeneric)
			return "", false
		}
		srcs := fm.FindJumpSources(st.StmID)
		// nil = incomplete CFG (FindJumpSources may already sticky); no invent label
		// empty non-nil = no gotos (do not fall back to SourceLabel)
		if srcs == nil {
			// incomplete CFG sticky — no invent step_hash soft-fallback past hole
			if !sessHasError(s) {
				sessNoteError(s, ErrGeneric)
			}
			return "", false
		}
		if len(srcs) > 0 {
			label = FindJumpLabel(fm, st.StmID)
			// resolve from source stmt when FindJumpLabel missed registry
			if label == "" && fm.Func != nil {
				src := FindStmtByIDSess(s, fm.Func, srcs[0])
				// residual ERROR sticky — no invent soft-continue label/SourceLabel past FindStmt hole
				if sessHasError(s) {
					return "", false
				}
				if src != nil {
					label = src.Label
				}
			}
		}
		// with FactMgr: do not fall back to SourceLabel without jump sources
		// (C++ only labels when find_jump_sources non-empty)
	} else if st.SourceLabel != "" {
		// no DFA / no FM: emit generation-time dest label
		label = st.SourceLabel
	}
	if label != "" {
		// Statement.cpp:910–912 — out << label << ":" << attr << endl;
		// NO output_tab: labels always column 0 (seed-2 lbl_1269:).
		_ = indent
		var b strings.Builder
		b.WriteString(label)
		b.WriteString(":")
		attr := st.LabelAttr
		if attr == "" && emitLabelAttrs && attrRng != nil {
			// Prefer emit bag s; FactMgr bag only when present (unit tests may omit FM).
			attrSess := s
			if attrSess == nil {
				attrSess = sessFromFM(fm)
			}
			if attrSess == nil {
				attrSess = testAmbientSession
			}
			if ag := EnsureLabelAttrGeneratorSess(attrSess); ag != nil {
				attr = ag.OutputSess(attrSess, attrRng)
			}
			// residual ERROR sticky — no invent soft-continue label past attr residual
			if sessHasError(s) {
				return "", false
			}
		}
		if attr != "" {
			b.WriteString(attr)
		}
		b.WriteString("\n")
		// Statement.cpp:905–914 — return 1 after label (no output_hash)
		return b.String(), true
	}
	// Statement.cpp:916 — output_hash when not a jump target
	// Statement.cpp:927–931 / OutputMgr.cpp:161–167
	// emitStepHash is set only when StepHashByStmt && ComputeHash (Block make)
	// so step_hash(n) is never invented without live helper defs
	if emitStepHash && !StmIDUnset(st.StmID) {
		return indent + "step_hash(" + Int2Str(st.StmID) + ");\n", false
	}
	return "", false
}

// PostOutput mirrors Statement::post_output.
// Statement.cpp:919–924 — paranoid assertions after non-block statements.
func PostOutput(st *Stmt, stParent *Block, fm *FactMgr, paranoid, concise bool, indent string) string {
	// options off: soft empty (no assert section invented when not requested)
	if !paranoid || concise {
		return ""
	}
	// when paranoid, Statement + FactMgr always live sticky
	if st == nil || fm == nil {
		noteErrFM(fm, ErrGeneric)
		return ""
	}
	if st.Kind == StmtBlock {
		return ""
	}
	out := fm.OutputAssertions(st, stParent, indent, true)
	// residual ERROR sticky — no invent soft-empty post past OutputAssertions residual
	if hasErrFM(fm) {
		return ""
	}
	return out
}
